package main

import (
	"flag"
	"log"
	"net/url"
	"os"
	"strconv"

	controller "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	// Decimal base for proper int64 to string conversion
	decimal = 10
	// Default URI scheme to access the virtual image
	defaultScheme = "file://"
)

var (
	imgPath string
	scheme  string
)

func init() {
	flag.StringVar(&imgPath, "image-path", "", "(Mandatory) URL address of the virtual image.")
	flag.StringVar(&scheme, "scheme", defaultScheme, "(Optional) Virtual image's URI scheme.")
	flag.Parse()
	if imgPath == "" {
		log.Printf("One or more mandatory parameters are missing")
		os.Exit(controller.ErrBadArguments)
	}
}

func main() {
	// Initialize 'qemu-img' handler
	log.Println("Initializing size-detection pod")
	qemuOperations := image.NewQEMUOperations()

	parsedURL, err := url.Parse(scheme + imgPath)
	if err != nil {
		log.Printf("Unable to parse the provided URL: '%s", err.Error())
		os.Exit(controller.ErrInvalidPath)
	}

	// Extract the data from the image
	imgInfo, err := qemuOperations.Info(parsedURL)
	if err != nil {
		log.Printf("Unable to extract information from '%s': '%s'", imgPath, err.Error())
		os.Exit(controller.ErrInvalidFile)
	}

	// Write the parsed virtual size to the termination message file
	strSize := strconv.FormatInt(imgInfo.VirtualSize, decimal)
	err = util.WriteTerminationMessage(strSize)
	if err != nil {
		log.Printf("Unable to write to termination file: '%s'", err.Error())
		os.Exit(controller.ErrBadTermFile)
	}

	log.Println("Size-detection binary has completed")
}
