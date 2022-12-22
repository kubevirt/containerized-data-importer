package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	cdiClientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/importer"
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

func main() {
	allCertDir, err := importer.CreateCertificateDir(certDir)
	if err != nil {
		log.Printf("Ignore common certificate dir: %v", err)
		allCertDir = certDir
	}

	digest, err := importer.GetImageDigest(url, accessKey, secretKey, allCertDir, insecureTLS)
	if err != nil {
		log.Fatalf("Failed to get image digest: %v", err)
	}
	log.Printf("Digest is %s", digest)

	cfg, err := clientcmd.BuildConfigFromFlags(kubeURL, configPath)
	if err != nil {
		log.Fatalf("Failed to build config; kubeURL %s configPath %s: %v", kubeURL, configPath, err)
	}

	// Don't proxy k8s api calls
	cfg.Proxy = func(r *http.Request) (*neturl.URL, error) {
		return nil, nil
	}

	cdiClient, err := cdiClientset.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Failed to create Clientset: %v", err)
	}

	dataImportCron, err := cdiClient.CdiV1beta1().DataImportCrons(cronNamespace).Get(context.TODO(), cronName, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Failed getting DataImportCron %s/%s: %v", cronNamespace, cronName, err)
	}
	cc.AddAnnotation(dataImportCron, controller.AnnLastCronTime, time.Now().Format(time.RFC3339))

	imports := dataImportCron.Status.CurrentImports
	if digest != "" && (imports == nil || digest != imports[0].Digest) &&
		digest != dataImportCron.Annotations[controller.AnnSourceDesiredDigest] {
		cc.AddAnnotation(dataImportCron, controller.AnnSourceDesiredDigest, digest)
		log.Printf("Digest updated")
	} else {
		log.Printf("No digest update")
	}

	_, err = cdiClient.CdiV1beta1().DataImportCrons(cronNamespace).Update(context.TODO(), dataImportCron, metav1.UpdateOptions{})
	if err != nil {
		log.Fatalf("Failed updating DataImportCron %s/%s: %v", cronNamespace, cronName, err)
	}
}
