package main

import (
	"os"
	"github.com/minio/minio-go"
	"fmt"
	"github.com/golang/glog"
	"bufio"
	"path/filepath"
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
			f, err := os.Create(getImageFile())
			if err != nil {
				glog.Fatalln(err)
			}
			defer f.Close()
			_, err = f.Write(tmpFile)
			if err != nil {
				glog.Fatalln(err)
			}
			f.Sync()
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
func getImageFile() string {
	//todo: not sure this will work with a url
	endpoint := getEndpoint()
	file := filepath.Base(endpoint)
	return file
}