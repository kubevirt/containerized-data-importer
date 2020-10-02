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

	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	serviceName   = "imageio"
	configMapName = serviceName + "-certs"
	certFile      = "tls.crt"
	keyFile       = "tls.key"
)

func main() {
	certDir := flag.String("certDir", "", "")
	klog.InitFlags(nil)
	flag.Parse()

	if err := utils.CreateCertForTestService(util.GetNamespace(), serviceName, configMapName, *certDir, certFile, keyFile); err != nil {
		klog.Fatal(errors.Wrapf(err, "populate certificate directory %s' errored: ", *certDir))
	}

	klog.Info("File initialization completed without error.")
}
