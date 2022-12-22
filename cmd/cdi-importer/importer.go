package main

// importer.go implements a data fetching service capable of pulling objects from remote object
// stores and writing to a local directory. It utilizes the minio-go client sdk for s3 remotes,
// https for public remotes, and "file" for local files. The main use-case for this importer is
// to copy VM images to a "golden" namespace for consumption by kubevirt.
// This process expects several environmental variables:
//    ImporterEndpoint       Endpoint url minus scheme, bucket/object and port, eg. s3.amazon.com.
//			      Access and secret keys are optional. If omitted no creds are passed
//			      to the object store client.
//    ImporterAccessKeyID  Optional. Access key is the user ID that uniquely identifies your
//			      account.
//    ImporterSecretKey     Optional. Secret key is the password to your account.

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/importer"
	"kubevirt.io/containerized-data-importer/pkg/util"
	prometheusutil "kubevirt.io/containerized-data-importer/pkg/util/prometheus"
)

func init() {
	klog.InitFlags(nil)
	flag.Parse()
}

func waitForReadyFile() {
	const readyFileTimeoutSeconds = 60
	readyFile, _ := util.ParseEnvVar(common.ImporterReadyFile, false)
	if readyFile == "" {
		return
	}
	for i := 0; i < readyFileTimeoutSeconds; i++ {
		if _, err := os.Stat(readyFile); err == nil {
			return
		}
		time.Sleep(time.Second)
	}
	err := util.WriteTerminationMessage(fmt.Sprintf("Timeout waiting for file %s", readyFile))
	if err != nil {
		klog.Errorf("%+v", err)
	}
	os.Exit(1)
}

func getHTTPEp(ep string) string {
	readyFile, err := util.ParseEnvVar(common.ImporterReadyFile, false)
	if err != nil {
		klog.Errorf("Failed parsing env var %s: %+v", common.ImporterReadyFile, err)
		os.Exit(1)
	}
	if len(readyFile) == 0 {
		return ep
	}
	imageName, err := os.ReadFile(readyFile)
	if err != nil {
		klog.Errorf("Failed reading file %s: %+v", readyFile, err)
		os.Exit(1)
	}
	if len(imageName) == 0 {
		return ep
	}
	return strings.TrimSuffix(ep, common.DiskImageName) + string(imageName)
}

func touchDoneFile() {
	doneFile, _ := util.ParseEnvVar(common.ImporterDoneFile, false)
	if doneFile == "" {
		return
	}
	f, err := os.OpenFile(doneFile, os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		klog.Errorf("Failed creating file %s: %+v", doneFile, err)
	}
	f.Close()
}

func main() {
	defer klog.Flush()

	certsDirectory, err := os.MkdirTemp("", "certsdir")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(certsDirectory)
	prometheusutil.StartPrometheusEndpoint(certsDirectory)
	klog.V(1).Infoln("Starting importer")

	source, _ := util.ParseEnvVar(common.ImporterSource, false)
	contentType, _ := util.ParseEnvVar(common.ImporterContentType, false)
	imageSize, _ := util.ParseEnvVar(common.ImporterImageSize, false)
	filesystemOverhead, _ := strconv.ParseFloat(os.Getenv(common.FilesystemOverheadVar), 64)
	preallocation, err := strconv.ParseBool(os.Getenv(common.Preallocation))

	volumeMode := v1.PersistentVolumeBlock
	if _, err := os.Stat(common.WriteBlockPath); os.IsNotExist(err) {
		volumeMode = v1.PersistentVolumeFilesystem
	} else {
		preallocation = true
	}

	// With writeback cache mode it's possible that the process will exit before all writes have been commited to storage.
	// To guarantee that our write was commited to storage, we make a fsync syscall and ensure success.
	// Also might be a good idea to sync any chmod's we might have done.
	defer fsyncDataFile(contentType, volumeMode)

	//Registry import currently support kubevirt content type only
	if contentType != string(cdiv1.DataVolumeKubeVirt) && (source == cc.SourceRegistry || source == cc.SourceImageio) {
		klog.Errorf("Unsupported content type %s when importing from %s", contentType, source)
		os.Exit(1)
	}

	availableDestSpace, err := util.GetAvailableSpaceByVolumeMode(volumeMode)
	if err != nil {
		klog.Errorf("%+v", err)
		os.Exit(1)
	}
	if source == cc.SourceNone {
		err := handleEmptyImage(contentType, imageSize, availableDestSpace, preallocation, volumeMode, filesystemOverhead)
		if err != nil {
			klog.Errorf("%+v", err)
			os.Exit(1)
		}
	} else {
		waitForReadyFile()
		exitCode := handleImport(source, contentType, volumeMode, imageSize, filesystemOverhead, preallocation)
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}
}

func handleEmptyImage(contentType string, imageSize string, availableDestSpace int64, preallocation bool, volumeMode v1.PersistentVolumeMode, filesystemOverhead float64) error {
	var preallocationApplied bool

	if contentType == string(cdiv1.DataVolumeKubeVirt) {
		createBlankImage(imageSize, availableDestSpace, preallocation, volumeMode, filesystemOverhead)
		preallocationApplied = preallocation
	} else {
		errorEmptyDiskWithContentTypeArchive()
	}

	err := importCompleteTerminationMessage(preallocationApplied)
	return err
}

func handleImport(
	source string,
	contentType string,
	volumeMode v1.PersistentVolumeMode,
	imageSize string,
	filesystemOverhead float64,
	preallocation bool) int {
	klog.V(1).Infoln("begin import process")

	ds := newDataSource(source, contentType, volumeMode)
	defer ds.Close()

	processor := newDataProcessor(contentType, volumeMode, ds, imageSize, filesystemOverhead, preallocation)
	err := processor.ProcessData()

	if err != nil {
		klog.Errorf("%+v", err)
		if err == importer.ErrRequiresScratchSpace {
			return common.ScratchSpaceNeededExitCode
		}
		err = util.WriteTerminationMessage(fmt.Sprintf("Unable to process data: %+v", err.Error()))
		if err != nil {
			klog.Errorf("%+v", err)
		}

		return 1
	}
	touchDoneFile()
	// due to the way some data sources can add additional information to termination message
	// after finished (ds.close() ) termination message has to be written first, before the
	// the ds is closed
	// TODO: think about making communication explicit, probably DS interface should be extended
	err = importCompleteTerminationMessage(processor.PreallocationApplied())
	if err != nil {
		klog.Errorf("%+v", err)
		return 1
	}

	return 0
}

func importCompleteTerminationMessage(preallocationApplied bool) error {
	message := "Import Complete"
	if preallocationApplied {
		message += ", " + common.PreallocationApplied
	}
	err := util.WriteTerminationMessage(message)
	if err != nil {
		return err
	}

	klog.V(1).Infoln(message)
	return nil
}

func newDataProcessor(contentType string, volumeMode v1.PersistentVolumeMode, ds importer.DataSourceInterface, imageSize string, filesystemOverhead float64, preallocation bool) *importer.DataProcessor {
	dest := getImporterDestPath(contentType, volumeMode)
	processor := importer.NewDataProcessor(ds, dest, common.ImporterDataDir, common.ScratchDataDir, imageSize, filesystemOverhead, preallocation)
	return processor
}

func getImporterDestPath(contentType string, volumeMode v1.PersistentVolumeMode) string {
	dest := common.ImporterWritePath

	if contentType == string(cdiv1.DataVolumeArchive) {
		dest = common.ImporterVolumePath
	}
	if volumeMode == v1.PersistentVolumeBlock {
		dest = common.WriteBlockPath
	}

	return dest
}

func newDataSource(source string, contentType string, volumeMode v1.PersistentVolumeMode) importer.DataSourceInterface {
	ep, _ := util.ParseEnvVar(common.ImporterEndpoint, false)
	acc, _ := util.ParseEnvVar(common.ImporterAccessKeyID, false)
	sec, _ := util.ParseEnvVar(common.ImporterSecretKey, false)
	diskID, _ := util.ParseEnvVar(common.ImporterDiskID, false)
	uuid, _ := util.ParseEnvVar(common.ImporterUUID, false)
	backingFile, _ := util.ParseEnvVar(common.ImporterBackingFile, false)
	certDir, _ := util.ParseEnvVar(common.ImporterCertDirVar, false)
	insecureTLS, _ := strconv.ParseBool(os.Getenv(common.InsecureTLSVar))
	thumbprint, _ := util.ParseEnvVar(common.ImporterThumbprint, false)

	currentCheckpoint, _ := util.ParseEnvVar(common.ImporterCurrentCheckpoint, false)
	previousCheckpoint, _ := util.ParseEnvVar(common.ImporterPreviousCheckpoint, false)
	finalCheckpoint, _ := util.ParseEnvVar(common.ImporterFinalCheckpoint, false)

	switch source {
	case cc.SourceHTTP:
		ds, err := importer.NewHTTPDataSource(getHTTPEp(ep), acc, sec, certDir, cdiv1.DataVolumeContentType(contentType))
		if err != nil {
			errorCannotConnectDataSource(err, "http")
		}
		return ds
	case cc.SourceImageio:
		ds, err := importer.NewImageioDataSource(ep, acc, sec, certDir, diskID, currentCheckpoint, previousCheckpoint)
		if err != nil {
			errorCannotConnectDataSource(err, "imageio")
		}
		return ds
	case cc.SourceRegistry:
		ds := importer.NewRegistryDataSource(ep, acc, sec, certDir, insecureTLS)
		return ds
	case cc.SourceS3:
		ds, err := importer.NewS3DataSource(ep, acc, sec, certDir)
		if err != nil {
			errorCannotConnectDataSource(err, "s3")
		}
		return ds
	case cc.SourceVDDK:
		ds, err := importer.NewVDDKDataSource(ep, acc, sec, thumbprint, uuid, backingFile, currentCheckpoint, previousCheckpoint, finalCheckpoint, volumeMode)
		if err != nil {
			errorCannotConnectDataSource(err, "vddk")
		}
		return ds
	default:
		klog.Errorf("Unknown source type %s\n", source)
		err := util.WriteTerminationMessage(fmt.Sprintf("Unknown data source: %s", source))
		if err != nil {
			klog.Errorf("%+v", err)
		}
		os.Exit(1)
	}

	return nil
}

func createBlankImage(imageSize string, availableDestSpace int64, preallocation bool, volumeMode v1.PersistentVolumeMode, filesystemOverhead float64) {
	requestImageSizeQuantity := resource.MustParse(imageSize)
	minSizeQuantity := util.MinQuantity(resource.NewScaledQuantity(availableDestSpace, 0), &requestImageSizeQuantity)

	if minSizeQuantity.Cmp(requestImageSizeQuantity) != 0 {
		// Available dest space is smaller than the size we want to create
		klog.Warningf("Available space less than requested size, creating blank image sized to available space: %s.\n", minSizeQuantity.String())
	}

	var err error
	if volumeMode == v1.PersistentVolumeFilesystem {
		quantityWithFSOverhead := util.GetUsableSpace(filesystemOverhead, minSizeQuantity.Value())
		klog.Infof("Space adjusted for filesystem overhead: %d.\n", quantityWithFSOverhead)
		err = image.CreateBlankImage(common.ImporterWritePath, *resource.NewScaledQuantity(quantityWithFSOverhead, 0), preallocation)
	} else if volumeMode == v1.PersistentVolumeBlock && preallocation {
		klog.V(1).Info("Preallocating blank block volume")
		err = image.PreallocateBlankBlock(common.WriteBlockPath, minSizeQuantity)
	}

	if err != nil {
		klog.Errorf("%+v", err)
		message := fmt.Sprintf("Unable to create blank image: %+v", err)
		err = util.WriteTerminationMessage(message)
		if err != nil {
			klog.Errorf("%+v", err)
		}
		os.Exit(1)
	}
}

func errorCannotConnectDataSource(err error, dsName string) {
	klog.Errorf("%+v", err)
	err = util.WriteTerminationMessage(fmt.Sprintf("Unable to connect to %s data source: %+v", dsName, err))
	if err != nil {
		klog.Errorf("%+v", err)
	}
	os.Exit(1)
}

func errorEmptyDiskWithContentTypeArchive() {
	klog.Errorf("%+v", errors.New("Cannot create empty disk with content type archive"))
	err := util.WriteTerminationMessage("Cannot create empty disk with content type archive")
	if err != nil {
		klog.Errorf("%+v", err)
	}
	os.Exit(1)
}

func fsyncDataFile(contentType string, volumeMode v1.PersistentVolumeMode) {
	dataFile := getImporterDestPath(contentType, volumeMode)
	file, err := os.Open(dataFile)
	if err != nil {
		klog.Errorf("could not get file descriptor for fsync call: %+v", err)
		os.Exit(1)
	}
	if err := file.Sync(); err != nil {
		klog.Errorf("could not fsync following qemu-img writing: %+v", err)
		os.Exit(1)
	}
	klog.V(3).Infof("Successfully completed fsync(%s) syscall, commited to disk\n", dataFile)
	file.Close()
}
