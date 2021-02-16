package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type key int

const (
	contentTypeKey   = "Content-Type"
	contentTypeText  = "text/plain"
	proxyHeaderKey   = "X-GoProxy"
	proxyHeaderValue = "import-proxy-test"
	username         = "foo"
	password         = "bar"
	httpPort         = "8080"
	httpPortWithAuth = "8081"
	withBasicAuth    = true
	noBasicAuth      = false
)

var logger *log.Logger

func startServer(port string, basicAuth bool, wg *sync.WaitGroup) {
	server := &http.Server{
		Addr: ":" + port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			appendForwardedForHeader(r)
			start := time.Now()
			if r.Method == http.MethodConnect {
				handleTunneling(w, r, basicAuth)
			} else {
				handleHTTP(w, r, basicAuth)
			}
			logger.Printf("INFO: METHOD:%s URL:%s SRC IP:%s DURATION:%s USER AGENT:%s\n",
				r.Method, r.URL, r.RemoteAddr, fmt.Sprint(time.Since(start)), r.UserAgent())

		}),
		ErrorLog: logger,
		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
	go func() {
		logger.Printf("INFO: started proxy on port %s\n", port)
		log.Fatal(server.ListenAndServe())
		wg.Done()
	}()
}

// Prepares the X-Forwarded-For header for another forwarding hop by appending the previous sender's
// IP address to the X-Forwarded-For chain.
func appendForwardedForHeader(req *http.Request) {
	// Copied from net/http/httputil/reverseproxy.go:
	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		// If we aren't the first proxy retain prior
		// X-Forwarded-For information as a comma+space
		// separated list and fold multiple headers into one.
		if prior, ok := req.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		req.Header.Set("X-Forwarded-For", clientIP)
	}
}

func handleTunneling(w http.ResponseWriter, req *http.Request, withAuth bool) {
	if withAuth {
		if !isAuthorized(w, req) {
			return
		}
	}
	destConn, err := net.DialTimeout("tcp", req.Host, 300*time.Second)
	if err != nil {
		logger.Printf("ERROR: URL:%s %v\n", req.Host, err.Error())
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		logger.Printf("ERROR: Hijacking not supported\n")
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		logger.Printf("ERROR: %v\n", err.Error())
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	go transfer(destConn, clientConn)
	go transfer(clientConn, destConn)
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}

func handleHTTP(w http.ResponseWriter, req *http.Request, withAuth bool) {
	if withAuth {
		if !isAuthorized(w, req) {
			return
		}
	}
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	sender(w, resp)
}

func isAuthorized(w http.ResponseWriter, req *http.Request) bool {
	//Basic auth for https connection
	sEnc := strings.Replace(strings.Replace(req.Header.Get("Proxy-Authorization"), "Proxy-Authorization=Basic ", "", -1), "Basic ", "", -1)
	sDec, _ := base64.StdEncoding.DecodeString(sEnc)
	if auth := strings.Split(string(sDec), ":"); len(auth) > 1 {
		user := auth[0]
		pass := auth[1]
		if username == user && password == pass {
			logger.Printf("INFO: Proxy authorization succeeded\n")
			return true
		}
	}

	//Basic auth for http connection
	user, pass, _ := req.BasicAuth()
	if username == user && password == pass {
		logger.Printf("INFO: Proxy authorization succeeded\n")
		return true
	}

	//Not authorized
	logger.Printf("ERROR: Proxy authorization failed\n")
	resp := newResponse(req,
		contentTypeText, http.StatusUnauthorized,
		"Basic auth wrong")
	sender(w, resp)
	return false
}

func newResponse(r *http.Request, contentType string, status int, body string) *http.Response {
	resp := &http.Response{}
	resp.Request = r
	resp.TransferEncoding = r.TransferEncoding
	resp.Header = make(http.Header)
	resp.Header.Add(contentTypeKey, contentType)
	resp.Header.Add(proxyHeaderKey, proxyHeaderValue)
	resp.StatusCode = status
	resp.Status = http.StatusText(status)
	buf := bytes.NewBufferString(body)
	resp.ContentLength = int64(buf.Len())
	resp.Body = ioutil.NopCloser(buf)
	return resp
}

func sender(w http.ResponseWriter, resp *http.Response) {
	resp.Header.Set(proxyHeaderKey, proxyHeaderValue)
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
func main() {
	logger = log.New(os.Stdout, "", log.LstdFlags)

	wg := new(sync.WaitGroup)

	wg.Add(1)
	startServer(httpPort, noBasicAuth, wg)

	wg.Add(1)
	startServer(httpPortWithAuth, withBasicAuth, wg)

	wg.Wait()
}
