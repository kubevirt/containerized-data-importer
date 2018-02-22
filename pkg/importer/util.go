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

// NewDataReader returns a reader of either S3 or HTTP streams.
//   http: if no security credentials are provided, http is assumed.
//	 s3:   if s3:// scheme or security credentials are provided, s3 is assumed.
//   If the url scheme is s3://, the importInfo.Url is altered to represent an AWS bucket.
func NewDataReader(importInfo *importInfo) (io.ReadCloser, error) {
	var dataReader io.ReadCloser
	if len(importInfo.AccessKeyId) > 0 {
		if importInfo.Url.Scheme == "s3" {
			// When s3:// is detected, the s3 client is directed to s3.amazonaws.com
			// Bucket is the url.host; object is the url.Path.
			// So we alter the Url object to represent an aws bucket.
			importInfo.Url.Path = strings.Join([]string{importInfo.Url.Host, importInfo.Url.Path}, "")
			importInfo.Url.Host = "s3.amazonaws.com"
		}
		dataReader = getDataWithS3Client(importInfo)
	} else if strings.Contains(importInfo.Url.Scheme, "http") && len(importInfo.AccessKeyId) == 0 {
		dataReader = getDataWithHTTP(importInfo)
	} else {
		return nil, fmt.Errorf("Unable to determinte client for streaming %v\n", importInfo.Url)
	}
	return dataReader, nil
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
	outFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	defer outFile.Close()
	if err != nil {
		return fmt.Errorf("func StreamDataToFile: create file error: %v", err)
	}
	if _, err = io.Copy(outFile, dataReader); err != nil {
		return fmt.Errorf("func StreamDataToFile: error streaming data: %v", err)
	}
	return nil
}

func getDataWithS3Client(importInfo *importInfo) io.ReadCloser {
	glog.Infoln("Using S3 client to get data")
	mc, err := minio.NewV4(importInfo.Url.Host, importInfo.AccessKeyId, importInfo.SecretKey, false)
	if err != nil {
		glog.Fatalf("getDataWithS3Client(): error building minio client for %q\n", importInfo.Url.Host)
	}
	bucket, object, err := parseDataPath(importInfo.Url.Path)
	glog.Infof("Attempting to get object %q via S3 client\n", importInfo.Url.Path)
	objectReader, err := mc.GetObject(bucket, object, minio.GetObjectOptions{})
	if err != nil {
		glog.Fatalf("func getDataWithS3Client: failed getting objectPath: %v", err)
	}
	return objectReader
}

func getDataWithHTTP(importInfo *importInfo) io.ReadCloser {
	glog.Infoln("Using HTTP GET to fetch data.")
	resp, err := http.Get(importInfo.Endpoint)
	if err != nil {
		glog.Fatalf("func getDataWithHTTP: response body error: %v", err)
	}
	return resp.Body
}

func parseDataPath(dataPath string) (string, string, error) {
	pathSlice := strings.Split(strings.Trim(dataPath, "/"), "/")
	return pathSlice[0], strings.Join(pathSlice[1:], "/"), nil
}
