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

package swift

import "errors"

// Static errors for Swift operator
var (
	// ErrNetworkAttachmentsMismatch indicates pods don't have interfaces with IPs as configured in NetworkAttachments
	ErrNetworkAttachmentsMismatch = errors.New("not all pods have interfaces with ips as configured in NetworkAttachments")

	// ErrNoPodIPAddress indicates unable to get any IP address for pod
	ErrNoPodIPAddress = errors.New("unable to get any IP address for pod")

	// ErrPodIPAddressRetrieval indicates error while getting IP address from pod in network
	ErrPodIPAddressRetrieval = errors.New("error while getting IP address from pod in network")

	// ErrPVCNotBound indicates PersistentVolumeClaim found but not bound yet
	ErrPVCNotBound = errors.New("PersistentVolumeClaim found, but not bound yet")

	// ErrPodNotFound indicates pod not found
	ErrPodNotFound = errors.New("pod not found")

	// ErrPodNodeNameNotSet indicates pod found but NodeName not yet set
	ErrPodNodeNameNotSet = errors.New("pod found, but NodeName not yet set")
)
