package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

func printFiles(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		fmt.Println(path)
		return nil
	})
}

func getImageFilename(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	if len(entries) != 1 || entries[0].IsDir() {
		return "", errors.Errorf("Invalid container image")
	}
	return entries[0].Name(), nil
}

func main() {
	port := flag.Int("p", 8100, "server port")
	directory := flag.String("image-dir", ".", "directory to serve")
	readyFile := flag.String("ready-file", "/shared/ready", "file to create when ready for connections")
	doneFile := flag.String("done-file", "/shared/done", "file created when the client is done")
	flag.Parse()

	if err := printFiles(*directory); err != nil {
		log.Fatalf("Failed walking the directory %s: %v", *directory, err)
	}
	imageFilename, err := getImageFilename(*directory)
	if err != nil {
		log.Fatalf("Failed get image filename in %s: %v", *directory, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/info", func(w http.ResponseWriter, _ *http.Request) {
		info := common.ServerInfo{
			Env: os.Environ(),
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(info)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.Handle("/", http.FileServer(http.Dir(*directory)))
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	addr := fmt.Sprintf("localhost:%d", *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed listening on %s err: %v", addr, err)
	}

	if err := os.WriteFile(*readyFile, []byte(imageFilename), 0600); err != nil {
		log.Fatalf("Failed creating \"ready\" file: %v", err)
	}
	defer os.Remove(*readyFile)

	go func() {
		log.Printf("Serving %s on HTTP port: %d\n", *directory, *port)
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Serve failed: %v", err)
		}
	}()

	for {
		if _, err := os.Stat(*doneFile); err == nil {
			break
		}
		time.Sleep(time.Second)
	}

	os.Remove(*doneFile)
	if err := server.Shutdown(context.TODO()); err != nil {
		log.Printf("Shutdown failed: %v\n", err)
	}
	log.Println("Importer has completed")
}
