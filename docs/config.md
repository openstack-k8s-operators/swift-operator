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

An `OpenStackDataPlaneDeployment` can use one or more
`OpenStackDataPlaneNodeSet`. Each `OpenStackDataPlaneNodeSet` can use different
disk and configuration settings to allow fine-grained control. Further
information can be found in the
[OpenStack Data Plane Operator documentation](https://openstack-k8s-operators.github.io/openstack-operator/dataplane/).

To deploy and run Swift storage services on dataplane nodes, you need to create
an `OpenStackDataPlaneNodeSet` with the following properties:

1. Included `swift` service
2. Sysctl setting to allow binding on port 873 for unprivileged rsync process
3. List of disks to be used for storage in Swift

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

### Add required sysctl setting for rsync

<!-- TODO: this section needs to be removed once this is set by default -->
Rsync is used to replicate data between nodes. It uses port 873 for this, which
is a privileged port by default. However, rsync is running unprivileged within
rootless podman, and thus needs an additional setting to allow binding to port
873.
This setting needs to be added to the `nodeTemplate` section, for example:

```
  nodeTemplate:
    ansible:
      ansibleVars:
        edpm_kernel_sysctl_extra_settings:
          net.ipv4.ip_unprivileged_port_start:
            value: 873
```

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
