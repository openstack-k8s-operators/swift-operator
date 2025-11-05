# Configuring and deploying OpenStack Swift

Swift can use either PersistentVolumes on OpenShift nodes or using disks on
external data plane nodes. Using external data plane nodes gives you more
flexibility and supports bigger storage deployments, whereas using
PersistentVolumes on OpenShift nodes is easier to deploy, but limited to a
single PV per node at the moment.

## Deploying Swift using PersistentVolumes storage

Default deployments should use at least 3 SwiftStorage replicas and 2
SwiftProxy replicas. Both values can be increased as needed to distribute
storage across more nodes and disks. The number of `ringReplicas` defines the
number of actual object copies in the cluster. As an example, you could set
`ringReplicas: 3` and `swiftStorage/replicas: 5`; in this case every object
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

## Deploying Swift storage on dataplane nodes

Swift storage services on dataplane nodes is especially useful when operating
large clusters with a lot of storage. In this case the Swift proxy service will
continue to run on the controlplane, and all Swift storage services will run on
dataplane nodes. It is recommended to not mix storage on PVs and dataplane
nodes permanently.

> **_NOTE:_** If there are no Swift dataplane nodes configured and `swiftStorage`
> replicas are set to 0 ring building is not possible (because there are no
> nodes/disks until dataplane nodes are defined), and as a result `swiftProxy`
> won't start until the dataplane is created. Therefore it is recommended to
> start Swift proxies after creating the dataplane and set both `swiftProxy` as
> well as `swiftStorage` replicas to 0 when creating the OpenStackControlPlane
> CR, for example:

```
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
        replicas: 0
      swiftRing:
        ringReplicas: 3
      swiftStorage:
        replicas: 0
```

An `OpenStackDataPlaneDeployment` can use one or more
`OpenStackDataPlaneNodeSet`. Each `OpenStackDataPlaneNodeSet` can use different
disk and configuration settings to allow fine-grained control. Further
information can be found in the
[OpenStack Data Plane Operator documentation](https://openstack-k8s-operators.github.io/openstack-operator/dataplane/).

To deploy and run Swift storage services on dataplane nodes, you need to create
an `OpenStackDataPlaneNodeSet` with the following properties:

1. Included `swift` service
2. List of disks to be used for storage in Swift

You also need to enable DNS forwarding to resolve dataplane hostnames within
the controlplane pods. First get the `clusterIP` of the resolver:

```
oc get svc dnsmasq-dns -o jsonpath=`{.spec.clusterIP}`
```

Next update the default DNS entry, adding the upstream resolver `clusterIP`:

```
apiVersion: operator.openshift.io/v1
kind: DNS
metadata:
  name: default
spec:
  servers:
  - name: swift
    zones:
    - storage.example.com
    forwardPlugin:
      policy: Random
      upstreams:
      - <clusterIP>
```


### Enabling Swift storage service on the dataplane nodes

The `swift` service has to be added to the `services` field list on the
`NodeSet` within an `OpenStackDataPlaneNodeSet` deployment. It should be added
to the end of the services list, for example:

```
    services:
    - repo-setup
    - bootstrap
    - download-cache
    - configure-network
    - validate-network
    - install-os
    - configure-os
    - ssh-known-hosts
    - run-os
    - reboot-os
    - install-certs
    - swift
```

This runs the required playbooks to configure Swift storage services.

### Define disks to be used by Swift on the dataplane nodes

If all nodes use the same type of disks these can be defined once in a global
`nodeTemplate` section within a `OpenStackDataPlaneNodeSet`. To allow fine
grained control on a per-node basis, disks can also be defined individually in
the `nodes` section.
Each disk can be assigned to a specific Swift region or zone; if the ring
management is enabled this will be taken into account to distribute replicas as
far as possible. Each disk should also be specified with a weight. If you don`t
use custom weights in your (existing) rings, these can be simply set to the
amount of GiB of the disks.
The following example shows a dataplane with 3 storage nodes, all of them using
2 disks, and the first node using an additional 3rd disk as well.

```
- apiVersion: dataplane.openstack.org/v1beta1
  kind: OpenStackDataPlaneNodeSet
  metadata:
    name: openstack-edpm-ipam
    namespace: openstack
  spec:
    ...
    networkAttachments:
    - ctlplane
    - storage
    nodeTemplate:
      ansible:
        ansibleVars:
          edpm_kernel_sysctl_extra_settings:
            net.ipv4.ip_unprivileged_port_start:
              value: 873
          edpm_swift_disks:
          - device: /dev/vdb
            path: /srv/node/vdb
            region: 0
            weight: 4000
            zone: 0
          - device: /dev/vdc
            path: /srv/node/vdc
            region: 0
            weight: 4000
            zone: 0
    nodes:
      edpm-swift-0:
        ansible:
          ansibleVars:
            edpm_swift_disks:
            - device: /dev/vdd
              path: /srv/node/vdd
              weight: 1000
        hostName: edpm-swift-0
        networks:
        - defaultRoute: true
          fixedIP: 192.168.122.100
          name: ctlplane
          subnetName: subnet1
        - name: internalapi
          subnetName: subnet1
        - name: storage
          subnetName: subnet1
        - name: tenant
          subnetName: subnet1
      edpm-swift-1:
        hostName: edpm-swift-1
        networks:
        - defaultRoute: true
          fixedIP: 192.168.122.101
          name: ctlplane
          subnetName: subnet1
        - name: internalapi
          subnetName: subnet1
        - name: storage
          subnetName: subnet1
        - name: tenant
          subnetName: subnet1
      edpm-swift-2:
        hostName: edpm-swift-2
        networks:
        - defaultRoute: true
          fixedIP: 192.168.122.102
          name: ctlplane
          subnetName: subnet1
        - name: internalapi
          subnetName: subnet1
        - name: storage
          subnetName: subnet1
        - name: tenant
          subnetName: subnet1
    ...
    services:
    - repo-setup
    - bootstrap
    - download-cache
    - configure-network
    - validate-network
    - install-os
    - configure-os
    - ssh-known-hosts
    - run-os
    - reboot-os
    - install-certs
    - swift
```



## Customizing Swift deployments

### Additional settings
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


### Changing default service configuration
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

### Example configuration
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


### Using externally managed Swift rings
By default Swift rings are created automatically and will include both PV and
dataplane devices. These rings will also be updated if the PVs or dataplane
disks are changing.
To allow more control about the rings, it is possible to disable the automatic
ring management. In this case you need to manage and rebelance the rings on
your own. To disable the automatic ring management, set
`spec.swift.template.swiftRing.enabled` to false in the OpenStackControlPlane,
for example:

```
apiVersion: core.openstack.org/v1beta1
kind: OpenStackControlPlane
metadata:
  name: openstack-galera-network-isolation
  namespace: openstack
spec:
  ...
  swift:
    template:
      swiftRing:
        enabled: false
  ...
```

Please note that you need to create the ring file ConfigMap with the required
ring files on your own in this case. You need to provide at least the following
files:

- account.ring.gz
- container.ring.gz
- object.ring.gz

These files must be added to the ConfigMap using the same names as keys. This
can be done easily if only these files are stored in a directory, for example:

```
oc create ConfigMap swift-ring-files --from-file=swift-ring-files/
```

Use the following command to update the ConfigMap:

```
oc create ConfigMap --dry-run=client -o yaml swift-ring-files --from-file=swift-ring-files/ | oc apply -f -
```


#### Resolving hostnames of manually managed ring nodes

Dataplane nodes will be automatically added to a `DNSData` resource to allow
resolving hostnames, which is required for Swift services to connect to storage
nodes. If you are using additional nodes that are not part of an
`OpenStackDataPlaneNodeSet` and use hostnames instead of IP addresses, you need
to create a custom `DNSData` resource.

For example:

```
apiVersion: network.openstack.org/v1beta1
kind: DNSData
metadata:
  name: openstack-external-swift-storage
  namespace: openstack
spec:
  dnsDataLabelSelectorValue: dnsdata
  hosts:
  - hostnames:
    - external-swift-0.storage.example.com
    ip: 172.18.2.100
  - hostnames:
    - external-swift-1.storage.example.com
    ip: 172.18.2.101
  - hostnames:
    - external-swift-2.storage.example.com
    ip: 172.18.2.102
```

#### Using larger rings
Rings are stored in ConfigMaps, and ConfigMaps itself are stored in etcd. The
size of a single ConfigMap is limited to 1MiB, and files must not be
more than 1MiB in total size therefore. For very large deployments or deployments
with a large number of Swift storage policies, this might not be sufficient. In
this case you can spread the rings over multiple ConfigMaps. The names of these
ConfigMaps need to be provided to the Swift template in this case, for example:

```
apiVersion: core.openstack.org/v1beta1
kind: OpenStackControlPlane
metadata:
  name: openstack-galera-network-isolation
  namespace: openstack
spec:
  ...
  swift:
    template:
      ringConfigMaps:
      - swift-ring-files-0
      - swift-ring-files-1
  ...
```

It is recommended to only store the `*.ring.gz` files to reduce required space
and disable automatic ring management by setting
`spec.swift.template.swiftRing.enabled` to false in the OpenStackControlPlane
(as described above).

#### Simplify usage of swift-ring-builder
`swift-ring-builder` is included in Swift, and using it is required to externally
manage rings. You can either install the required packages or source, or use an
alias, podman and container image. This makes it easy to use the exact same
version of `swift-ring-builder` that is used on the deployment itself.

The following defines a shell alias to swift-ring-builder, executed by podman
using a container image and mounts the current directory into the container:

```
alias swift-ring-builder="podman run -it --userns=keep-id:uid=42445 -v .:/etc/swift:Z -w /etc/swift quay.io/podified-antelope-centos9/openstack-swift-proxy-server:current-podified swift-ring-builder"
```

It is recommended to use a dedicated directory for the swift ring files and
change to this directory before using the alias. Once done, you can use
`swift-ring-builder` without installing Swift locally.

### Customizing Swift on dataplane nodes
Configuration files are included from the ConfigMaps and Secrets defined in the
service itself. The default service looks like this:

```
apiVersion: dataplane.openstack.org/v1beta1
kind: OpenStackDataPlaneService
metadata:
  name: swift-customized
spec:
  playbook: osp.edpm.swift
  dataSources:
    - secretRef:
        name: swift-conf
    - configMapRef:
        name: swift-storage-config-data
    - configMapRef:
        name: swift-ring-files
  edpmServiceType: swift
```

You can create a customized service and include additional ConfigMaps or
Secrets as needed to apply modified configurations as needed. This is also
needed if you deploy with multiple ring file ConfigMaps. In any case a custom
service name should be used, for example `swift-customized` in this case. The
same name needs to be used in the list of services `OpenStackDataPlaneNodeSet`.

Customizing the service also allows to apply different configuration to
different nodesets. For example, if you have two different type of nodes with
different performance characteristics, you might want to use different
configuration files with specifically tuned settings per `nodeSet`.
