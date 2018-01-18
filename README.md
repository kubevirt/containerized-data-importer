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

 `install` scans imports and resolves missing and unsued dependencies. `-v` removes nested vendor and Godeps/_workspace directories.

