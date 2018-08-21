gocli_image="kubevirtci/gocli@sha256:df958c060ca8d90701a1b592400b33852029979ad6d5c1d9b79683033704b690"
gocli="docker run --net=host --privileged --rm -v /var/run/docker.sock:/var/run/docker.sock $gocli_image"
gocli_interactive="docker run --net=host --privileged --rm -it -v /var/run/docker.sock:/var/run/docker.sock $gocli_image"
