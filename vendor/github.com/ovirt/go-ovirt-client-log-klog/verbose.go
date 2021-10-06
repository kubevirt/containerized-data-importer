package kloglogger

import (
	log "github.com/ovirt/go-ovirt-client-log/v2"
	"k8s.io/klog/v2"
)

// NewVerbose creates a logger based on the klog.V function. It logs all messages
func NewVerbose(
	debugLevel klog.Verbose,
	infoLevel klog.Verbose,
	warningLevel klog.Verbose,
	errorLevel klog.Verbose,
) log.Logger {
	return &verboseLogger{
		debugLevel:   debugLevel,
		infoLevel:    infoLevel,
		warningLevel: warningLevel,
		errorLevel:   errorLevel,
	}
}

type verboseLogger struct {
	debugLevel   klog.Verbose
	infoLevel    klog.Verbose
	warningLevel klog.Verbose
	errorLevel   klog.Verbose
}

func (v verboseLogger) Debugf(format string, args ...interface{}) {
	v.debugLevel.Infof(format, args...)
}

func (v verboseLogger) Infof(format string, args ...interface{}) {
	v.infoLevel.Infof(format, args...)
}

func (v verboseLogger) Warningf(format string, args ...interface{}) {
	v.warningLevel.Infof(format, args...)
}

func (v verboseLogger) Errorf(format string, args ...interface{}) {
	v.errorLevel.Infof(format, args...)
}
