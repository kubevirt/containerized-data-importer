/*
Copyright 2018 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package importer

import (
	"io"
	"net/url"

	"github.com/pkg/errors"
	"k8s.io/klog"
)

//StreamContext - allows cleanup of temporary data created by specific import method
type StreamContext interface {
	getDataFilePath() string  //returns a path to retrieved data file in case it is stored
	isEncryptedChannel() bool //return true if the data is streamed from encrypted channel
	isRemoteStreaming() bool  //return true if the data is streamed form remote endpoint and not from locally saved
	cleanup() error           //frees all resources allocated by specifc importer for temporary usage
}

//Streamer - generic interface that defines streaming method API
type Streamer interface {
	//establishes io.ReadCloser on VM disk image file located in the provided location
	//returns established reader and context that might contain temporary data used by Streamer
	//if StreamContext is not nil, caller should call cleanup() method of StreamContext at the end of the sequence
	stream() (io.ReadCloser, StreamContext, error)
}

//GetDataStream - factory method that initilizes streamer with respect to provided scheme
func GetDataStream(streamScheme string, url *url.URL, secKey string, accessKey string, certDir string, insecureTLS bool, dataDir string) (io.ReadCloser, StreamContext, error) {
	var r io.ReadCloser
	var ctxt StreamContext
	var err error
	switch streamScheme {
	case "s3":
		r, ctxt, err = S3DataStreamer(url, secKey, accessKey, certDir, insecureTLS).stream()
	case "http", "https":
		r, ctxt, err = HTTPDataStreamer(url, secKey, accessKey, certDir, insecureTLS).stream()
	case "docker", "oci":
		r, ctxt, err = RegistryDataStreamer(url, secKey, accessKey, certDir, insecureTLS, dataDir).stream()
	default:
		klog.V(1).Infoln("Error in Streamer Selector - invalid url scheme")
		return nil, nil, errors.Errorf("invalid url scheme: %q", streamScheme)
	}

	if err != nil {
		klog.Errorf(err.Error(), "Error in Stream Selector")
		klog.V(1).Infoln("Error in Error in Stream Selector")
		return nil, nil, err
	}

	return r, ctxt, nil
}
