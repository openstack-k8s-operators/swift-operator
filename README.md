# swift-operator
The swift-operator is an OpenShift Operator built using the
[Operator Framework for Go](https://github.com/operator-framework) and provides
an easy way to install and manage an OpenStack Swift installation on OpenShift.

Some useful information about the implementation might be found in the [Design
Decisisons](docs/design-decisions.md).

## Getting Started
You’ll need an OpenShift cluster to run against, for example [Red Hat CodeReady Containers](https://access.redhat.com/documentation/en-us/red_hat_codeready_containers/2.0/html/getting_started_guide/index).

You also need a running Keystone instance and a few PersistentVolumes. There
are two simple ways to test this, both are using [install_yamls](https://github.com/openstack-k8s-operators/install_yamls).


### Setup CRC
```sh
git clone https://github.com/openstack-k8s-operators/install_yamls
pushd install_yaml/devsetup
CPUS=12 MEMORY=25600 DISK=100 make crc
eval $(crc oc-env)
make crc_attach_default_interface
popd
```

### Deploy Swift
There are multiple ways to deploy Swift, one running only Keystone, MariaDB and
Swift and another one running a full OpenStack deployment using the
openstack-operator.

#### Deploy without using the openstack-operator
```sh
pushd install_yaml
# Setup PVs and deploy the operators
make crc_storage mariadb keystone swift
# Once operators are ready, deploy services
make mariadb_deploy keystone_deploy swift_deploy
popd
```

Once everything is ready a couple of pods should run including Swift:
```sh
$ oc get pods
NAME                           READY   STATUS      RESTARTS   AGE
keystone-6c76bbbd67-ctk6p      1/1     Running     0          69s
keystone-bootstrap-5x7r2       0/1     Completed   0          78s
keystone-db-create-lk5zv       0/1     Completed   0          99s
keystone-db-sync-ncntj         0/1     Completed   0          89s
mariadb-openstack              1/1     Running     0          100s
openstack-db-init-z8vgs        0/1     Completed   0          2m
swift-proxy-749fffd9f9-zjf57   3/3     Running     0          77s
swift-ring-rebalance-46vnr     0/1     Completed   0          93s
swift-storage-0                16/16   Running     0          2m2s
```

#### Deploy using the openstack-operator
```sh
pushd install_yaml
# Setup PVs and deploy the operators
make crc_storage input openstack
# Once operators are ready, deploy services
make openstack_deploy
popd
```

### Run swiftclient inside the cluster
If you want to use the `swift` CLI inside the cluster, deploy another pod:

```sh
oc apply -f https://raw.githubusercontent.com/openstack-k8s-operators/swift-operator/main/config/samples/swiftclient.yaml
```

Now you can run regular `swift` CLI commands within the cluster:

```sh
$ oc rsh openstackclient swift stat -v
            StorageURL: http://swift-public-openstack.apps-crc.testing/v1/AUTH_ebc5...
            Auth Token: gAAAAABklVH8utLpwDdfR_2_vDd-aYasUFzjoca7yo_YME8RUbiwyqhK6qp...
               Account: AUTH_ebc5787630474735905073de1fcd675f
            Containers: 0
               Objects: 0
                 Bytes: 0
          Content-Type: text/plain; charset=utf-8
           X-Timestamp: 1687507453.27883
       X-Put-Timestamp: 1687507453.27883
                  Vary: Accept
            X-Trans-Id: txbb13222da0d94527a7101-00649551fc
X-Openstack-Request-Id: txbb13222da0d94527a7101-00649551fc
            Set-Cookie: 8555ec7bc3b761f9a531b621867c3563=e9a23689a3c471db8c6babe532...
         Cache-Control: private
```


## TODO

- [ ] Improve reconciliation
- [ ] Fix hashes and status conditions
- [ ] Add node/pod affinity to ensure storage pods are running on different nodes
- [ ] Use more than one PV per storage pod
- [ ] Replace simple rebalance script with a smarter Python-based tool
- [ ] Refactor conf file copying, Secret and ConfigMap usage
- [ ] Use Kolla
- [ ] Support Network isolation
- [ ] Use memcached if available within cluster

## License

Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
