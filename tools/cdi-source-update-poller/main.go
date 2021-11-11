package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	cdiClientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

var (
	configPath    string
	kubeURL       string
	cronNamespace string
	cronName      string
	url           string
	certDir       string
	accessKey     string
	secretKey     string
	insecureTLS   bool
)

func init() {
	flag.StringVar(&configPath, "kubeconfig", os.Getenv("KUBECONFIG"), "(Optional) Overrides $KUBECONFIG.")
	flag.StringVar(&kubeURL, "server", "", "(Optional) URL address of a remote api server.  Do not set for local clusters.")
	flag.StringVar(&cronNamespace, "ns", "", "DataImportCron namespace.")
	flag.StringVar(&cronName, "cron", "", "DataImportCron name.")
	flag.StringVar(&url, "url", "", "registry source url.")
	flag.StringVar(&certDir, "certdir", "", "registry certificates path.")
	flag.Parse()
	if url == "" || cronNamespace == "" || cronName == "" {
		log.Fatalf("One or more mandatory parameters are missing")
	}
	accessKey, _ = util.ParseEnvVar(common.ImporterAccessKeyID, false)
	secretKey, _ = util.ParseEnvVar(common.ImporterSecretKey, false)
	insecureTLS, _ = strconv.ParseBool(os.Getenv(common.InsecureTLSVar))
}

func cmdRun(cmd string) (outStr string, err error) {
	var out, stderr bytes.Buffer
	command := exec.Command("sh", "-c", cmd)
	command.Stdout = &out
	command.Stderr = &stderr
	err = command.Run()
	if err != nil {
		log.Printf("Failed to exec command \"%s\": %v: %s", cmd, err, stderr.String())
	}
	outStr = out.String()
	return
}

func main() {
	cmd := "skopeo inspect"
	if insecureTLS {
		cmd += " --tls-verify=false"
	}
	if certDir != "" {
		cmd += " --cert-dir " + certDir
	}
	if accessKey != "" {
		cmd += " --creds " + accessKey
		if secretKey != "" {
			cmd += ":" + secretKey
		}
	}
	cmd += " " + url
	out, err := cmdRun(cmd)
	if err != nil {
		os.Exit(1)
	}
	fmt.Println("out:", out)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		log.Fatalf("Failed Unmarshal: %v", err)
	}
	digest, ok := result["Digest"].(string)
	if !ok {
		log.Fatalf("Failed reading digest")
	}
	fmt.Println("Digest is", digest)

	cfg, err := clientcmd.BuildConfigFromFlags(kubeURL, configPath)
	if err != nil {
		log.Fatalf("Failed BuildConfigFromFlags, kubeURL %s configPath %s: %v", kubeURL, configPath, err)
	}
	cdiClient, err := cdiClientset.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Failed NewForConfig: %v", err)
	}

	dataImportCron, err := cdiClient.CdiV1beta1().DataImportCrons(cronNamespace).Get(context.TODO(), cronName, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Failed getting DataImportCron, cronNamespace %s cronName %s: %v", cronNamespace, cronName, err)
	}
	imports := dataImportCron.Status.CurrentImports
	if digest != "" && (imports == nil || digest != imports[0].Digest) &&
		digest != dataImportCron.Annotations[controller.AnnSourceDesiredDigest] {
		if dataImportCron.Annotations == nil {
			dataImportCron.Annotations = make(map[string]string)
		}
		dataImportCron.Annotations[controller.AnnSourceDesiredDigest] = digest
		dataImportCron, err = cdiClient.CdiV1beta1().DataImportCrons(cronNamespace).Update(context.TODO(), dataImportCron, metav1.UpdateOptions{})
		if err != nil {
			log.Fatalf("Failed updating DataImportCron, cronNamespace %s cronName %s: %v", cronNamespace, cronName, err)
		}
		fmt.Println("Updated DataImportCron", dataImportCron.Name)
	} else {
		fmt.Println("No update to DataImportCron", dataImportCron.Name)
	}
}
