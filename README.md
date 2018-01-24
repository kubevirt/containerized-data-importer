# VM Import Controller

Download source:

 `go get github.com/yard-turkey/vm-image-import`

 or

 `mkdir $GOPATH/src/yard-turkey/`
 `git clone git@github.com:yard-turkey/vm-image-import.git $GOPATH/src/yard-turkey/`

 Dep Management

 Using glide to handle vendoring of deps.

 First install glide

 `curl https://glide.sh/get | sh`

 The run it from the repo root

 `glide install -v`

 `install` scans imports and resolves missing and unsued dependencies.
 `-v` removes nested vendor and Godeps/_workspace directories.
 
 
 ## Export ENV variables
 
 ```
export IMPORTER_ACCESS_KEY_ID="xyzzy"       # must be base64 encoded
export IMPORTER_SECRET_KEY="xyzz"           # must be base64 encoded
export IMPORTER_ENDPOINT=s3.amazonaws.com   # if using aws s3
export IMPORTER_OBJECT_PATH=<bucket-name>/<vm-image-name>
```

## AWS S3 setup

$HOME/.aws/credentials
```
[default]
aws_access_key_id = <your-access-key>
aws_secret_access_key = <your-secret>
```
 
base64 key is decoded by image importer.

## Mino Client setup

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
 base64 key is decoded by image importer.
