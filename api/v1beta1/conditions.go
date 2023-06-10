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

package v1beta1

import (
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
)

// Swift Condition Types used by API objects.
const (
	// SwiftRingReadyCondition Status=True condition which indicates if the SwiftRing is configured and operational
	SwiftRingReadyCondition condition.Type = "SwiftRingReady"

	// SwiftStorageReadyCondition Status=True condition which indicates if the SwiftStorage is configured and operational
	SwiftStorageReadyCondition condition.Type = "SwiftStorageReady"

	// SwiftProxyReadyCondition Status=True condition which indicates if the SwiftProxy is configured and operational
	SwiftProxyReadyCondition condition.Type = "SwiftProxyReady"
)

// Common Messages used by API objects.
const (
	//
	// SwiftRingReady condition messages
	//
	// SwiftRingReadyInitMessage
	SwiftRingReadyInitMessage = "SwiftRing not started"

	// SwiftRingReadyErrorMessage
	SwiftRingReadyErrorMessage = "SwiftRing error occured %s"

	//
	// SwiftStorageReady condition messages
	//
	// SwiftStorageReadyInitMessage
	SwiftStorageReadyInitMessage = "SwiftStorage not started"

	// SwiftStorageReadyErrorMessage
	SwiftStorageReadyErrorMessage = "SwiftStorage error occured %s"

	//
	// SwiftProxyReady condition messages
	//
	// SwiftProxyReadyInitMessage
	SwiftProxyReadyInitMessage = "SwiftProxy not started"

	// SwiftProxyReadyErrorMessage
	SwiftProxyReadyErrorMessage = "SwiftProxy error occured %s"
)
