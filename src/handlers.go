package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/minio/minio-go"
	"log"
	"strings"
)

func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "API Server Version 1.0!\n")
}
func ShowPic(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	w.Write([]byte("<html><body><img src='http://localhost:8888/getBucket/demo-bucket1/pic1.jpg'></body></html>"))
}


func GetAllFile(w http.ResponseWriter, r *http.Request) {

	useSSL := false
	myFiles := make([]string,0)

	// Initialize minio client object.
	minioClient, err := minio.NewV2(endpoint, accessKeyID, secretAccessKey, useSSL)
	if err != nil {
		log.Fatalln(err)
	}
	
	vars := mux.Vars(r)
	bucketName :=  vars["bucketName"]
	//hostname := vars["url"]


	fmt.Printf("Getting all file names for bucket: %v\n", bucketName)
	fmt.Printf("Cluster IP:Port %v\n", endpoint)
	fmt.Printf("Cluster Creditials: %v ***** %v\n", accessKeyID, secretAccessKey)


	// Create a done channel to control 'ListObjectsV2' go routine.
	doneCh := make(chan struct{})

	// Indicate to our routine to exit cleanly upon return.
	defer close(doneCh)

	isRecursive := true
	objectCh := minioClient.ListObjectsV2(bucketName, "", isRecursive, doneCh)

	for object := range objectCh {
		fmt.Printf("object =%v\n", object)
		if object.Err != nil {
			fmt.Println(object.Err)
			return
		}
		myFiles = append(myFiles,object.Key)
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(myFiles); err != nil {
		panic(err)
	}
}

func GetFile(w http.ResponseWriter, r *http.Request) {

	useSSL := false

	// Initialize minio client object.
	minioClient, err := minio.NewV2(endpoint, accessKeyID, secretAccessKey, useSSL)
	if err != nil {
		log.Fatalln(err)
	}

	// Make a new bucket called mymusic.
	vars := mux.Vars(r)
	bucketName :=  vars["bucketName"]
	reqFile :=  vars["fileName"]

	fmt.Printf("Getting file: %v\n", reqFile)


	// Create a done channel to control 'ListObjectsV2' go routine.
	doneCh := make(chan struct{})

	// Indicate to our routine to exit cleanly upon return.
	defer close(doneCh)

	isRecursive := true
	objectCh := minioClient.ListObjectsV2(bucketName, "", isRecursive, doneCh)

	pic := make([]byte,0)

	for object := range objectCh {
		if object.Err != nil {
			fmt.Println(object.Err)
			return
		}
		if (object.Key == reqFile) {
			miniofileobject, err := minioClient.GetObject(bucketName, object.Key)
			if err != nil {
				fmt.Println(err)
				return
			}
			tmpFile := make([]byte, object.Size)
			_, err = miniofileobject.Read(tmpFile)
			pic = tmpFile
		}
	}


	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	w.Write(pic)
}

func PutFile(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(10000000)
	myvalue := r.PostFormValue("data")
	fmt.Printf("File being upload with size= %v\n", len(myvalue))

	file := strings.NewReader(myvalue)

	type response struct {
		status string
		id     string
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response{"ok","1"}); err != nil {
		panic(err)
	}

	useSSL := false

	// Initialize minio client object.
	minioClient, err := minio.NewV2(endpoint, accessKeyID, secretAccessKey, useSSL)
	if err != nil {
		log.Fatalln(err)
	}

	// Make a new bucket called mymusic.
	vars := mux.Vars(r)
	bucketName :=  vars["bucketName"]
	newFile :=  vars["fileName"]


	_, err = minioClient.PutObject(bucketName, newFile, file, "application/octet-stream")
	if err != nil {
		fmt.Println(err)
		return
	}
}