package tests

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"

	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	clientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
)

var KubectlPath = ""
var OcPath = ""
var CDIInstallNamespace = "kube-system"

var (
	kubeconfig string
	master     string
)

func init() {
	flag.StringVar(&KubectlPath, "kubectl-path", "", "Set path to kubectl binary")
	flag.StringVar(&OcPath, "oc-path", "", "Set path to oc binary")
	flag.StringVar(&CDIInstallNamespace, "installed-namespace", "kube-system", "Set the namespace CDI is installed in")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
}

// CDIFailHandler call ginkgo.Fail with printing the additional information
func CDIFailHandler(message string, callerSkip ...int) {
	if len(callerSkip) > 0 {
		callerSkip[0]++
	}
	Fail(message, callerSkip...)
}

func RunKubectlCommand(args ...string) (string, error) {
	kubeconfig := flag.Lookup("kubeconfig").Value
	if kubeconfig == nil || kubeconfig.String() == "" {
		return "", fmt.Errorf("can not find kubeconfig")
	}

	master := flag.Lookup("master").Value
	if master != nil && master.String() != "" {
		args = append(args, "--server", master.String())
	}

	cmd := exec.Command(KubectlPath, args...)
	kubeconfEnv := fmt.Sprintf("KUBECONFIG=%s", kubeconfig.String())
	cmd.Env = append(os.Environ(), kubeconfEnv)

	stdOutBytes, err := cmd.Output()
	if err != nil {
		return string(stdOutBytes), err
	}
	return string(stdOutBytes), nil
}

func SkipIfNoKubectl() {
	if KubectlPath == "" {
		Skip("Skip test that requires kubectl binary")
	}
}

func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

// Gets an instance of a kubernetes client that includes all the CDI extensions.
func GetCDIClientOrDie() *clientset.Clientset {

	cfg, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	PanicOnError(err)
	cdiClient, err := clientset.NewForConfig(cfg)
	PanicOnError(err)

	return cdiClient
}

func GetKubeClient() (*kubernetes.Clientset, error) {
	return GetKubeClientFromFlags(master, kubeconfig)
}

func GetKubeClientFromFlags(master string, kubeconfig string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		return nil, err
	}
	return GetKubeClientFromRESTConfig(config)
}

func GetKubeClientFromRESTConfig(config *rest.Config) (*kubernetes.Clientset, error) {
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON

	return kubernetes.NewForConfig(config)
}

func CreatePVC(namespace string, name string, size string) *k8sv1.PersistentVolumeClaim {
	client, err := GetKubeClient()
	PanicOnError(err)
	pvc, err := client.CoreV1().PersistentVolumeClaims(namespace).Create(newPVC(name, size))
	PanicOnError(err)
	return pvc
}

func DeletePVC(namespace string, pvc *k8sv1.PersistentVolumeClaim) {
	client, err := GetKubeClient()
	PanicOnError(err)
	err = client.CoreV1().PersistentVolumeClaims(namespace).Delete(pvc.GetName(), nil)
	PanicOnError(err)
}

func newPVC(name string, size string) *k8sv1.PersistentVolumeClaim {
	quantity, err := resource.ParseQuantity(size)
	PanicOnError(err)

	return &k8sv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: k8sv1.PersistentVolumeClaimSpec{
			AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
			Resources: k8sv1.ResourceRequirements{
				Requests: k8sv1.ResourceList{
					"storage": quantity,
				},
			},
		},
	}
}

func RunPodWithPVC(namespace string, podName string, pvc *k8sv1.PersistentVolumeClaim, cmd string) {
	client, err := GetKubeClient()
	PanicOnError(err)
	_, err = client.CoreV1().Pods(namespace).Create(newCmdPod(podName, pvc.GetName(), cmd))
	PanicOnError(err)
}

func DeletePod(namespace string, name string) {
	client, err := GetKubeClient()
	PanicOnError(err)
	err = client.CoreV1().Pods(namespace).Delete(name, nil)
	PanicOnError(err)
}

func newCmdPod(name string, pvcName string, cmd string) *k8sv1.Pod {
	return &k8sv1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: k8sv1.PodSpec{
			RestartPolicy: k8sv1.RestartPolicyNever,
			Containers: []k8sv1.Container{
				{
					Name:    "runner",
					Image:   "fedora:28",
					Command: []string{"/bin/bash", "-c", cmd},
					VolumeMounts: []k8sv1.VolumeMount{
						{
							Name:      "pvc",
							MountPath: "/pvc",
						},
					},
				},
			},
			Volumes: []k8sv1.Volume{
				{
					Name: "pvc",
					VolumeSource: k8sv1.VolumeSource{
						PersistentVolumeClaim: &k8sv1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}
}
