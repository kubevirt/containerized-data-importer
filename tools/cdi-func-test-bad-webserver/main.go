package main

import (
	"fmt"
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

func redirect(w http.ResponseWriter, r *http.Request) {
	re := regexp.MustCompile(`[^/]*$`)
	requestedFile := re.Find([]byte(r.URL.String()))

	cdiNamespace := os.Getenv("CDI_NAMESPACE")
	redirectURL := fmt.Sprintf("http://cdi-file-host.%s/%s", cdiNamespace, requestedFile)
	http.Redirect(w, r, redirectURL, 301)
}

func incrementAndGetCounter() uint64 {
	a := atomic.AddUint64(&counter, 1)
	fmt.Printf("Counter: %d\n", a)

	return a
}

var counter uint64 = 0

func main() {
	http.HandleFunc("/forbidden-HEAD/", failHEAD)
	http.HandleFunc("/flaky/", flaky)
	err := http.ListenAndServe(":9090", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
