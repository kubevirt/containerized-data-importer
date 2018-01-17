package main

import (
"log"
"net/http"
"os"
b64 "encoding/base64"
)

var tmp_endpoint, _ = b64.StdEncoding.DecodeString(os.Getenv("BUCKET_ENDPOINT"))
var tmp_accessKeyID, _ = b64.StdEncoding.DecodeString(os.Getenv("BUCKET_ID"))
var tmp_secretAccessKey, _ = b64.StdEncoding.DecodeString(os.Getenv("BUCKET_PWORD"))
var endpoint = string(tmp_endpoint[:])
var accessKeyID = string(tmp_accessKeyID[:])
var secretAccessKey = string(tmp_secretAccessKey[:])

func main() {

	router := NewRouter()

	log.Fatal(http.ListenAndServe(":8888", router))
}
