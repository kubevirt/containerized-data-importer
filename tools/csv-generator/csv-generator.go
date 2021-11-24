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
	"os"

	cdicluster "kubevirt.io/containerized-data-importer/pkg/operator/resources/cluster"
	cdioperator "kubevirt.io/containerized-data-importer/pkg/operator/resources/operator"
	"kubevirt.io/containerized-data-importer/tools/util"
)

var (
	csvVersion         = flag.String("csv-version", "", "")
	replacesCsvVersion = flag.String("replaces-csv-version", "", "")
	namespace          = flag.String("namespace", "", "")
	pullPolicy         = flag.String("pull-policy", "", "")

	cdiLogoBase64 = flag.String("cdi-logo-base64", "", "")
	verbosity     = flag.String("verbosity", "1", "")

	operatorVersion = flag.String("operator-version", "", "")

	operatorImage     = flag.String("operator-image", "", "")
	controllerImage   = flag.String("controller-image", "", "")
	importerImage     = flag.String("importer-image", "", "")
	clonerImage       = flag.String("cloner-image", "", "")
	apiServerImage    = flag.String("apiserver-image", "", "")
	uploadProxyImage  = flag.String("uploadproxy-image", "", "")
	uploadServerImage = flag.String("uploadserver-image", "", "")
	dumpCRDs          = flag.Bool("dump-crds", false, "optional - dumps cdi-operator related crd manifests to stdout")
)

func main() {
	flag.Parse()

	data := cdioperator.ClusterServiceVersionData{
		CsvVersion:         *csvVersion,
		ReplacesCsvVersion: *replacesCsvVersion,
		Namespace:          *namespace,
		ImagePullPolicy:    *pullPolicy,
		IconBase64:         *cdiLogoBase64,
		Verbosity:          *verbosity,

		OperatorVersion: *operatorVersion,

		ControllerImage:   *controllerImage,
		ImporterImage:     *importerImage,
		ClonerImage:       *clonerImage,
		APIServerImage:    *apiServerImage,
		UplodaProxyImage:  *uploadProxyImage,
		UplodaServerImage: *uploadServerImage,
		OperatorImage:     *operatorImage,
	}

	csv, err := cdioperator.NewClusterServiceVersion(&data)
	if err != nil {
		panic(err)
	}
	util.MarshallObject(csv, os.Stdout)

	if *dumpCRDs {
		cidCrd := cdioperator.NewCdiCrd()
		util.MarshallObject(cidCrd, os.Stdout)
		dataImportCronCrd := cdicluster.NewDataImportCronCrd()
		util.MarshallObject(dataImportCronCrd, os.Stdout)
	}
}
