package image

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/ioutil"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	. "github.com/kubevirt/containerized-data-importer/pkg/common"
	"github.com/xi2/xz"
)

func GzReader(r io.ReadCloser) (io.ReadCloser, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, errors.Wrap(err, "could not create gzip reader")
	}
	glog.V(Vadmin).Infof("gzip: extracting %q\n", gz.Name)
	return gz, nil
}

func XzReader(r io.ReadCloser) (io.ReadCloser, error) {
	glog.V(Vdebug).Infoln("XzReader: xz format")
	xz, err := xz.NewReader(r, 0) //note: default dict size may be too small
	if err != nil {
		return nil, errors.Wrap(err, "could not create xz reader")
	}
	return ioutil.NopCloser(xz), nil
}

func TarReader(r io.ReadCloser) (io.ReadCloser, error) {
	tr := tar.NewReader(r)
	hdr, err := tr.Next() // advance cursor to 1st (and only) file in tarball
	if err != nil {
		return r, errors.Wrap(err, "could not read tar header")
	}
	glog.V(Vadmin).Infof("tar: extracting %q\n", hdr.Name)
	return ioutil.NopCloser(tr), nil
}
