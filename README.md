# VM Import Controller
This repo implements a vm image importer (copier) to a known location inside a Kubernetes or Openshift cluster. This immutable vm image copy is considered a _golden image_ source for later vm cloning and instantiation.

# Getting Started

### Download source:

`# in github fork yard-turkey/vm-image-import to your personal repo`, then:
```
cd $GOPATH/src/
mkdir -p github.com/yard-turkey/
go get github.com/yard-turkey/vm-image-import
cd github.com/vm-image-import
git remote set-url origin <url-to-your-personal-repo>
git push origin master -f
```

 or

 ```
 cd $GOPATH/src/github.com/
 mkdir yard-turkey && cd yard-turkey
 git clone <your-repo-url-for-vm-image-import>
 cd vm-image-import
 git remote add upstream 	https://github.com/yard-turkey/vm-image-import.git
 ```

### Use glide to handle vendoring of dependencies.

Install glide:

 `curl https://glide.sh/get | sh`

Then run it from the repo root

 `glide install -v`

 `glide install` scans imports and resolves missing and unsued dependencies.
 `-v` removes nested vendor and Godeps/_workspace directories.

### Compile importer binary from source

```
cd cd $GOPATH/src/github.com/yard-turkey/
make importer
```
which places the binary in _./bin/vm-importer_.
**Note:** the binary has not been containerized yet so it cannot be executed from a pod.

### Export ENV variables

Before running the importer binary several environment variables must be exported:
 
 ```
export IMPORTER_ACCESS_KEY_ID="xyzzy"       # may later be base64 encoded
export IMPORTER_SECRET_KEY="xyzz"           # may later be base64 encoded
export IMPORTER_ENDPOINT=s3.amazonaws.com   # if using aws s3
export IMPORTER_OBJECT_PATH=<bucket-name>/<vm-image-name>
```

### Run the importer

```
./bin/importer
```
which copyies the image named by the `IMPORTER_OBJECT_PATH` environment variable to your current working directory.


### S3-compatible client setup

#### AWS S3 cli:
$HOME/.aws/credentials
```
[default]
aws_access_key_id = <your-access-key>
aws_secret_access_key = <your-secret>
```

#### Mino cli:

$HOME/.mc/config.json:
```
{
        "version": "8",
        "hosts": {
                "s3": {
                        "url": "https://s3.amazonaws.com",
                        "accessKey": "<your-access-key>",
                        "secretKey": "<your-secret>",
                        "api": "S3v4"
                }
        }
}
```
