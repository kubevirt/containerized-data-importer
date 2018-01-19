package main

import (
	"github.com/golang/glog"
	"flag"
)

func init() {
	flag.Parse()
}

func main() {

	defer glog.Flush()


	//---------- Importer --------------------------
	//1)Setup the environment
	//2) create io buffer and copy the img into it
	//3) Save that buffer into mnt which should be there from the controller creating the pvc

	// ??) Do I need to run this in K8s -- no, not for testing
		// Set a flag that says env=local or env= k8s/ocp

	//Setup Environment
	//S3 endpoint - endpoint plus bucket + image size
	//Credential
	//mntpoint



	glog.Infoln("Setup Environment\n")
	if  isTestEnv() {
		mntPoint := "/mnt/place"
	} else {

	}



	glog.Infoln("Getting Image files\n")
	//Minio getFile(image_name)
	//put it into buffer
	glog.Infoln("Writing image to goldem pvc\n")
	//write buffer to mnt point




	//ListPods()


}
