package importer

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/golang/glog"
	"github.com/kubevirt/containerized-data-importer/pkg/common"
)

func ParseEnvVar(envVarName string, decode bool) string {
	value := os.Getenv(envVarName)
	if decode {
		v, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			glog.Fatalf("ParseEnvVar: error decoding environment variable %q", envVarName)
		}
		value = fmt.Sprintf("%s", v)
	}
	return value
}

// Parse the required endpoint and return the url struct.
func ParseEndpoint(endpt string) (*url.URL, error) {
	if endpt == "" {
		endpt = ParseEnvVar(common.IMPORTER_ENDPOINT, false)
		if endpt == "" {
			return nil, fmt.Errorf("ParseEndpoint: endpoint %q is missing or blank\n", common.IMPORTER_ENDPOINT)
		}
	}
	return url.Parse(endpt)
}

func StreamDataToFile(dataReader io.Reader, filePath string) error {
	// Attempt to create the file with name filePath.  If it exists, fail.
	outFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.ModePerm)
	defer outFile.Close()
	if err != nil {
		return fmt.Errorf("StreamDataToFile: create file error: %v", err)
	}
	glog.Infof("StreamDataToFile: begin import...\n")
	if _, err = io.Copy(outFile, dataReader); err != nil {
		os.Remove(outFile.Name())
		return fmt.Errorf("StreamDataToFile: error streaming data: %v", err)
	}
	return nil
}
