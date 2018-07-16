## Getting Started For Developers

### Download CDI

To download the source directly, simply

`$ go get -u kubevirt.io/containerized-data-importer`

### Lint, Test, Build

GnuMake is used to drive a set of scripts that handle linting, testing, compiling, and containerizing.  Executing the scripts directly is not supported at present.

    NOTE: Standard builds require a running Docker daemon!

The standard workflow is performed inside a helper container to normalize the build and test environment for all devs.  Building in the host environment is supported by the Makefile, but is not recommended.

    Docker builds may be disabled by setting DOCKER=0; e.g.
    $ make all DOCKER=0

`$ make all` executes the full workflow.  For granular control of the workflow, several Make targets are defined:

- `all`: cleans up previous build, then compiles all CDI packages and builds containers
- `clean`: cleans up previous build artifacts
- `build`: compile all CDI binary artifacts
    - `build-controller`: compile cdi-controller binary
    - `build-importer`: compile cdi-importer binary
- `test`: execute all tests
    - `test-unit`: execute all tests under `./pkg`
    - `test-functional`: execute all tests under `./test`
- `docker`: compile all binaries and build all containerized
    - `docker-controller`: compile cdi-controller and build cdi-controller image
    - `docker-importer`: compile cdi-importer and build cdi-importer image
    - `docker-cloner`: build the cdi-cloner image ( cloner is driven by a shell script, not a binary )
- `vet`: lint all CDI packages
- `format`: Execute `shfmt`, `goimports`, and `go vet` on all CDI packages.  Writes back to the source files.

In addition to the above recipes, directories may be targeted specifically by setting the `WHAT` make variable, e.g.

Running `$ make vet WHAT=pkg/image` will execute `go vet` on the `kubevirt.io/containerized-data-importer/pkg/image`

### Submit PRs

All PRs should originate from forks of kubevirt.io/containerized-data-importer.  Work should not be done directly in the upstream repository.  Open new working branches from master/HEAD of your forked repository and push them to your remote repo.  Then submit PRs of the working branch against the upstream master branch.

### Vendoring Dependencies

This project uses `glide` as it's dependency manager.  At present, all project dependencies are vendored; using `glide` is unnecessary in the normal work flow.

Install glide:

`curl https://glide.sh/get | sh`

Then run it from the repo root

`glide install -v`

`glide install` scans imports and resolves missing and unused dependencies. `-v` removes nested vendor and Godeps/_workspace directories.

### S3-compatible client setup:

#### AWS S3 cli
$HOME/.aws/credentials
```
[default]
aws_access_key_id = <your-access-key>
aws_secret_access_key = <your-secret>
```

#### Minio cli

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
