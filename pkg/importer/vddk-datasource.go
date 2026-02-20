package importer

import (
	v1 "k8s.io/api/core/v1"
)

// VDDKDataSourceConfig holds parameters for creating a VDDK data source.
type VDDKDataSourceConfig struct {
	Endpoint          string
	AccessKey         string
	SecKey            string
	Thumbprint        string
	UUID              string
	BackingFile       string
	CurrentCheckpoint string
	PreviousCheckpoint string
	FinalCheckpoint   string
	VolumeMode        v1.PersistentVolumeMode
	CertDir           string
	InsecureTLS       bool
}
