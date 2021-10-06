package kloglogger

import (
	log "github.com/ovirt/go-ovirt-client-log/v2"
	"k8s.io/klog/v2"
)

// New creates a simple klog logger which logs on the following levels:
//
// - Debug logs are sent to Infof
// - Info logs are sent to Infof
// - Warning logs are sent to Warningf
// - Error logs are sent to Errorf
func New() log.Logger {
	return &logger{}
}

type logger struct {
}

func (l logger) Debugf(format string, args ...interface{}) {
	klog.Infof(format, args...)
}

func (l logger) Infof(format string, args ...interface{}) {
	klog.Infof(format, args...)
}

func (l logger) Warningf(format string, args ...interface{}) {
	klog.Warningf(format, args...)
}

func (l logger) Errorf(format string, args ...interface{}) {
	klog.Errorf(format, args...)
}
