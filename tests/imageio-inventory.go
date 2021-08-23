package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"

	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

type imageIoInventoryData struct {
	DiskID          string
	StorageDomainID string
	Snapshots       []imageIoDiskSnapshot
}

type imageIoDiskSnapshot struct {
	SnapshotID   string
	SnapshotSize uint64
	SnapshotType string
	TransferURL  string
	TransferID   string
}

type imageIoMockResponse struct {
	ResponseBody string `json:"responseBody"`
	ResponseCode int    `json:"responseCode"`
	Times        int    `json:"times"`
}

type imageIoMockResponseSequence struct {
	Path      string                `json:"path"`
	Method    string                `json:"method"`
	Responses []imageIoMockResponse `json:"responses"`
}

type imageIoTicket struct {
	UUID    string   `json:"uuid"`
	Size    uint64   `json:"size"`
	URL     string   `json:"url"`
	Timeout uint64   `json:"timeout"`
	Ops     []string `json:"ops"`
}

// ResetImageIoInventory resets the fakeovirt inventory to the defaults.
// Accepts a list of configurators (see fakeovirt documentation): static-sso, static-vms, static-namespace, static-transfers
func ResetImageIoInventory(f *framework.Framework, configurators ...string) {
	imageioRootURL := fmt.Sprintf(utils.ImageioRootURL, f.CdiInstallNs)
	reset := imageioRootURL + "/reset"
	if len(configurators) > 0 {
		reset += "?configurators=" + strings.Join(configurators, ",")
	}

	// Find the imageio simulator pod
	pod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, "imageio-deployment", "app=imageio")
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(pod).ToNot(gomega.BeNil())

	_, err = RunKubectlCommand(f, "exec", "-n", pod.Namespace, pod.Name, "-c", "fakeovirt", "--", "/usr/bin/curl", "-s", "--cacert", "/app/imageio.crt", "-X", "POST", reset)
	gomega.Expect(err).To(gomega.BeNil())
}

// CreateImageIoWarmImportInventory constructs ImageIO inventory updates for a multi-stage import
func CreateImageIoWarmImportInventory(f *framework.Framework, diskID string, storageDomainID string, snapshots []string) {
	imageioImageURL := fmt.Sprintf(utils.ImageioImageURL, f.CdiInstallNs)

	var snapshotInfo []imageIoDiskSnapshot
	for index, snapshot := range snapshots {
		snapshotInfo = append(snapshotInfo, imageIoDiskSnapshot{
			SnapshotID:   snapshot,
			SnapshotSize: getSnapshotSize(snapshot),
			SnapshotType: getSnapshotType(snapshot),
			TransferURL:  fmt.Sprintf("%s/images/%s", imageioImageURL, snapshot),
			TransferID:   fmt.Sprintf("transfer-%d", index),
		})
	}
	data := &imageIoInventoryData{
		DiskID:          diskID,
		StorageDomainID: storageDomainID,
		Snapshots:       snapshotInfo,
	}

	// Reset fakeovirt inventory with just the SSO responses
	ResetImageIoInventory(f, "static-sso")

	// Find the imageio simulator pod
	pod, err := utils.FindPodByPrefix(f.K8sClient, f.CdiInstallNs, "imageio-deployment", "app=imageio")
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(pod).ToNot(gomega.BeNil())

	// Create sequence of responses to correctly run the importer through a multi-stage import
	responseSequenceJSON := createResponseSequences(data)
	postInventoryStubs(f, pod, responseSequenceJSON)

	// Upload disk images to imageiotest
	for _, snapshot := range data.Snapshots {
		copyDiskImage(f, pod, snapshot.SnapshotID)
		addTicket(f, pod, snapshot)
	}
}

// Create a disk response
func createDiskResponseBody(data *imageIoInventoryData) string {
	var disk bytes.Buffer
	diskTemplate, err := template.New("disk").Parse(`
		<disk href="/ovirt-engine/api/disks/{{.DiskID}}" id="{{.DiskID}}">
			<name>disk-{{.DiskID}}</name>
			<total_size>46137344</total_size>
			<storage_domains>
				<storage_domain href="/ovirt-engine/api/storagedomains/{{.StorageDomainID}}" id="{{.StorageDomainID}}"/>
			</storage_domains>
		</disk>
		`)
	gomega.Expect(err).To(gomega.BeNil())
	err = diskTemplate.Execute(&disk, data)
	gomega.Expect(err).To(gomega.BeNil())
	return disk.String()
}

// Create a disk snapshots response
func createDiskSnapshotsResponseBody(data *imageIoInventoryData) string {
	var diskSnapshots bytes.Buffer
	diskSnapshotsTemplate, err := template.New("disksnapshots").Parse(`
		<disk_snapshots>
			{{range .Snapshots}}
			<disk_snapshot href="/ovirt-engine/api/storagedomains/{{$.StorageDomainID}}/disksnapshots/{{.SnapshotID}}" id="{{.SnapshotID}}">
				<name>snapshot-{{.SnapshotID}}</name>
				<disk href="/ovirt-engine/api/disks/{{$.DiskID}}" id="{{$.DiskID}}"/>
				<format_type>{{.SnapshotType}}</format_type>
				<storage_domain href="/ovirt-engine/api/storagedomains/{{$.StorageDomainID}}" id="{{$.StorageDomainID}}"/>
				<storage_domains>
					<storage_domain href="/ovirt-engine/api/storagedomains/{{$.StorageDomainID}}" id="{{$.StorageDomainID}}"/>
				</storage_domains>
				<total_size>0</total_size>
				<actual_size>{{.SnapshotSize}}</actual_size>
			</disk_snapshot>
			{{end}}
		</disk_snapshots>
		`)
	gomega.Expect(err).To(gomega.BeNil())
	err = diskSnapshotsTemplate.Execute(&diskSnapshots, data)
	gomega.Expect(err).To(gomega.BeNil())
	return diskSnapshots.String()
}

// Create a storage domains response
func createStorageDomainsResponseBody(data *imageIoInventoryData) string {
	var storageDomains bytes.Buffer
	storageDomainsTemplate, err := template.New("storagedomains").Parse(`
		<storagedomains>
			<storagedomain href="/ovirt-engine/api/storagedomains/{{.StorageDomainID}}" id="{{.StorageDomainID}}">
				<name>storagedomain-{{.StorageDomainID}}</name>
				<link href="/ovirt-engine/api/storagedomains/{{.StorageDomainID}}/disksnapshots" rel="disksnapshots"/>
			</storagedomain>
		</storagedomains>
		`)
	gomega.Expect(err).To(gomega.BeNil())
	err = storageDomainsTemplate.Execute(&storageDomains, data)
	gomega.Expect(err).To(gomega.BeNil())
	return storageDomains.String()
}

// Create image transfer response
func createTransferResponseBody(snapshot *imageIoDiskSnapshot, phase string) string {
	var imageTransfer bytes.Buffer
	imageTransferTemplate, err := template.New("imagetransfer-" + snapshot.SnapshotID).Parse(`
			<image_transfer href="/ovirt-engine/api/imagetransfers/{{.TransferID}}" id="{{.TransferID}}">
				<direction>download</direction>
				<format>{{.SnapshotType}}</format>
				<phase>` + phase + `</phase>
				<transfer_url>{{.TransferURL}}</transfer_url>
			</image_transfer>
		`)
	gomega.Expect(err).To(gomega.BeNil())
	err = imageTransferTemplate.Execute(&imageTransfer, snapshot)
	gomega.Expect(err).To(gomega.BeNil())
	return imageTransfer.String()
}

func createResponseSequences(data *imageIoInventoryData) *bytes.Buffer {
	diskResponse := createDiskResponseBody(data)
	diskSnapshotsResponse := createDiskSnapshotsResponseBody(data)
	storageDomainsResponse := createStorageDomainsResponseBody(data)

	// Construct response sequences
	responseSequences := []imageIoMockResponseSequence{
		{
			Path:   fmt.Sprintf("/ovirt-engine/api/disks/%s", data.DiskID),
			Method: "GET",
			Responses: []imageIoMockResponse{
				{
					ResponseBody: diskResponse,
					ResponseCode: 200,
				},
			},
		},
		{
			Path:   fmt.Sprintf("/ovirt-engine/api/storagedomains/%s/disksnapshots", data.StorageDomainID),
			Method: "GET",
			Responses: []imageIoMockResponse{
				{
					ResponseBody: diskSnapshotsResponse,
					ResponseCode: 200,
				},
			},
		},
		{
			Path:   fmt.Sprintf("/ovirt-engine/api/storagedomains/%s", data.StorageDomainID),
			Method: "GET",
			Responses: []imageIoMockResponse{
				{
					ResponseBody: storageDomainsResponse,
					ResponseCode: 200,
				},
			},
		},
	}

	// Add responses to individual transfer finalize requests, just needs HTTP sucess
	for index, snapshot := range data.Snapshots {
		times := 1
		if index == 1 {
			times = 3 // Handle scratch space pod restarts on first real snapshot
		}
		responseSequences = append(responseSequences, imageIoMockResponseSequence{
			Path:   fmt.Sprintf("/ovirt-engine/api/imagetransfers/%s/finalize", snapshot.TransferID),
			Method: "POST",
			Responses: []imageIoMockResponse{
				{
					ResponseCode: 200,
					Times:        times,
				},
			},
		})
	}

	// Create responses for individual image transfers
	var transferResponses []imageIoMockResponse
	for index, snapshot := range data.Snapshots {
		times := 1
		if index == 1 {
			times = 3 // Handle scratch space pod restarts on first real snapshot
		}
		transferringResponse := createTransferResponseBody(&snapshot, "transferring")
		transferResponses = append(transferResponses, imageIoMockResponse{
			ResponseBody: transferringResponse,
			ResponseCode: 200,
			Times:        times,
		})
	}
	responseSequences = append(responseSequences, imageIoMockResponseSequence{
		Path:      "/ovirt-engine/api/imagetransfers",
		Method:    "POST",
		Responses: transferResponses,
	})

	// Create responses for final individual image transfer GETs
	for index, snapshot := range data.Snapshots {
		times := 1
		if index == 1 {
			times = 3 // Handle scratch space pod restarts on first real snapshot
		}
		finalizingResponse := createTransferResponseBody(&snapshot, "finalizing_success")
		responseSequences = append(responseSequences, imageIoMockResponseSequence{
			Path:   fmt.Sprintf("/ovirt-engine/api/imagetransfers/%s", snapshot.TransferID),
			Method: "GET",
			Responses: []imageIoMockResponse{
				{
					ResponseBody: finalizingResponse,
					ResponseCode: 200,
					Times:        times,
				},
			},
		})
	}

	// Encode JSON and return bytes to send to curl's stdin
	responseSequenceJSON := new(bytes.Buffer)
	encoder := json.NewEncoder(responseSequenceJSON)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(responseSequences)
	gomega.Expect(err).To(gomega.BeNil())

	return responseSequenceJSON
}

// Send inventory response sequences to fakeovirt stubs API
func postInventoryStubs(f *framework.Framework, pod *v1.Pod, stubs *bytes.Buffer) {
	imageioRootURL := fmt.Sprintf(utils.ImageioRootURL, f.CdiInstallNs)
	stub := imageioRootURL + "/stub"
	command := CreateKubectlCommand(f, "exec", "-n", pod.Namespace, pod.Name, "-c", "fakeovirt", "-i", "--", "/usr/bin/curl", "-s", "--cacert", "/app/imageio.crt", "-X", "POST", "-d", "@-", stub)
	command.Stdin = stubs
	command.Stdout = os.Stdout
	command.Stderr = command.Stdout
	err := command.Run()
	gomega.Expect(err).To(gomega.BeNil())
}

// Copy local disk image to imageiotest
func copyDiskImage(f *framework.Framework, pod *v1.Pod, name string) {
	path := getSnapshotPath(name)
	dest := fmt.Sprintf("%s:/images/%s", pod.Name, name)
	_, err := RunKubectlCommand(f, "cp", "-n", pod.Namespace, "-c", "imageiotest", path, dest)
	gomega.Expect(err).To(gomega.BeNil())
}

// Add ticket to imageiotest API, so importer can download it
func addTicket(f *framework.Framework, pod *v1.Pod, snapshot imageIoDiskSnapshot) {

	// Create ticket
	ticket := imageIoTicket{
		UUID:    snapshot.SnapshotID,
		Size:    snapshot.SnapshotSize,
		URL:     fmt.Sprintf("file:///images/%s", snapshot.SnapshotID),
		Timeout: 30000000000000,
		Ops:     []string{"read"},
	}

	// Encode as JSON
	ticketBytes := new(bytes.Buffer)
	encoder := json.NewEncoder(ticketBytes)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(ticket)
	gomega.Expect(err).To(gomega.BeNil())

	// Post to API in imageiotest container
	command := CreateKubectlCommand(f, "exec", "-i", "-n", pod.Namespace, pod.Name, "-c", "imageiotest", "--", "/usr/bin/curl", "-s", "--unix-socket", "/ovirt-imageio/daemon/test/daemon.sock", "-X", "PUT", "-d", "@-", fmt.Sprintf("http://localhost:12345/tickets/%s", snapshot.SnapshotID))
	command.Stdin = ticketBytes
	command.Stdout = os.Stdout
	command.Stderr = command.Stdout
	err = command.Run()
	gomega.Expect(err).To(gomega.BeNil())
}

// Get snapshot path from test images directory
func getSnapshotPath(name string) string {
	return fmt.Sprintf("../tests/images/%s", name)
}

// Get file size from test images directory
func getSnapshotSize(snapshot string) uint64 {
	path := getSnapshotPath(snapshot)
	info, err := os.Stat(path)
	gomega.Expect(err).To(gomega.BeNil())
	return uint64(info.Size())
}

// Get snapshot type from file extension, just raw or cow.
// This gets passed to the ImageIO transfer request.
func getSnapshotType(snapshot string) string {
	path := getSnapshotPath(snapshot)
	extension := filepath.Ext(path)
	if extension == "qcow2" {
		return "cow"
	}
	return "raw"
}
