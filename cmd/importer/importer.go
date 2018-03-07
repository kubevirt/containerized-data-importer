package main

// importer.go implements a data fetching service capable of pulling objects from remote object stores
// and writing to a local directory.  It utilizes the minio-go client sdk.
// This process expects several environmental variables:
//    IMPORTER_ENDPOINT       Endpoint url minus scheme, bucket/object and port, eg. s3.amazon.com
// Access and secret keys are optional. If omitted no creds are passed to the object store client
//    IMPORTER_ACCESS_KEY_ID  Optional. Access key is the user ID that uniquely identifies your account.
//    IMPORTER_SECRET_KEY     Optional. Secret key is the password to your account

import (
	"net/url"
	"flag"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/kubevirt/containerized-data-importer/pkg/common"
	"github.com/kubevirt/containerized-data-importer/pkg/image"
	. "github.com/kubevirt/containerized-data-importer/pkg/importer"
)

func init() {
	flag.Parse()
}

func main() {
	defer glog.Flush()
	glog.Infoln("main: Starting importer")
	ep, err := url.Parse(ParseEnvVar(common.IMPORTER_ENDPOINT, false))
	if err != nil {
		glog.Fatalf("main: Error parsing endpoint %q: %v", ep, err)
	}
	fn := filepath.Base(ep.Path)
	if !image.IsValidImageFile(fn) {
		glog.Fatalf("main: unsupported source file %q. Supported extensions: %v", fn, image.SupportedFileExtensions)
	}
	if image.IsCompressed(fn) {
		glog.Infof("main: decompressing file %q/n", fn)
		if fn, err = image.Decompress(fn); err != nil {
			glog.Fatalf("main: decompress error: %v\n", err)
		}
	}
	glog.Infof("main: importing file %q\n", fn)
	acc := ParseEnvVar(common.IMPORTER_ACCESS_KEY_ID, false)
	sec := ParseEnvVar(common.IMPORTER_SECRET_KEY, false)
	dataStream, err := NewDataStream(ep, acc, sec).DataStreamSelector()
	if err != nil {
		glog.Fatalf("main: error getting data stream: %v", err)
	}
	defer dataStream.Close()
	glog.Infof("Beginning import from %s\n", ep)
	if err = StreamDataToFile(dataStream, common.IMPORTER_WRITE_PATH); err != nil {
		glog.Fatalf("main: unable to stream data to file: %v\n", err)
	}
	glog.Infoln("main: Import complete, exiting")
}
