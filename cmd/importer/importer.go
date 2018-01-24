package main

import (
	"flag"
	"github.com/golang/glog"
	"io"
	"os"
	"strings"
)

// importer.go implements a data fetching service capable of pulling objects from remote object stores
// and writing to a local directory.  It utilizes the minio-go client sdk.
// This process expects several environmental variables:
//    IMPORTER_URL            Full url + path to object. Mutually exclusive with IMPORTER_ENDPOINT
//    IMPORTER_OBJECT_PATH    Full path of object (e.g. bucket/object)
//    IMPORTER_ACCESS_KEY_ID  Secret key is the password to your account
//    IMPORTER_SECRET_KEY     Access key is the user ID that uniquely identifies your account.

const (
	IMPORTER_URL           = "IMPORTER_URL"
	IMPORTER_ENDPOINT      = "IMPORTER_ENDPOINT"
	IMPORTER_OBJECT_PATH   = "IMPORTER_OBJECT_PATH"
	IMPORTER_ACCESS_KEY_ID = "IMPORTER_ACCESS_KEY_ID"
	IMPORTER_SECRET_KEY    = "IMPORTER_SECRET_KEY"
)

type importInfo struct {
	url         string
	endpoint    string
	objectPath  string
	accessKeyId string
	secretKey   string
}

func init() {
	// TODO verify destination volume exists (and is mountpoint?)
	flag.Parse()
}
// asfdasdfsadfasdf
func main() {
	defer glog.Flush()
	imp := &importInfo{
		endpoint:    parseEnvVar(IMPORTER_ENDPOINT, false),
		objectPath:  parseEnvVar(IMPORTER_OBJECT_PATH, false),
		accessKeyId: parseEnvVar(IMPORTER_ACCESS_KEY_ID, false),
		secretKey: parseEnvVar(IMPORTER_SECRET_KEY, false),
	}
	if len(imp.endpoint) > 0 && len(imp.url) > 0 {
		glog.Fatalf("Detected IMPORTER_URL and IMPORTER_ENDPOINT non-nil values. Variables are mutually exclusive.")
	} else if len(imp.endpoint) > 0 {
		// TODO check required vars
	}
	reader := getDataWithClient(imp)
	defer reader.Close()
	obj := parseEnvVar(IMPORTER_OBJECT_PATH, false)
	objSlice := strings.Split(obj, "/")
	obj = objSlice[len(objSlice)-1]
	outFile, err := os.Create(obj)
	defer outFile.Close()
	if err != nil {
		glog.Fatalf("func main: create file error: %v", err)
	}
	if _, err = io.Copy(outFile, reader); err != nil {
		glog.Fatalf("func main: error streaming data: %v", err)
	}
}
