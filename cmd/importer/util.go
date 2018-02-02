package main

import (
	"encoding/base64"
	"fmt"
	"github.com/golang/glog"
	"github.com/minio/minio-go"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func parseEnvVar(envVarName string, decode bool) string {
	value := os.Getenv(envVarName)
	if decode {
		v, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			glog.Fatalf("Error decoding environment variable %q", envVarName)
		}
		value = fmt.Sprintf("%s", v)
	}
	return value
}

func getDataWithClient(importInfo *importInfo) io.ReadCloser {
	mc, err := minio.NewV4(importInfo.endpoint, importInfo.accessKeyId, importInfo.secretKey, false)
	if err != nil {
		glog.Fatalf("func getDataWithClient: Could not create Minio client: %v", err)
	}
	bucket, object, err := parseDataPath(importInfo.objectPath, false)
	glog.Infof("Streaming object %s", importInfo.objectPath)
	objectReader, err := mc.GetObject(bucket, object, minio.GetObjectOptions{})
	if err != nil {
		glog.Fatalf("func getDataWithClient: failed getting objectPath: %v", err)
	}
	return objectReader
}

func getDataWithHTTP(importInfo *importInfo) io.ReadCloser {
	resp, err := http.Get(importInfo.url)
	if err != nil {
		glog.Fatalf("func streamDataFromURL: response body error: %v", err)
	}
	return resp.Body
}

func parseDataPath(dataPath string, fromUrl bool) (string, string, error) {
	var bucket, filename string
	if fromUrl {
		url, err := url.Parse(dataPath)
		if err != nil {
			return "", "", fmt.Errorf("func parseDataPath: %s", err)
		}
		pathSlice := strings.Split(url.Path, "/")
		bucket = ""
		filename = pathSlice[len(pathSlice)-1]
		return bucket, filename, nil
	}
	pathSlice := strings.Split(dataPath, "/")
	bucket = pathSlice[0]
	// Rejoin object name. Convert / to _
	filename = strings.Join(pathSlice[1:], "_")
	return bucket, filename, nil
}

func streamDataToFile(dataReader io.ReadCloser, filename string) error {
	outFile, err := os.Create(filepath.Join(WRITE_PATH, filename))
	defer outFile.Close()
	if err != nil {
		return fmt.Errorf("func streamDataToFile: create file error: %v", err)
	}
	if _, err = io.Copy(outFile, dataReader); err != nil {
		return fmt.Errorf("func streamDataToFile: error streaming data: %v", err)
	}
	return nil
}
