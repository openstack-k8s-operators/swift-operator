# Design decisions

## Storage StatefulSet

Swift needs storage devices for the actual data, and these need to be
accessible using the same hostname (or IP) during their lifetime. A StatefulSet
in combination with a Headless Service is exactly what we need in this case.

From
[https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/):

> If you want to use storage volumes to provide persistence for your workload,
> you can use a StatefulSet as part of the solution. Although individual Pods
> in a StatefulSet are susceptible to failure, the persistent Pod identifiers
> make it easier to match existing volumes to the new Pods that replace any
> that have failed.

Swift requires quite a few services to access these PVs, and all of them are
running in a single pod.

Additionally, volumes are not deleted if the StatefulSet is deleted. This is a
very welcome safety limitation - an unwanted removal of the StatefulSet (or the
whole deployment) will not immediately result in a catastrophic data loss, but
can be recovered from (though this might need human interaction to fix).

### Headless service

The headless service makes it possible to access the storage pod (and
especially the `*-server` processes) directly by using a DNS name. For example,
if the pod name is `swift-storage-0` and the SwiftStorage instance is named
`swift-storage`, it becomes accessible using `swift-storage-0.swift-storage`.
This makes it easily usable within the Swift rings, and IP changes are now
transparent and don't require an update of the rings.


### NetworkPolicy & Labels

Swift backend services must only be accessible by other backend services and
the Swift proxy. To limit access, a NetworkPolicy is added to allow only
traffic between these pods. The NetworkPolicy itself depends on labels, and
these must match to allow traffic. Therefore labels must not be unique; instead
all pods must use the same label to allow access. This is also the reason why
the swift-operator is not using labels from
[lib-common](https://github.com/openstack-k8s-operators/lib-common) (yet).


## Swift rings

Swift services require information about the disks to use, and this includes
sizes (weights) and hostnames (or IPs). However, the sizes are not known when
starting the StatefulSet - but the StatefulSet requires rings to actually scale
up to the request size. It's basically a chicken-and-egg problem.

To solve this, an initial list of devices and their requested size will be
used. This CSV list of devices is then stored in a ConfigMap, and the ConfigMap
itself is watched by the SwiftRing instance. Once it is available (or changed),
it will trigger a rebalance job.

### Rebalance script

Rebalancing Swift rings requires the `swift-ring-builder` to be executed. Right
now this is done via a shell script; however this is just a dumb implementation
for now, as there is no check between current state and requested state. All
devices (PVs) are simply added every time it runs (ignoring existing ones),
and weights are simply set every time for every device.

A follow up refactoring will implement this in Python, directly using Swift
ringbuilder functions as well as python-requests to retrieve and update
ConfigMaps.

### Ring synchronization

Rings are stored in ConfigMaps, and these are mounted within the SwiftProxy and
SwiftStorage instances. An updated ConfigMap will also update these files and
they become available at their mountpoints.
However, all ring and builder files are stored in a tar file
`swiftrings.tar.gz`, and this needs to be unpacked and copied over to
`/etc/swift`. There is one container per SwiftProxy and SwiftStorage pod named
`ring-sync`, which actually copies over these files if the mtime did change.

This will also be improved to watch the ConfigMaps directly and only trigger an
update if there are changes.
