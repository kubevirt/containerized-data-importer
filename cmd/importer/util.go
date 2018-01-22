package main

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/minio/minio-go"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	IMPORTER_ENDPOINT      = "IMPORTER_ENDPOINT"
	IMPORTER_OBJECT_PATH   = "IMPORTER_OBJECT_PATH"
	IMPORTER_ACCESS_KEY_ID = "IMPORTER_ACCESS_KEY_ID"
	IMPORTER_SECRET_KEY    = "IMPORTER_SECRET_KEY"
)

func parseEnvVar(envVarName string) string {
	var value string
	if value = os.Getenv(envVarName); len(value) == 0 {
		glog.Fatalf("func parseEnvVar: Environment Variable not set: %s", envVarName)
	}
	glog.Infof("Success parsing environment variable %q", envVarName)
	return value
}

func getDataWithClient(ep, path, accKey, secKey string) *minio.Object {
	fmt.Printf("Copying file: %s\n")
	mc, err := minio.NewV4(ep, accKey, secKey, false)
	if err != nil {
		glog.Fatalf("func getDataWithClient: Could not create Minio client: %v", err)
	}
	parsedPath := strings.Split(path, "/")  //TODO use filepath pkg instead
	objectName := strings.Join(parsedPath[1:], "/")
	bucketName := parsedPath[0]
	objectReader, err := mc.GetObject(bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		glog.Fatalf("func getDataWithClient: failed getting object: %v", err)
	}
	return objectReader
}

// TODO not sure if we actually need this, but it's cool!
func getDataWithHTTP(url string) io.Reader {
	splicedUrl := strings.Split(url, "/")
	file := splicedUrl[len(splicedUrl)-1]
	resp, err := http.Get(url)
	defer resp.Body.Close()
	if err != nil {
		glog.Fatalf("func streamDataFromURL: response body error: %v", err)
	}
	outFile, err := os.Create(file)
	defer outFile.Close()
	if err != nil {
		glog.Fatalf("func streamDataFromURL: create file error: %v", err)
	}
	if _, err = io.Copy(outFile, resp.Body); err != nil {
		glog.Fatalf("func streamDataFromURL: error streaming data: %v", err)
	}
	return resp.Body
}