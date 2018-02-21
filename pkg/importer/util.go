package importer

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/minio/minio-go"
)


type importInfo struct {
	Endpoint    string
	Url         *url.URL
	AccessKeyId string
	SecretKey   string
}


// NewImportInfo: construct a new importInfo object from params.
func NewImportInfo(endpoint, accessKey, secretKey string) (*importInfo, error) {
	// check vars
	if len(endpoint) == 0 {
		return nil, fmt.Errorf("NewImportInfo(): IMPORTER_ENDPOINT is empty")
	}
	if len(accessKey) == 0 || len(secretKey) == 0 {
		glog.Warningln("NewImportInfo(): IMPORTER_ACCESS_KEY_ID and/or IMPORTER_SECRET_KEY env variables are empty")
	}
	url, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("NewImportInfo(): error parsing url: %v\n", err)
	}
	return &importInfo{
		Endpoint:    endpoint,
		Url:         url,
		AccessKeyId: accessKey,
		SecretKey:   secretKey,
	}, nil
}

// newDataReader: given an Endpoint or url return a reader and file name.
func NewDataReader(info *importInfo) (io.ReadCloser, string, error) {
	_, filename, err := parseDataPath(info.Url.Path)
	if err != nil {
		return nil, "", fmt.Errorf("newDataReader Endpoint: %v", err)
	}
	var dataReader io.ReadCloser
	switch info.Url.Scheme {
	case "s3":
		glog.Infof("Importing data from S3 Endpoint: %s", info.Endpoint)
		dataReader = getDataWithS3Client(info)
	case "http":
		fallthrough
	case "https":
		glog.Infof("Importing data from URL: %s", info.Endpoint)
		dataReader = getDataWithHTTP(info)
	case "":
		return nil, "", fmt.Errorf("newDataReader: no url scheme found")
	}
	return dataReader, filename, nil
}

func ParseEnvVar(envVarName string, decode bool) string {
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

func StreamDataToFile(dataReader io.ReadCloser, filePath string) error {
	// Attempt to create the file with name filePath.  If it exists, fail.
	outFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL, 0666)
	defer outFile.Close()
	if err != nil {
		return fmt.Errorf("func streamDataToFile: create file error: %v", err)
	}
	if _, err = io.Copy(outFile, dataReader); err != nil {
		return fmt.Errorf("func streamDataToFile: error streaming data: %v", err)
	}
	return nil
}

func getDataWithS3Client(importInfo *importInfo) io.ReadCloser {
	mc, err := minio.NewV4(importInfo.Endpoint, importInfo.AccessKeyId, importInfo.SecretKey, false)
	if err != nil {
		glog.Fatalf("func getDataWithS3Client: Could not create Minio client: %v", err)
	}
	bucket, object, err := parseDataPath(importInfo.Url.Path)
	glog.Infof("Streaming object %s", importInfo.Url.Path)
	objectReader, err := mc.GetObject(bucket, object, minio.GetObjectOptions{})
	if err != nil {
		glog.Fatalf("func getDataWithS3Client: failed getting objectPath: %v", err)
	}
	return objectReader
}

func getDataWithHTTP(importInfo *importInfo) io.ReadCloser {
	resp, err := http.Get(importInfo.Url.RawPath)
	if err != nil {
		glog.Fatalf("func streamDataFromURL: response body error: %v", err)
	}
	return resp.Body
}

func parseDataPath(dataPath string) (string, string, error) {
	pathSlice := strings.Split(dataPath, "/")
	return pathSlice[0], strings.Join(pathSlice[1:], "_"), nil
}
