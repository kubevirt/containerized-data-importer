package image

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/golang/glog"
	"github.com/xi2/xz"
)

func GzReader(r io.ReadCloser) (io.ReadCloser, error) {
	glog.Infoln("GzReader: gz format")
	return gzip.NewReader(r)
}

func XzReader(r io.ReadCloser) (io.ReadCloser, error) {
	glog.Infoln("XzReader: xz format")
	rdr, err := xz.NewReader(r, 0) //note: default dict size may be too small
	if err != nil {
		return nil, fmt.Errorf("XzReader: error creating xz Reader: %v\n", err)
	}
	return ioutil.NopCloser(rdr), nil
}

func TarReader(r io.ReadCloser) (io.ReadCloser, error) {
	glog.Infoln("TarReader: tar format")
	tr := tar.NewReader(r)
	hdr, err := tr.Next() // advance cursor to 1st (and only) file in tarball
	if err != nil {
		return r, fmt.Errorf("TarReader: reading tarfile header: %v\n", err)
	}
	glog.Infof("TarReader: extracting %q\n", hdr.Name)
	return ioutil.NopCloser(tr), nil
}
