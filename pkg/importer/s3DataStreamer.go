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
	"strings"

	minio "github.com/minio/minio-go"
	"github.com/pkg/errors"
	"k8s.io/klog"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

//s3DataStreamer
type s3DataStreamer struct {
	secKey    string
	accessKey string
	url       *url.URL
}

//S3DataStreamer - -creates tsreamer that fetches file from S3 source
func S3DataStreamer(url *url.URL, secKey, accessKey, certDir string, insecureTLS bool) Streamer {
	return s3DataStreamer{
		url:       url,
		secKey:    secKey,
		accessKey: accessKey,
	}
}

func (i s3DataStreamer) getDataFilePath() string {
	return ""
}

func (i s3DataStreamer) cleanup() error {
	return nil
}

func (i s3DataStreamer) isEncryptedChannel() bool {
	return (i.accessKey != "" && i.secKey != "")
}

func (i s3DataStreamer) isRemoteStreaming() bool {
	return true
}

func (i s3DataStreamer) stream() (io.ReadCloser, StreamContext, error) {
	klog.V(3).Infoln("Using S3 client to get data")
	bucket := i.url.Host
	object := strings.Trim(i.url.Path, "/")
	mc, err := minio.NewV4(common.ImporterS3Host, i.accessKey, i.secKey, false)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "could not build minio client for %q", i.url.Host)
	}
	klog.V(2).Infof("Attempting to get object %q via S3 client\n", i.url.String())
	objectReader, err := mc.GetObject(bucket, object, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, errors.Wrapf(err, "could not get s3 object: \"%s/%s\"", bucket, object)
	}
	return objectReader, &i, nil
}
