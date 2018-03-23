package main

// importer.go implements a data fetching service capable of pulling objects from remote object stores
// and writing to a local directory.  It utilizes the minio-go client sdk.
// This process expects several environmental variables:
//    IMPORTER_ENDPOINT       Endpoint url minus scheme, bucket/object and port, eg. s3.amazon.com
// Access and secret keys are optional. If omitted no creds are passed to the object store client
//    IMPORTER_ACCESS_KEY_ID  Optional. Access key is the user ID that uniquely identifies your account.
//    IMPORTER_SECRET_KEY     Optional. Secret key is the password to your account

import (
	"bytes"
	"flag"
	"io"
	"os"
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
	ep, err := ParseEndpoint("")
	if err != nil {
		glog.Errorf("main: endpoint error: %v\n", err)
		os.Exit(1)
	}
	acc := ParseEnvVar(common.IMPORTER_ACCESS_KEY_ID, false)
	sec := ParseEnvVar(common.IMPORTER_SECRET_KEY, false)
	fn := filepath.Base(ep.Path)
	if !image.IsSupporedFileType(fn) {
		glog.Errorf("main: unsupported source file %q. Supported types: %v\n", fn, image.SupportedFileExtensions)
		os.Exit(1)
	}

	// Initialize the input io stream (typically http or s3 client)
	glog.Infof("main: importing file %q\n", fn)
	dataStream, err := NewDataStream(ep, acc, sec)
	if err != nil {
		glog.Errorf("main: %q error: %v\n", ep.Path, err)
		os.Exit(1)
	}
	defer dataStream.DataRdr.Close()

	glog.Infof("Beginning import from %s\n", ep.Path)
	unpackedStream, err := image.UnpackData(fn, dataStream.DataRdr)
	if err != nil {
		glog.Errorf("main: %v\n", err)
		os.Exit(1)
	}

	// chkBuf is used to match first 4 bytes of io stream.
	chkBuf := make([]byte, 4)
	unpackedStream.Read(chkBuf)

	// Reconstruct the stream.  `dataStreamReader` is considered the new head of the stream.
	dataStreamReader := io.MultiReader(bytes.NewReader(chkBuf), unpackedStream)
	if image.MatchQcow2MagicNum(chkBuf) {
		// If the stream matches qcow2 format, write to /tmp/, then convert to raw disk in /data/.
		glog.Infoln("main: detected qcow2 magic number.")
		tmpFile := filepath.Join("/tmp", fn)
		err = StreamDataToFile(dataStreamReader, tmpFile)
		if err != nil {
			glog.Fatalf("main: error streaming data to file: %v\n", err)
		}
		err = image.ConvertQcow2ToRaw(tmpFile, common.IMPORTER_WRITE_PATH)
		if err != nil {
			glog.Fatalf("main: error converting qcow2 image: %v\n", err)
		}
		err = os.Remove(tmpFile)
		if err != nil {
			glog.Fatalf("main: error removing temp file %v: %v\n", tmpFile, err)
		}
	} else {
		// Otherwise, write directly to /data/
		if err = StreamDataToFile(dataStreamReader, common.IMPORTER_WRITE_PATH); err != nil {
			glog.Errorf("main: unable to stream data to file: %v\n", err)
			os.Exit(1)
		}
	}
	glog.Infoln("main: Import complete, exiting")
}
