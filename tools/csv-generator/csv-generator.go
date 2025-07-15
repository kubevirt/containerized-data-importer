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

	operatorImage       = flag.String("operator-image", "", "")
	controllerImage     = flag.String("controller-image", "", "")
	importerImage       = flag.String("importer-image", "", "")
	clonerImage         = flag.String("cloner-image", "", "")
	apiServerImage      = flag.String("apiserver-image", "", "")
	uploadProxyImage    = flag.String("uploadproxy-image", "", "")
	uploadServerImage   = flag.String("uploadserver-image", "", "")
	ovirtPopulatorImage = flag.String("ovirt-populator-image", "", "")
	dumpCRDs            = flag.Bool("dump-crds", false, "optional - dumps cdi-operator related crd manifests to stdout")
	dumpNetworkPolicies = flag.Bool("dump-network-policies", false, "optional - dumps cdi-operator related network-policy manifests to stdout")
	dnsNamespace        = flag.String("dns-namespace", "kube-system", "optional - DNS namespace for the DNS network policy. Only used in conjunction with the dump-network-policies option")
	dnsLabelKey         = flag.String("dns-pod-selector-label", "k8s-app", "optional - DNS pod selector label key for the DNS network policy. Only used in conjunction with the dump-network-policies option")
	dnsLabelValue       = flag.String("dns-pod-selector-value", "kube-dns", "optional - DNS pod selector label value for the DNS network policy. Only used in conjunction with the dump-network-policies option")
	apiNamespace        = flag.String("api-namespace", "kube-system", "optional - kube-apiserver namespace for the api network policy. Only used in conjunction with the dump-network-policies option")
	apiLabelKey         = flag.String("api-pod-selector-label", "component", "optional - kube-apiserver pod selector label key for the api network policy. Only used in conjunction with the dump-network-policies option")
	apiLabelValue       = flag.String("api-pod-selector-value", "kube-apiserver", "optional - kube-apiserver pod selector label value for the api network policy. Only used in conjunction with the dump-network-policies option")
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

		ControllerImage:     *controllerImage,
		ImporterImage:       *importerImage,
		ClonerImage:         *clonerImage,
		OvirtPopulatorImage: *ovirtPopulatorImage,
		APIServerImage:      *apiServerImage,
		UplodaProxyImage:    *uploadProxyImage,
		UplodaServerImage:   *uploadServerImage,
		OperatorImage:       *operatorImage,
	}

	csv, err := cdioperator.NewClusterServiceVersion(&data)
	if err != nil {
		panic(err)
	}
	if err = util.MarshallObject(csv, os.Stdout); err != nil {
		panic(err)
	}

	if *dumpCRDs {
		cidCrd := cdioperator.NewCdiCrd()
		if err = util.MarshallObject(cidCrd, os.Stdout); err != nil {
			panic(err)
		}
	}

	if *dumpNetworkPolicies {
		cdiNPs := cdioperator.NewCdiNetworkPolicies(data.Namespace, *dnsNamespace, *dnsLabelKey, *dnsLabelValue, *apiNamespace, *apiLabelKey, *apiLabelValue)
		for _, v := range cdiNPs {
			if err = util.MarshallObject(v, os.Stdout); err != nil {
				panic(err)
			}
		}
	}
}
