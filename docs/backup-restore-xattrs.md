# Swift Backup/Restore: Extended Attributes (xattrs)

## Problem

OADP DataMover uses kopia for filesystem-level PVC data transfer. Even when
starting from a CSI volume snapshot (block-level), DataMover mounts the
snapshot filesystem and copies files individually using kopia. Kopia does not
preserve extended attributes (xattrs) during this process
([kopia/kopia#544](https://github.com/kopia/kopia/issues/544)).

Swift stores all object metadata in `user.swift.metadata` xattrs on data
files — the object name, Content-Type, ETag, Content-Length, and timestamps
are all in the xattr, not in the file content. After a backup/restore cycle,
the file content is intact but the xattrs are lost. Swift's object-auditor
detects files without valid metadata and quarantines them within minutes of
the storage pod starting. All Swift API requests return 404.

## Solution

Two components work together to preserve xattrs across backup/restore:

1. **Backup:** A Velero pre-backup hook in the PVC Backup CR dumps all xattrs
   to a file on the PVC before the CSI snapshot. The dump file is captured in
   the snapshot and transferred by kopia as regular file content.

2. **Restore:** An init container on the Swift storage StatefulSet restores
   xattrs from the dump file before any Swift service container starts. This
   prevents the object-auditor from quarantining data.

## Backup: Velero Pre-Backup Hook

The xattr dump is defined as a hook in the OADP PVC Backup CR. This gives
control over the timeout and command without requiring operator changes.

The hook targets Swift storage pods by label (`component: swift-storage`),
runs `getfattr` in the `object-server` container, and with `onError: Fail`
aborts the backup if the dump fails.

Velero fires pre-backup hooks on pods whose PVCs match the backup's
`labelSelector`, even if the pods themselves don't carry the label.

### PVC Backup CR with hook

```bash
cat <<EOF | oc apply -f -
apiVersion: velero.io/v1
kind: Backup
metadata:
  name: openstack-backup-pvcs-\${BACKUP_TS}
  namespace: openshift-adp
  annotations:
    openstack.org/csv-version: "\${CSV_VERSION}"
    openstack.org/catalog-source-image: "\${CATALOG_IMG}"
    openstack.org/operator-image: "\${OPERATOR_IMG}"
spec:
  includedNamespaces:
  - openstack
  labelSelector:
    matchLabels:
      backup.openstack.org/backup: "true"
  snapshotVolumes: true
  defaultVolumesToFsBackup: false
  snapshotMoveData: true
  volumeSnapshotLocations: []
  storageLocation: velero-1
  ttl: 720h
  hooks:
    resources:
      - name: swift-xattr-backup
        includedNamespaces:
          - openstack
        labelSelector:
          matchLabels:
            component: swift-storage
        pre:
          - exec:
              container: object-server
              command:
                - /bin/bash
                - -c
                - |
                  set -e
                  DUMP="/srv/node/pv/.swift-xattrs.dump"
                  rm -f "$DUMP"
                  getfattr -R -d -m user.swift /srv/node/pv/ 1> "$DUMP"
                  echo "xattr backup complete: $(grep -c '^# file:' "$DUMP") files"
              onError: Fail
              timeout: 300s
EOF
```

> **Note:** For large deployments, the `getfattr` command can be parallelized
> across partition directories using `find` and `xargs -P` to reduce the dump
> duration. Customize the hook command in the Backup CR as needed.

The dump file (`.swift-xattrs.dump`) lives on the PVC and is ignored by
Swift — it is not inside any partition directory and does not match Swift's
data file naming conventions.

### Dump file format and size

The dump uses `getfattr -R -d` output, restored natively by
`setfattr --restore`:

```
# file: srv/node/pv/objects/790/9ab/c598b0cdb4df425c0cb5b2c4767c09ab/1778160244.94060.data
user.swift.metadata=0sgAJ9cQAoY19jb2RlY3MK...
user.swift.metadata_checksum="4266f4b6d601dc2915b69b92c3b0fd44"
```

Size is proportional to object count, not data size (~500 KB per 1K objects).

### Staleness window

The `getfattr` traversal is not atomic. Objects written between the dump
completing and the CSI snapshot being taken will have data in the backup but
no xattrs in the dump. After restore, these objects may be quarantined by the
auditor. For control plane Swift this is a minimal risk — write rates are low
(Glance images are uploaded infrequently) and the dump completes quickly:

| Deployment size | Estimated dump duration |
|-----------------|------------------------|
| < 10K objects   | seconds                |
| 100K objects    | 1-2 minutes            |
| 1M+ objects     | 5-10 minutes           |

With multi-replica Swift, the hook runs independently on each pod, so objects
missed in one replica's dump may be captured in another's. If a healthy copy
exists on another storage node, the replicator can recover the object.

## Restore: Init Container

Velero restore hooks only apply to pods that Velero itself restores. Swift
storage pods are created by the swift-operator when the
OpenStackControlPlane CR is reconciled, so Velero restore hooks cannot be
used.

Instead, an init container (`xattr-restore`) is added to the Swift storage
StatefulSet. It runs before all Swift service containers on every pod
startup. The restore script is loaded from a ConfigMap
(`swift-xattr-restore-script`) created by the operator via the standard
`TemplateTypeScripts` pattern (`templates/swiftstorage/bin/xattr-restore.sh`).

The init container:

1. Checks for `/srv/node/pv/.swift-xattrs.dump`
2. If found (post-restore), restores xattrs using `setfattr --restore`
3. Validates all `.data` files have `user.swift.metadata` xattr
4. Logs any files missing xattrs to `/srv/node/pv/.swift-xattrs.missing`
5. Renames the dump to `.swift-xattrs.dump.applied` so subsequent restarts
   skip the restore
6. If no dump file exists (normal startup), exits immediately

With `set -e`, any failure in `setfattr` causes the init container to exit
non-zero. Kubernetes keeps the pod in `Init:CrashLoopBackOff` and the main
containers (including the object-auditor) never start — no data is
quarantined.

## Recovery

### Init container failure

If the init container fails (corrupted dump, permission issues), the pod
stays in `Init:CrashLoopBackOff`. The object-auditor never starts, so no
data is quarantined. To recover:

1. Scale down Swift storage via the OpenStackControlPlane CR:
   ```bash
   oc patch openstackcontrolplane <name> -n openstack --type merge \
     -p '{"spec":{"swift":{"template":{"swiftStorage":{"replicas":0}}}}}'
   ```
2. Wait for the pod to terminate and release the PVC
3. Run a manual Job that mounts the PVC, fixes the issue (e.g., re-run
   `setfattr --restore` or rename the dump file), and exits
4. Scale back up:
   ```bash
   oc patch openstackcontrolplane <name> -n openstack --type merge \
     -p '{"spec":{"swift":{"template":{"swiftStorage":{"replicas":1}}}}}'
   ```
5. The pod starts, init container finds no dump file, skips restore, main
   containers start normally

### Quarantined objects

If objects are quarantined (from the staleness window, or if the init
container could not restore their xattrs), the file content is intact —
only the xattr metadata is lost. Quarantined files can be identified by
their MD5 checksum and matched back to their original objects.

**Identify quarantined files:**

```bash
oc exec swift-storage-0 -n openstack -c object-server -- bash -c '
  for f in /srv/node/pv/quarantined/objects/*/*.data; do
    [ -f "$f" ] || continue
    MD5=$(md5sum "$f" | cut -d" " -f1)
    SIZE=$(stat -c%s "$f")
    echo "$MD5  $SIZE  $f"
  done
'
```

**Match against Glance images:**

```bash
oc exec openstackclient -- openstack image list --long \
  -c ID -c Name -c Checksum -c Size
```

Match the MD5 from the quarantined file to the Checksum column.

**Match via Swift container database (if Glance API is unavailable):**

```bash
oc exec swift-storage-0 -n openstack -c object-server -- python3 -c '
import sqlite3, glob
for db in glob.glob("/srv/node/pv/containers/*/*/*/*.db"):
    conn = sqlite3.connect(db)
    print(f"=== {db} ===")
    for row in conn.execute("SELECT name, etag, content_type, size FROM object WHERE deleted=0"):
        print(f"  {row[0]}  etag={row[1]}  type={row[2]}  size={row[3]}")
    conn.close()
'
```

**Re-upload matched objects:**

```bash
# Copy the quarantined file out
oc cp swift-storage-0:/srv/node/pv/quarantined/objects/<hash>/<timestamp>.data \
  /tmp/recovered-image.raw -c object-server

# For Glance images: delete the broken entry and re-upload
oc exec openstackclient -- openstack image delete <image-id>
oc exec openstackclient -- openstack image create \
  --file /tmp/recovered-image.raw \
  --disk-format raw --container-format bare <image-name>
```

For multi-replica deployments, check if a healthy copy exists on another
replica first — the replicator may have already recovered the object.

## Upstream Tracking

The xattr limitation in kopia is tracked at
[kopia/kopia#544](https://github.com/kopia/kopia/issues/544).
