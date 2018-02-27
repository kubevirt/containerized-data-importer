package main
// importer.go implements a data fetching service capable of pulling objects from remote object stores
// and writing to a local directory.  It utilizes the minio-go client sdk.
// This process expects several environmental variables:
//    IMPORTER_ENDPOINT       Endpoint url minus scheme, bucket/object and port, eg. s3.amazon.com
// Access and secret keys are optional. If omitted no creds are passed to the object store client
//    IMPORTER_ACCESS_KEY_ID  Optional. Access key is the user ID that uniquely identifies your account.
//    IMPORTER_SECRET_KEY     Optional. Secret key is the password to your account


import (
	"flag"
	"net/url"

	"github.com/kubevirt/containerized-data-importer/pkg/common"
	"github.com/golang/glog"
	. "github.com/kubevirt/containerized-data-importer/pkg/importer"
)

func init() {
	flag.Parse()
}

func main() {
	defer glog.Flush()
	glog.Infoln("Starting importer")
	ep := ParseEnvVar(common.IMPORTER_ENDPOINT, false)
	acc := ParseEnvVar(common.IMPORTER_ACCESS_KEY_ID, false)
	sec := ParseEnvVar(common.IMPORTER_SECRET_KEY, false)
	importInfo, err := NewImportInfo(ep, acc, sec)
	if err != nil {
		glog.Fatalf("main: unable to get env variables: %v\n", err)
	}
	importInfo.Url, err = url.Parse(importInfo.Endpoint)
	if err != nil {
		glog.Fatalf("main(): \n")
	}
	dataReader, err := NewDataReader(importInfo)
	if err != nil {
		glog.Fatalf("main: unable to create data reader: %v\n", err)
	}
	defer dataReader.Close()
	glog.Infof("Beginning import from %s\n", importInfo.Url.RawPath)
	if err = StreamDataToFile(dataReader, common.IMPORTER_WRITE_PATH); err != nil {
		glog.Fatalf("main: unable to stream data to file: %v\n", err)
	}
	glog.Infoln("Import complete, exiting")
}
