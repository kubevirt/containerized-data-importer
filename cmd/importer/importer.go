package main

import (
	"flag"
	"github.com/golang/glog"
	"fmt"
	"io"
	"os"
	//"strings"
	"strings"
	"path/filepath"
)

// importer.go implements a data fetching service capable of pulling objects from remote object stores
// and writing to a local directory.  It utilizes the minio-go client sdk.
// This process expects several environmental variables:
//    IMPORTER_URL            Full url + path to object. Mutually exclusive with IMPORTER_ENDPOINT
//    IMPORTER_ENDPOINT       Endpoint url minus scheme, bucket/object and port, eg. s3.amazon.com
//			      Mutually exclusive with IMPORTER_URL
//    IMPORTER_OBJECT_PATH    Full path of object (e.g. bucket/object)
//    IMPORTER_ACCESS_KEY_ID  Secret key is the password to your account
//    IMPORTER_SECRET_KEY     Access key is the user ID that uniquely identifies your account.

const (
	IMPORTER_URL           = "IMPORTER_URL"
	IMPORTER_ENDPOINT      = "IMPORTER_ENDPOINT"
	IMPORTER_OBJECT_PATH   = "IMPORTER_OBJECT_PATH"
	IMPORTER_ACCESS_KEY_ID = "IMPORTER_ACCESS_KEY_ID"
	IMPORTER_SECRET_KEY    = "IMPORTER_SECRET_KEY"
	IMPORTER_DESTINATION   = "IMPORTER_DESTINATION"
)

type importInfo struct {
	url         string
	endpoint    string
	objectPath  string
	accessKeyId string
	secretKey   string
	dest        string
}

func init() {
	flag.Parse()
}

func main() {
	defer glog.Flush()
	glog.Info("Starting importer")
	imp, err := getEnvVars()
	if err != nil {
		glog.Fatalf("unable to get env variables: %v", err)
	}
	// create object reader
	reader := getDataWithClient(imp)
	defer reader.Close()
	// Parse bucket and object name (handles directory abstraction in object names)
	objSlice := strings.Split(imp.objectPath, "/")
	// Rejoin object name. Convert / to _
	obj := strings.Join(objSlice[1:], "_")
	glog.Infof("Writing %s to %s", obj, imp.dest)
	outFile, err := os.Create(filepath.Join(imp.dest, obj))
	defer outFile.Close()
	if err != nil {
		glog.Fatalf("func main: create file error: %v", err)
	}
	if _, err = io.Copy(outFile, reader); err != nil {
		glog.Fatalf("func main: error streaming data: %v", err)
	}
	glog.Infof("Streaming complete, exiting")
}

// getEnvVars: get predefined exported env variables, perform syntax and semantic checking,
// return struc containing these vars.
// TODO: maybe the access key and secret need to be decoded from base64?
func getEnvVars() (*importInfo, error) {
	url := parseEnvVar(IMPORTER_URL, false)
	ep := parseEnvVar(IMPORTER_ENDPOINT, false)
	op := parseEnvVar(IMPORTER_OBJECT_PATH, false)
	acc := parseEnvVar(IMPORTER_ACCESS_KEY_ID, false)
	sec := parseEnvVar(IMPORTER_SECRET_KEY, false)
	dest := parseEnvVar(IMPORTER_DESTINATION, false)
	// check vars
	// TODO log the endpoint to be used
	if len(ep) > 0 && len(url) > 0 {
		return nil, fmt.Errorf("IMPORTER_ENDPOINT and IMPORTER_URL cannot both be defined")
	}
	if len(ep) == 0 && len(url) == 0 {
		return nil, fmt.Errorf("IMPORTER_ENDPOINT or IMPORTER_URL must be defined")
	}
	if len(op) == 0 || len(acc) == 0 || len(sec) == 0 {
		return nil, fmt.Errorf("IMPORTER_OBJECT_PATH and/or IMPORTER_ACCESS_KEY_ID and/or IMPORTER_SECRET_KEY are empty")
	}
	if len(dest) == 0 {
		glog.Infof("%s not set, default: /", IMPORTER_DESTINATION)
		dest = "./"
	}
	return &importInfo{
		url:	     url,
		endpoint:    ep,
		objectPath:  op,
		accessKeyId: acc,
		secretKey:   sec,
		dest:        dest,
	}, nil
}
