package main

import (
	"os"
	"github.com/minio/minio-go"
	"fmt"
	"github.com/golang/glog"
	"io"
	"net/http"
	"strings"
)

func isTestEnv() bool {
	if os.Getenv("IMPORT_ENV") == "local" {
		return true
	} else {
		return false

	}

}

func GetFile() {

	useSSL := false

	// Initialize minio client object.
	minioClient, err := minio.NewV2(getEndpoint(), getAccessKeyID(), getSecretAccessKey(), useSSL)
	if err != nil {
		glog.Fatalln(err)
	}

	// Make a new bucket called mymusic.
	bucketName := getBucketName(getEndpoint())
	imageFile := getImageFile(getEndpoint())

	fmt.Printf("Getting file: %v\n", imageFile)

	// Create a done channel to control 'ListObjectsV2' go routine.
	doneCh := make(chan struct{})

	// Indicate to our routine to exit cleanly upon return.
	defer close(doneCh)

	isRecursive := true
	objectCh := minioClient.ListObjectsV2(bucketName, "", isRecursive, doneCh)

	pic := make([]byte, 0)

	for object := range objectCh {
		if object.Err != nil {
			fmt.Println(object.Err)
			return
		}
		if object.Key == imageFile {
			miniofileobject, err := minioClient.GetObject(bucketName, object.Key, nil)
			if err != nil {
				fmt.Println(err)
				return
			}
			tmpFile := make([]byte, object.Size)
			_, err = miniofileobject.Read(tmpFile)
			pic = tmpFile
			err := os.Create(getImageFile())
			if err != nil {
				glog.Fatalln(err)
			}
			//todo: do I need this?
			//w := bufio.NewWriter(f)
			//w.Flush()

		}
	}

}

func getEndpoint() string{
	//Grab from ENV?
 	return os.Getenv("ENDPOINT")
}
func getAccessKeyID() string{
	return os.Getenv("KEYID")
}
func getSecretAccessKey() string{
	return os.Getenv("SECRETKEY")
}
func getBucketName() string {
	//todo: write parser
	endpoint := getEndpoint()
	return endpoint
}
func getImageFile(url string) error {
	//todo: not sure this will work with a url
	splicedUrl := strings.Split(url, "/")
	file := splicedUrl[len(splicedUrl) - 1]
	resp, err := http.Get(url)
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	outFile, err := os.Create(file)
	defer outFile.Close()
	if err != nil {
		return err
	}
	if _, err = io.Copy(outFile, resp.Body); err != nil {
		return err
	}
	return nil
}