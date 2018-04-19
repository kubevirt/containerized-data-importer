package common

// Common types and constants used by the importer and controller.
// TODO: maybe the vm cloner can use these common values

const (
	CDI_VERSION = "0.4.0-alpha.0"
	IMPORTER_DEFAULT_IMAGE = "docker.io/kubevirt/cdi-importer:" + CDI_VERSION

	// host file constants:
	IMPORTER_WRITE_DIR  = "/data"
	IMPORTER_WRITE_FILE = "disk.img"
	IMPORTER_WRITE_PATH = IMPORTER_WRITE_DIR + "/" + IMPORTER_WRITE_FILE
	// importer container constants:
	IMPORTER_PODNAME  = "importer"
	IMPORTER_DATA_DIR = "/data"
	IMPORTER_S3_HOST  = "s3.amazonaws.com"
	// env var names
	IMPORTER_ENDPOINT      = "IMPORTER_ENDPOINT"
	IMPORTER_ACCESS_KEY_ID = "IMPORTER_ACCESS_KEY_ID"
	IMPORTER_SECRET_KEY    = "IMPORTER_SECRET_KEY"
	// key names expected in credential secret
	KeyAccess = "accessKeyId"
	KeySecret = "secretKey"
)
