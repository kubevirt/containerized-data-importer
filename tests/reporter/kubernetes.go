package reporter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/onsi/ginkgo/v2"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	cdiClientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
)

func getMaxFailsFromEnv() int {
	maxFailsEnv := os.Getenv("REPORTER_MAX_FAILS")
	if maxFailsEnv == "" {
		fmt.Fprintf(os.Stderr, "defaulting to 10 reported failures\n")
		return 10
	}

	maxFails, err := strconv.Atoi(maxFailsEnv)
	if err != nil { // if the variable is set with a non int value
		fmt.Println("Invalid REPORTER_MAX_FAILS variable, defaulting to 10")
		return 10
	}

	fmt.Fprintf(os.Stderr, "Number of reported failures[%d]\n", maxFails)
	return maxFails
}

// KubernetesReporter is the struct that holds the report info.
type KubernetesReporter struct {
	FailureCount int
	ArtifactsDir string
	maxFails     int
}

// NewKubernetesReporter creates a new instance of the reporter.
func NewKubernetesReporter(artifactsDir string) *KubernetesReporter {
	return &KubernetesReporter{
		FailureCount: 0,
		ArtifactsDir: artifactsDir,
		maxFails:     getMaxFailsFromEnv(),
	}
}

// Dump dumps the current state of the cluster. The relevant logs are collected starting
// from the since parameter.
func (r *KubernetesReporter) Dump(kubeCli *kubernetes.Clientset, cdiClient *cdiClientset.Clientset, since time.Duration) {
	// If we got not directory, print to stderr
	if r.ArtifactsDir == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "Current failure count[%d]\n", r.FailureCount)
	if r.FailureCount > r.maxFails {
		return
	}

	// Can call this as many times as needed, if the directory exists, nothing happens.
	if err := os.MkdirAll(r.ArtifactsDir, 0777); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create directory: %v\n", err)
		return
	}

	r.logCSIDrivers(kubeCli)
	r.logDVs(cdiClient)
	r.logEvents(kubeCli, since)
	r.logNodes(kubeCli)
	r.logPVCs(kubeCli)
	r.logPVs(kubeCli)
	r.logStorageClasses(kubeCli)
	r.logPods(kubeCli)
	r.logServices(kubeCli)
	r.logEndpoints(kubeCli)
	r.logLogs(kubeCli, since)
}

// Cleanup cleans up the current content of the artifactsDir
func (r *KubernetesReporter) Cleanup() {
	// clean up artifacts from previous run
	if r.ArtifactsDir != "" {
		os.RemoveAll(r.ArtifactsDir)
	}
}

func getSerialPrefix() string {
	if ginkgo.CurrentSpecReport().IsSerial {
		return "serial_"
	}
	return ""
}

func (r *KubernetesReporter) logObjects(elements interface{}, name string) {
	f, err := os.OpenFile(filepath.Join(r.ArtifactsDir, fmt.Sprintf("%s%d_%s.log", getSerialPrefix(), r.FailureCount, name)),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open the file: %v", err)
		return
	}
	defer f.Close()

	j, err := json.MarshalIndent(elements, "", "    ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal %s: %v\n", name, err)
		return
	}
	fmt.Fprintln(f, string(j))
}

func (r *KubernetesReporter) logPods(kubeCli *kubernetes.Clientset) {
	pods, err := kubeCli.CoreV1().Pods(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch pods: %v\n", err)
		return
	}

	r.logObjects(pods, "pods")
}

func (r *KubernetesReporter) logServices(kubeCli *kubernetes.Clientset) {
	services, err := kubeCli.CoreV1().Services(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch services: %v\n", err)
		return
	}

	r.logObjects(services, "services")
}

func (r *KubernetesReporter) logEndpoints(kubeCli *kubernetes.Clientset) {
	endpoints, err := kubeCli.CoreV1().Endpoints(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch endpointss: %v\n", err)
		return
	}

	r.logObjects(endpoints, "endpoints")
}

func (r *KubernetesReporter) logNodes(kubeCli *kubernetes.Clientset) {
	nodes, err := kubeCli.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch nodes: %v\n", err)
		return
	}

	r.logObjects(nodes, "nodes")
}

func (r *KubernetesReporter) logPVs(kubeCli *kubernetes.Clientset) {
	pvs, err := kubeCli.CoreV1().PersistentVolumes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch pvs: %v\n", err)
		return
	}

	r.logObjects(pvs, "pvs")
}

func (r *KubernetesReporter) logPVCs(kubeCli *kubernetes.Clientset) {
	pvcs, err := kubeCli.CoreV1().PersistentVolumeClaims(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch pvcs: %v\n", err)
		return
	}

	r.logObjects(pvcs, "pvcs")
}

func (r *KubernetesReporter) logStorageClasses(kubeCli *kubernetes.Clientset) {
	scs, err := kubeCli.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch storage classes: %v\n", err)
		return
	}

	r.logObjects(scs, "storageclasses")
}

func (r *KubernetesReporter) logDVs(cdiClientset *cdiClientset.Clientset) {
	dvs, err := cdiClientset.CdiV1beta1().DataVolumes(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch datavolumes: %v\n", err)
		return
	}

	r.logObjects(dvs, "dvs")
}

func (r *KubernetesReporter) logCSIDrivers(kubeCli *kubernetes.Clientset) {
	csiDrivers, err := kubeCli.StorageV1().CSIDrivers().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch csidrivers: %v\n", err)
		return
	}

	r.logObjects(csiDrivers, "csidrivers")
}

func (r *KubernetesReporter) logEvents(kubeCli *kubernetes.Clientset, since time.Duration) {
	startTime := time.Now().Add(-since).Add(-5 * time.Second)

	events, err := kubeCli.CoreV1().Events(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return
	}

	e := events.Items
	sort.Slice(e, func(i, j int) bool {
		return e[i].LastTimestamp.After(e[j].LastTimestamp.Time)
	})

	eventsToPrint := v1.EventList{}
	for _, event := range e {
		if event.LastTimestamp.Time.After(startTime) {
			eventsToPrint.Items = append(eventsToPrint.Items, event)
		}
	}

	r.logObjects(eventsToPrint, "events")
}

func (r *KubernetesReporter) logLogs(kubeCli *kubernetes.Clientset, since time.Duration) {
	logsdir := filepath.Join(r.ArtifactsDir, "pods")

	if err := os.MkdirAll(logsdir, 0777); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create directory: %v\n", err)
		return
	}

	startTime := time.Now().Add(-since).Add(-5 * time.Second)

	pods, err := kubeCli.CoreV1().Pods(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch pods: %v\n", err)
		return
	}

	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			current, err := os.OpenFile(filepath.Join(logsdir, fmt.Sprintf("%s%d_%s_%s-%s.log", getSerialPrefix(), r.FailureCount, pod.Namespace, pod.Name, container.Name)), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to open the file: %v\n", err)
				return
			}
			defer current.Close()

			previous, err := os.OpenFile(filepath.Join(logsdir, fmt.Sprintf("%s%d_%s_%s-%s_previous.log", getSerialPrefix(), r.FailureCount, pod.Namespace, pod.Name, container.Name)), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to open the file: %v\n", err)
				return
			}
			defer previous.Close()

			logStart := metav1.NewTime(startTime)
			logs, err := kubeCli.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{SinceTime: &logStart, Container: container.Name}).DoRaw(context.TODO())
			if err == nil {
				fmt.Fprintln(current, string(logs))
			}

			logs, err = kubeCli.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{SinceTime: &logStart, Container: container.Name, Previous: true}).DoRaw(context.TODO())
			if err == nil {
				fmt.Fprintln(previous, string(logs))
			}
		}
	}
}
