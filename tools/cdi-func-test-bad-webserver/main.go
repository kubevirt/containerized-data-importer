package main

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
)

func failHEAD(w http.ResponseWriter, r *http.Request) {
	if r.Method == "HEAD" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	redirect(w, r)
}

func flaky(w http.ResponseWriter, r *http.Request) {
	if getCounter()%4 == 3 {
		// succeed after 10 attempts
		redirect(w, r)
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

func redirect(w http.ResponseWriter, r *http.Request) {
	re := regexp.MustCompile(`[^/]*$`)
	requestedFile := re.Find([]byte(r.URL.String()))

	redirectURL := fmt.Sprintf("http://cdi-file-host.cdi/%s", requestedFile)
	http.Redirect(w, r, redirectURL, 301)
}

func initCounter() {
	newCounterBuf := make([]byte, binary.MaxVarintLen64)
	_ = binary.PutUvarint(newCounterBuf, 0)

	ioutil.WriteFile("state", newCounterBuf, 0644)
}

func getCounter() uint64 {
	counterBuf, err := ioutil.ReadFile("state")
	if err != nil {
		panic(err)
	}

	counter, n := binary.Uvarint(counterBuf)
	if n <= 0 {
		counter = 0
	}

	newCounterBuf := make([]byte, binary.MaxVarintLen64)
	_ = binary.PutUvarint(newCounterBuf, counter+1)
	ioutil.WriteFile("state", newCounterBuf, 0644)

	return counter
}

func main() {
	initCounter()
	http.HandleFunc("/forbidden-HEAD/", failHEAD)
	http.HandleFunc("/flaky/", flaky)
	err := http.ListenAndServe(":9090", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
