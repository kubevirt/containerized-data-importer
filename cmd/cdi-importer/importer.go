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
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"github.com/pkg/errors"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
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
	readyFile, _ := util.ParseEnvVar(common.ImporterReadyFile, false)
	if readyFile == "" {
		return
	}
	for {
		if _, err := os.Stat(readyFile); err == nil {
			break
		}
		time.Sleep(time.Second)
	}
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

	certsDirectory, err := ioutil.TempDir("", "certsdir")
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
	var preallocationApplied bool
	var ds importer.DataSourceInterface

	//Registry import currently support kubevirt content type only
	if contentType != string(cdiv1.DataVolumeKubeVirt) && (source == controller.SourceRegistry || source == controller.SourceImageio) {
		klog.Errorf("Unsupported content type %s when importing from %s", contentType, source)
		os.Exit(1)
	}

	volumeMode := v1.PersistentVolumeBlock
	if _, err := os.Stat(common.WriteBlockPath); os.IsNotExist(err) {
		volumeMode = v1.PersistentVolumeFilesystem
	} else {
		preallocation = true
	}

	availableDestSpace, err := util.GetAvailableSpaceByVolumeMode(volumeMode)
	if err != nil {
		klog.Errorf("%+v", err)
		os.Exit(1)
	}
	if source == controller.SourceNone {
		if contentType == string(cdiv1.DataVolumeKubeVirt) {
			createBlankImage(imageSize, availableDestSpace, preallocation, volumeMode, filesystemOverhead)
			preallocationApplied = preallocation
		} else {
			errorEmptyDiskWithContentTypeArchive()
		}
	} else {
		klog.V(1).Infoln("begin import process")

		ds = newDataSource(source, contentType, volumeMode)
		defer ds.Close()

		processor := newDataProcessor(contentType, volumeMode, ds, imageSize, filesystemOverhead, preallocation)
		waitForReadyFile()
		err = processor.ProcessData()

		if err != nil {
			klog.Errorf("%+v", err)
			if err == importer.ErrRequiresScratchSpace {
				ds.Close()
				os.Exit(common.ScratchSpaceNeededExitCode)
			}
			err = util.WriteTerminationMessage(fmt.Sprintf("Unable to process data: %+v", err))
			if err != nil {
				klog.Errorf("%+v", err)
			}
			ds.Close()
			os.Exit(1)
		}
		touchDoneFile()
		preallocationApplied = processor.PreallocationApplied()
	}
	message := "Import Complete"
	if preallocationApplied {
		message += ", " + common.PreallocationApplied
	}
	err = util.WriteTerminationMessage(message)
	if err != nil {
		klog.Errorf("%+v", err)
		if ds != nil {
			ds.Close()
		}
		os.Exit(1)
	}
	klog.V(1).Infoln(message)
}

func newDataProcessor(contentType string, volumeMode v1.PersistentVolumeMode, ds importer.DataSourceInterface, imageSize string, filesystemOverhead float64, preallocation bool) *importer.DataProcessor {
	dest := common.ImporterWritePath
	if contentType == string(cdiv1.DataVolumeArchive) {
		dest = common.ImporterVolumePath
	}

	if volumeMode == v1.PersistentVolumeBlock {
		dest = common.WriteBlockPath
	}
	processor := importer.NewDataProcessor(ds, dest, common.ImporterDataDir, common.ScratchDataDir, imageSize, filesystemOverhead, preallocation)
	return processor
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
	case controller.SourceHTTP:
		ds, err := importer.NewHTTPDataSource(ep, acc, sec, certDir, cdiv1.DataVolumeContentType(contentType))
		if err != nil {
			errorCannotConnectDataSource(err, "http")
		}
		return ds
	case controller.SourceImageio:
		ds, err := importer.NewImageioDataSource(ep, acc, sec, certDir, diskID, currentCheckpoint, previousCheckpoint)
		if err != nil {
			errorCannotConnectDataSource(err, "imageio")
		}
		return ds
	case controller.SourceRegistry:
		ds := importer.NewRegistryDataSource(ep, acc, sec, certDir, insecureTLS)
		return ds
	case controller.SourceS3:
		ds, err := importer.NewS3DataSource(ep, acc, sec, certDir)
		if err != nil {
			errorCannotConnectDataSource(err, "s3")
		}
		return ds
	case controller.SourceVDDK:
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
		quantityWithFSOverhead := importer.GetUsableSpace(filesystemOverhead, minSizeQuantity.Value())
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
