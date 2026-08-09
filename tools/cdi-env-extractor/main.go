package main

import (
	"flag"
	"log"
	"os"
	"strings"
)

func main() {
	envFile := flag.String("env-file", "/shared/env", "file to create for saving the image environment variables")
	flag.Parse()

	envString := strings.Join(os.Environ(), "\n") + "\n"

	f, err := os.OpenFile(*envFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Failed creating %s: %v", *envFile, err)
	}
	if _, err := f.WriteString(envString); err != nil {
		log.Fatalf("Failed writing %s: %v", *envFile, err)
	}
	if err := f.Sync(); err != nil {
		log.Fatalf("Failed syncing %s: %v", *envFile, err)
	}
	if err := f.Close(); err != nil {
		log.Fatalf("Failed closing %s: %v", *envFile, err)
	}
	log.Printf("Extracted environment variables to %s", *envFile)
}
