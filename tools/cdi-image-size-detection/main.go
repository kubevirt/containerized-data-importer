package main

import (
	"flag"
	"log"
	"net/url"
	"strconv"

	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	// Decimal base for proper int64 to string conversion.
	decimal = 10
	// Default URI scheme to access the virtual image
	defaultScheme = "file://"
)

var (
	imgURL   string
	termPath string
	scheme   string
)

func init() {
	flag.StringVar(&imgURL, "url", "", "(Mandatory) URL address of the virtual image.")
	flag.StringVar(&termPath, "path", "", "(Mandatory) Container's status message path.")
	flag.StringVar(&scheme, "scheme", defaultScheme, "(Optional) Virtual image's URI scheme.")
	flag.Parse()
	if imgURL == "" || termPath == "" {
		log.Fatalf("One or more mandatory parameters are missing")
	}
}

func main() {
	// Initialize 'qemu-img' handler
	log.Println("Initializing size-detection pod")
	var qemuOperations = image.NewQEMUOperations()

	parsedURL, err := url.Parse(scheme + imgURL)
	if err != nil {
		log.Fatalf("Unable to parse the provided URL: '%s", err.Error())
	}

	// Extract the data from the image
	imgInfo, err := qemuOperations.Info(parsedURL)
	if err != nil {
		log.Fatalf("Unable to extract information from '%s': '%s'", imgURL, err.Error())
	}

	strSize := strconv.FormatInt(imgInfo.VirtualSize, decimal)
	// Write the parsed virtual size to the termination message file
	err = util.WriteTerminationMessageToFile(termPath, strSize)
	if err != nil {
		log.Fatalf("Unable to write to file '%s': '%s'", termPath, err.Error())
	}

	log.Println("Size-detection binary has completed")
}
