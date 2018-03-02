package importer

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/golang/glog"
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

func StreamDataToFile(dataReader io.ReadCloser, filePath string) error {
	// Attempt to create the file with name filePath.  If it exists, fail.
	outFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	defer outFile.Close()
	if err != nil {
		return fmt.Errorf("StreamDataToFile: create file error: %v", err)
	}
	if _, err = io.Copy(outFile, dataReader); err != nil {
		return fmt.Errorf("StreamDataToFile: error streaming data: %v", err)
	}
	return nil
}
