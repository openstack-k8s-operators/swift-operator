# Configuring and deploying OpenStack Swift

Swift can use either PersistentVolumes on OpenShift nodes or using disks on
external data plane nodes. Using external data plane nodes gives you more
flexibility and supports bigger storage deployments, whereas using
PersistentVolumes on OpenShift nodes is easier to deploy, but limited to a
single PV per node at the moment.

## Deploying Swift using Storage on the OpenShift nodes using PersistentVolumes

### Deploying Swift

Default deployments should use at least 3 SwiftStorage replicas and 2
SwiftProxy replicas. Both values can be increased as needed to distribute
storage across more nodes and disks. The number of `ringReplicas` defines the
number of actual object copies in the cluster. As an example, you could set
`ringReplicas: 3' and 'swiftStorage/replicas: 5'; in this case every object
will be stored on 3 different PVs, and the total number of PVs is 5.

```yaml
apiVersion: core.openstack.org/v1beta1
kind: OpenStackControlPlane
metadata:
  name: openstack-galera-network-isolation
  namespace: openstack
spec:
  ...
  swift:
    enabled: true
    template:
      swiftProxy:
        replicas: 2
      swiftRing:
        ringReplicas: 3
      swiftStorage:
        replicas: 3
        storageClass: swift-storage
        storageRequest: 100Gi
```

The above example uses the the StorageClass `swift-storage`. It is recommended
to use a separate storage class to be used by Swift. An example class might
look like this:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: swift-storage
  annotations:
    storageclass.kubernetes.io/is-default-class: "false"
provisioner: kubernetes.io/no-provisioner
reclaimPolicy: Retain
volumeBindingMode: WaitForFirstConsumer
```

Swift requires multiple PersistentVolumes. It is strongly recommended to create
these PVs on different nodes, and only use one PV per node to maximize
availability and data durability. Use external data plane nodes if you want to
deploy a larger Object storage cluster using multiple disks per node.

An example PV might look like this:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  finalizers:
  - kubernetes.io/pv-protection
  name: swift-storage-01
spec:
  accessModes:
  - ReadWriteOnce
  - ReadWriteMany
  - ReadOnlyMany
  capacity:
    storage: 500Gi
  local:
    path: /mnt/swift/pv
  nodeAffinity:
    required:
      nodeSelectorTerms:
      - matchExpressions:
        - key: kubernetes.io/hostname
          operator: In
          values:
          - storage-host-0
  persistentVolumeReclaimPolicy: Delete
  storageClassName: swift-storage
  volumeMode: Filesystem
```

### Customizing Swift deployments

#### Additional settings
There are a couple of settings available to customize the Swift deployment.
Some of these did exist already when using TripleO/director. The following
options can be set in the CustomResource to change the default:

| Service/Option               | Description                                                                                 |
| ---------------------------- | ------------------------------------------------------------------------------------------- |
| swiftProxy/ceilometerEnabled | Enables the ceilometer middleware in the proxy server                                       |
| swiftProxy/encryptionEnabled | Enables object encryption using Barbican                                                    |
| swiftRing/minPartHours       | The minimum time (in hours) before a partition in a ring can be moved following a rebalance |
| swiftRing/partPower          | Partition Power to use when building Swift rings                                            |
| swiftRing/ringReplicas       | How many object replicas to use in the swift rings                                          |


#### Changing default service configuration
Configuration files for each service can be customized using the
`defaultConfigOverwrite` option in combination with using specific keys. For
example, to customize the object-server configuration you would specify options
using the key `01-object-server.conf`. The following keys can be used to
overwrite default settings:

| Service          | Key                      |
| ---------------- | ------------------------ |
| account-server   | 01-account-server.conf   |
| container-server | 01-container-server.conf |
| object-server    | 01-object-server.conf    |
| object-expirer   | 01-object-expirer.conf   |
| proxy-server     | 01-proxy-server.conf     |

#### Example configuration
The following CustomResource example shows how to use some of the options from
above:

```yaml
apiVersion: core.openstack.org/v1beta1
kind: OpenStackControlPlane
metadata:
  name: openstack-galera-network-isolation
  namespace: openstack
spec:
  ...
  swift:
    enabled: true
    template:
      swiftProxy:
        ceilometerEnabled: true
        encryptionEnabled: true
        replicas: 2
      swiftRing:
        minPartHours: 4
        partPower: 12
        ringReplicas: 3
      swiftStorage:
        replicas: 3
        storageClass: local-storage
        storageRequest: 10Gi
        defaultConfigOverwrite:
          01-object-server.conf: |
            [DEFAULT]
            workers = 4
```
