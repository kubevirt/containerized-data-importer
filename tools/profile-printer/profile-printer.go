//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

package main

import (
	"fmt"

	sc "kubevirt.io/containerized-data-importer/pkg/storagecapabilities"
)

func main() {
	fmt.Println("provisioner, accessMode, volumeMode")
	for k, v := range sc.CapabilitiesByProvisionerKey {
		fmt.Printf("%s", k)
		for _, caps := range v {
			fmt.Printf(", ")
			fmt.Printf("%s, %s", caps.AccessMode, caps.VolumeMode)
		}
		fmt.Println()
	}
}
