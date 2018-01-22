package main

import (
	"flag"
	"github.com/golang/glog"
	"os"
	"strings"
	"io"
)

// importer.go implements a data fetching service capable of pulling objects from remote object stores
// and writing to a local directory.  It utilizes the minio-go client sdk.
// This process expects several environmental variables:
//    IMPORTER_ENDPOINT       URL of object server
//    IMPORTER_OBJECT_PATH    Full path of object (e.g. bucket/object)
//    IMPORTER_ACCESS_KEY_ID  Secret key is the password to your account
//    IMPORTER_SECRET_KEY     Access key is the user ID that uniquely identifies your account.

func init() {
	// TODO verify destination volume exists (and is mountpoint?)
	flag.Parse()
}

func main() {
	defer glog.Flush()
	reader := getDataWithClient(
		parseEnvVar(IMPORTER_ENDPOINT),
		parseEnvVar(IMPORTER_OBJECT_PATH),
		parseEnvVar(IMPORTER_ACCESS_KEY_ID),
		parseEnvVar(IMPORTER_SECRET_KEY))
	defer reader.Close()
	obj := parseEnvVar(IMPORTER_OBJECT_PATH)
	objSlice := strings.Split(obj, "/")
	obj = objSlice[len(objSlice)]
	outFile, err := os.Create(obj)
	defer outFile.Close()
	if err != nil {
		glog.Fatalf("func main: create file error: %v", err)
	}
	if _, err = io.Copy(outFile, reader); err != nil {
		glog.Fatalf("func main: error streaming data: %v", err)
	}
}