# swift-operator
The swift-operator is an OpenShift Operator built using the
[Operator Framework for Go](https://github.com/operator-framework) and provides
an easy way to install and manage an OpenStack Swift installation on OpenShift.

## Available documentation
- [Configuration](docs/config.md)
- [Adoption](https://openstack-k8s-operators.github.io/data-plane-adoption/)
- [Design Decisions](docs/design-decisions.md)

## Getting Started
The easiest way is to deploy using the openstack-operator and [install_yamls](https://github.com/openstack-k8s-operators/install_yamls).

### Deploy using the openstack-operator
```sh
git clone https://github.com/openstack-k8s-operators/install_yamls.git ~/install_yamls/

cd ~/install_yamls/devsetup/
make download_tools
CPUS=12 MEMORY=25600 DISK=100 make crc
make crc_attach_default_interface

cd ~/install_yamls
make crc_storage input openstack_wait openstack_wait_deploy
```

### Accessing Swift using the CLI

You can use the `openstackclient` CLI to access the Swift deployment directly,
for example:

```sh
$ oc rsh openstackclient
$ openstack container create test
$ openstack object save test obj
```

If you want to use the `swift` CLI instead, use the openstackclient pod and set the required OS_ env variables:

```sh
$ oc rsh openstackclient

$ export OS_AUTH_URL=$(grep -o 'https.*' ~/.config/openstack/clouds.yaml)
$ export OS_USERNAME=$(grep -Po '(?<=username: ).*' ~/.config/openstack/clouds.yaml)
$ export OS_PROJECT_NAME=$(grep -Po '(?<=project_name: ).*' ~/.config/openstack/clouds.yaml)
$ export OS_PASSWORD=$(grep -Po '(?<=password: ").*(?=")' ~/.config/openstack/secure.yaml)
$ export OS_AUTH_VERSION=3
```

You can now use the `swift` command directly, for example:

```sh
$ swift stat -v
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
