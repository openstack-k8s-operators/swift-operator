/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package swift provides common constants and utilities for Swift services
package swift

const (
	// RunAsUser is the UID for running swift containers
	RunAsUser int64 = 42445
	// ProxyPort is the port for Swift proxy service
	ProxyPort int32 = 8081
	// ProxyHttpdPort is the HTTP port for Swift proxy
	ProxyHttpdPort int32 = 8080

	// AccountServerPort is the port for Swift account server
	AccountServerPort int32 = 6202
	// ContainerServerPort is the port for Swift container server
	ContainerServerPort int32 = 6201
	// ObjectServerPort is the port for Swift object server
	ObjectServerPort int32 = 6200
	// RsyncPort is the port for rsync service
	RsyncPort int32 = 873

	// ServiceName is the name of the Swift service
	ServiceName = "swift"
	// ServiceType is the OpenStack service type for Swift
	ServiceType = "object-store"
	// ServiceAccount is the service account name for Swift
	ServiceAccount = "swift-swift"
	// ServiceDescription is the description of the Swift service
	ServiceDescription = "Swift Object Storage"

	// ClaimName is the persistent volume claim name for Swift
	ClaimName = "swift"

	// SwiftConfSecretName is the name of the secret containing swift.conf
	// Must match with settings in the dataplane-operator and adoption docs/tests
	SwiftConfSecretName = "swift-conf"
)
