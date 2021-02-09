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
	"flag"
	"fmt"
	"os"
	"path/filepath"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"kubevirt.io/containerized-data-importer/pkg/operator/resources/cluster"
	cdioperator "kubevirt.io/containerized-data-importer/pkg/operator/resources/operator"
	"kubevirt.io/containerized-data-importer/tools/util"
)

var (
	exportPath = flag.String("export-path", "", "")
)

// Export the CDI CRDs schemas from code to yaml.
func main() {
	flag.Parse()

	fmt.Printf("Exporting to directory: %s\n", *exportPath)
	if *exportPath != "" {
		if err := os.Mkdir(*exportPath, 0755); err != nil {
			if !os.IsExist(err) {
				fmt.Printf("error %s", err.Error())
				panic(err)
			}
		}
	}
	crds := make([]*extv1.CustomResourceDefinition, 0)
	crds = append(crds, cdioperator.NewCdiCrd())
	crds = append(crds, cluster.NewCdiConfigCrd())
	crds = append(crds, cluster.NewDataVolumeCrd())
	crds = append(crds, cluster.NewStorageProfileCrd())
	crds = append(crds, cluster.NewObjectTransferCrd())

	for _, crd := range crds {
		crdPath := filepath.Join(*exportPath, crd.GetObjectMeta().GetName())
		crdSchemaFile, err := os.Create(crdPath)
		if err != nil {
			panic(err)
		}
		crd.Spec.Conversion = nil
		util.MarshallObject(crd, crdSchemaFile)
	}
}
