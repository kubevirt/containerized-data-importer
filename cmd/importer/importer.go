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

	glog.Infof("Beginning import from %q\n", ep.Path)
	unpackedStream, err := image.UnpackData(fn, dataStream.DataRdr)
	if err != nil {
		glog.Errorf("main: %v\n", err)
		os.Exit(1)
	}

	magicStr, err := image.GetMagicNumber(unpackedStream)
	if err != nil {
		glog.Errorf("main: %v\n", err)
		os.Exit(1)
	}
	qemu := image.MatchQcow2MagicNum(magicStr)

	// Don't lose bytes read in getting the magic number. MultiReader reads from each
	// passed-in reader in order until the last reader returns eof.
	dataStreamReader := io.MultiReader(bytes.NewReader(magicStr), unpackedStream)

	// copy image file
	out := common.IMPORTER_WRITE_PATH
	if qemu {
		// copy to tmp; the qemu conversion will write to final destination
		out = filepath.Join("/tmp", fn)
	}
	err = StreamDataToFile(dataStreamReader, out)
	if err != nil {
		glog.Errorf("main: unable to stream data to file %q: %v\n", out, err)
		os.Exit(1)
	}
	if qemu {
		glog.Infoln("main: converting qcow2 image to raw")
		err = image.ConvertQcow2ToRaw(out, common.IMPORTER_WRITE_PATH)
		if err != nil {
			glog.Fatalf("main: error converting qcow2 image: %v\n", err)
		}
		err = os.Remove(out)
		if err != nil {
			glog.Fatalf("main: error removing temp file %v: %v\n", out, err)
		}
	}
	glog.Infoln("main: Import complete, exiting")
}
