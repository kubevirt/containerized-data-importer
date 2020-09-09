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

	"github.com/pkg/errors"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
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

func main() {
	defer klog.Flush()

	certsDirectory, err := ioutil.TempDir("", "certsdir")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(certsDirectory)
	prometheusutil.StartPrometheusEndpoint(certsDirectory)

	klog.V(1).Infoln("Starting importer")
	ep, _ := util.ParseEnvVar(common.ImporterEndpoint, false)
	acc, _ := util.ParseEnvVar(common.ImporterAccessKeyID, false)
	sec, _ := util.ParseEnvVar(common.ImporterSecretKey, false)
	source, _ := util.ParseEnvVar(common.ImporterSource, false)
	contentType, _ := util.ParseEnvVar(common.ImporterContentType, false)
	imageSize, _ := util.ParseEnvVar(common.ImporterImageSize, false)
	certDir, _ := util.ParseEnvVar(common.ImporterCertDirVar, false)
	insecureTLS, _ := strconv.ParseBool(os.Getenv(common.InsecureTLSVar))
	diskID, _ := util.ParseEnvVar(common.ImporterDiskID, false)
	uuid, _ := util.ParseEnvVar(common.ImporterUUID, false)
	backingFile, _ := util.ParseEnvVar(common.ImporterBackingFile, false)
	thumbprint, _ := util.ParseEnvVar(common.ImporterThumbprint, false)

	//Registry import currently support kubevirt content type only
	if contentType != string(cdiv1.DataVolumeKubeVirt) && (source == controller.SourceRegistry || source == controller.SourceImageio) {
		klog.Errorf("Unsupported content type %s when importing from %s", contentType, source)
		os.Exit(1)
	}

	volumeMode := v1.PersistentVolumeBlock
	if _, err := os.Stat(common.WriteBlockPath); os.IsNotExist(err) {
		volumeMode = v1.PersistentVolumeFilesystem
	}

	dest := common.ImporterWritePath
	if contentType == string(cdiv1.DataVolumeArchive) {
		dest = common.ImporterVolumePath
	}

	if volumeMode == v1.PersistentVolumeBlock {
		dest = common.WriteBlockPath
	}

	dataDir := common.ImporterDataDir
	availableDestSpace, err := util.GetAvailableSpaceByVolumeMode(volumeMode)
	if err != nil {
		klog.Errorf("%+v", err)
		os.Exit(1)
	}
	if source == controller.SourceNone && contentType == string(cdiv1.DataVolumeKubeVirt) {
		requestImageSizeQuantity := resource.MustParse(imageSize)
		minSizeQuantity := util.MinQuantity(resource.NewScaledQuantity(availableDestSpace, 0), &requestImageSizeQuantity)
		if minSizeQuantity.Cmp(requestImageSizeQuantity) != 0 {
			// Available dest space is smaller than the size we want to create
			klog.Warningf("Available space less than requested size, creating blank image sized to available space: %s.\n", minSizeQuantity.String())
		}
		err := image.CreateBlankImage(common.ImporterWritePath, minSizeQuantity)
		if err != nil {
			klog.Errorf("%+v", err)
			err = util.WriteTerminationMessage(fmt.Sprintf("Unable to create blank image: %+v", err))
			if err != nil {
				klog.Errorf("%+v", err)
			}
			os.Exit(1)
		}
	} else if source == controller.SourceNone && contentType == string(cdiv1.DataVolumeArchive) {
		klog.Errorf("%+v", errors.New("Cannot create empty disk with content type archive"))
		err = util.WriteTerminationMessage("Cannot create empty disk with content type archive")
		if err != nil {
			klog.Errorf("%+v", err)
		}
		os.Exit(1)
	} else {
		klog.V(1).Infoln("begin import process")
		var dp importer.DataSourceInterface
		switch source {
		case controller.SourceHTTP:
			dp, err = importer.NewHTTPDataSource(ep, acc, sec, certDir, cdiv1.DataVolumeContentType(contentType))
			if err != nil {
				klog.Errorf("%+v", err)
				err = util.WriteTerminationMessage(fmt.Sprintf("Unable to connect to http data source: %+v", err))
				if err != nil {
					klog.Errorf("%+v", err)
				}
				os.Exit(1)
			}
		case controller.SourceImageio:
			dp, err = importer.NewImageioDataSource(ep, acc, sec, certDir, diskID)
			if err != nil {
				klog.Errorf("%+v", err)
				err = util.WriteTerminationMessage(fmt.Sprintf("Unable to connect to imageio data source: %+v", err))
				if err != nil {
					klog.Errorf("%+v", err)
				}
				os.Exit(1)
			}
		case controller.SourceRegistry:
			dp = importer.NewRegistryDataSource(ep, acc, sec, certDir, insecureTLS)
		case controller.SourceS3:
			dp, err = importer.NewS3DataSource(ep, acc, sec)
			if err != nil {
				klog.Errorf("%+v", err)
				err = util.WriteTerminationMessage(fmt.Sprintf("Unable to connect to s3 data source: %+v", err))
				if err != nil {
					klog.Errorf("%+v", err)
				}
				os.Exit(1)
			}
		case controller.SourceVDDK:
			dp, err = importer.NewVDDKDataSource(ep, acc, sec, thumbprint, uuid, backingFile)
			if err != nil {
				klog.Errorf("%+v", err)
				err = util.WriteTerminationMessage(fmt.Sprintf("Unable to connect to vddk data source: %+v", err))
				if err != nil {
					klog.Errorf("%+v", err)
				}
				os.Exit(1)
			}
		default:
			klog.Errorf("Unknown source type %s\n", source)
			err = util.WriteTerminationMessage(fmt.Sprintf("Unknown data source: %s", source))
			if err != nil {
				klog.Errorf("%+v", err)
			}
			os.Exit(1)
		}
		defer dp.Close()
		processor := importer.NewDataProcessor(dp, dest, dataDir, common.ScratchDataDir, imageSize)
		err = processor.ProcessData()
		if err != nil {
			klog.Errorf("%+v", err)
			if err == importer.ErrRequiresScratchSpace {
				os.Exit(common.ScratchSpaceNeededExitCode)
			}
			err = util.WriteTerminationMessage(fmt.Sprintf("Unable to process data: %+v", err))
			if err != nil {
				klog.Errorf("%+v", err)
			}
			os.Exit(1)
		}
	}
	err = util.WriteTerminationMessage("Import Complete")
	if err != nil {
		klog.Errorf("%+v", err)
		os.Exit(1)
	}
	klog.V(1).Infoln("Import complete")
}
