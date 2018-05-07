package main

// importer.go implements a data fetching service capable of pulling objects from remote object
// stores and writing to a local directory. It utilizes the minio-go client sdk for s3 remotes,
// https for public remotes, and "file" for local files. The main use-case for this importer is
// to copy VM images to a "golden" namespace for consumption by kubevirt.
// This process expects several environmental variables:
//    IMPORTER_ENDPOINT       Endpoint url minus scheme, bucket/object and port, eg. s3.amazon.com.
//			      Access and secret keys are optional. If omitted no creds are passed
//			      to the object store client.
//    IMPORTER_ACCESS_KEY_ID  Optional. Access key is the user ID that uniquely identifies your
//			      account.
//    IMPORTER_SECRET_KEY     Optional. Secret key is the password to your account.

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/kubevirt/containerized-data-importer/pkg/common"
	"github.com/kubevirt/containerized-data-importer/pkg/image"
	. "github.com/kubevirt/containerized-data-importer/pkg/importer"
	. "github.com/kubevirt/containerized-data-importer/pkg/utils/errors"
)

func init() {
	flag.Parse()
}

func main() {
	defer glog.Flush()

	glog.Infoln("main: Starting importer")
	ep, err := ParseEndpoint("")
	if err != nil {
		glog.Errorf("main: endpoint error: %v\n", err)
		os.Exit(1)
	}
	acc := ParseEnvVar(common.IMPORTER_ACCESS_KEY_ID, false)
	sec := ParseEnvVar(common.IMPORTER_SECRET_KEY, false)
	fn := filepath.Base(ep.Path)
	if !image.IsSupporedFileType(fn) {
		Errf("main: unsupported source file %q. Supported types: %v\n", fn, image.SupportedFileExtensions).Log()
		os.Exit(1)
	}
	glog.Infof("main: beginning import from %q\n", ep.Path)
	dataStream := NewDataStream(ep, acc, sec)
	err = dataStream.Copy(common.IMPORTER_WRITE_PATH)
	if err != nil {
		glog.Errorf("main: copy error: %v\n", err)
		os.Exit(1)
	}
	glog.Infoln("main: Import complete, exiting")
}
