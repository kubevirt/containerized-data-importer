package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync/atomic"
)

func failHEAD(w http.ResponseWriter, r *http.Request) {
	if r.Method == "HEAD" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	redirect(w, r)
}

func flaky(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" && incrementAndGetCounter()%4 == 3 {
		fmt.Printf("Method: %s, Redirecting\n", r.Method)
		redirect(w, r)

		return
	}

	fmt.Printf("Method: %s, Status: %v\n", r.Method, http.StatusServiceUnavailable)
	w.WriteHeader(http.StatusServiceUnavailable)
}

func badContentType(w http.ResponseWriter, r *http.Request) {
	actualFileURL := getEquivalentFileHostURL(r.URL.String())

	//nolint:gosec // This is not production code.
	resp, err := http.Get(actualFileURL)
	if err != nil {
		panic("Couldn't fetch URL")
	}

	w.Header().Set("Content-Type", "text/html")

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		panic(fmt.Errorf("badContentType: failed to write the response; %w", err))
	}
}

func noAcceptRanges(w http.ResponseWriter, r *http.Request) {
	actualFileURL := getEquivalentFileHostURL(r.URL.String())

	//nolint:gosec // This is not production code
	resp, err := http.Get(actualFileURL)
	if err != nil {
		panic("Couldn't fetch URL")
	}

	defer resp.Body.Close()

	contentLength, ok := resp.Header["Content-Length"]
	if !ok {
		panic("No content length from cdi-file-host")
	}

	w.Header().Set("Content-Length", contentLength[0])

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		panic(fmt.Errorf("noAcceptRanges: failed to write the response; %w", err))
	}
}

func redirect(w http.ResponseWriter, r *http.Request) {
	redirectURL := getEquivalentFileHostURL(r.URL.String())
	http.Redirect(w, r, redirectURL, 301)
}

func getEquivalentFileHostURL(url string) string {
	re := regexp.MustCompile(`[^/]*$`)
	requestedFile := re.Find([]byte(url))

	cdiNamespace := os.Getenv("CDI_NAMESPACE")
	return fmt.Sprintf("http://cdi-file-host.%s/%s", cdiNamespace, requestedFile)
}

func incrementAndGetCounter() uint64 {
	a := atomic.AddUint64(&counter, 1)
	fmt.Printf("Counter: %d\n", a)

	return a
}

var counter uint64

func main() {
	http.HandleFunc("/forbidden-HEAD/", failHEAD)
	http.HandleFunc("/flaky/", flaky)
	http.HandleFunc("/no-accept-ranges/", noAcceptRanges)
	http.HandleFunc("/bad-content-type/", badContentType)
	err := http.ListenAndServe(":9090", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
