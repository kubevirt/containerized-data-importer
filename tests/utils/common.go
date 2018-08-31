package utils

const (
	// cdi-file-host pod/service relative values
	FileHostName       = "cdi-file-host" // deployment and service name
	FileHostNs         = "kube-system"   // deployment and service namespace
	FileHostS3Bucket   = "images"        // s3 bucket name (e.g. http://<serviceIP:port>/FileHostS3Bucket/image)
	AccessKeyValue     = "admin"         // http && s3 username, see hack/build/docker/cdi-func-test-file-host-http/htpasswd
	SecretKeyValue     = "password"      // http && s3 password,  ditto
	HttpAuthPortName   = "http-auth"     // cdi-file-host service auth port
	HttpNoAuthPortName = "http-no-auth"  // cdi-file-host service no-auth port, requires AccessKeyValue and SecretKeyValue
)
