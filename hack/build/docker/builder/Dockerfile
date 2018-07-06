FROM fedora:27

RUN dnf install -y qemu xz gzip git && dnf clean all

ENV GIMME_GO_VERSION=1.10 GOPATH="/go" GOBIN="/usr/bin"

RUN mkdir -p /gimme && curl -sL https://raw.githubusercontent.com/travis-ci/gimme/master/gimme | HOME=/gimme bash >> /etc/profile.d/gimme.sh

RUN \
    mkdir -p ${GOPATH} && \
    source /etc/profile.d/gimme.sh && \
    eval $(go env) && \
    go get golang.org/x/tools/cmd/goimports && \
    cd $GOPATH/src/golang.org/x/tools/cmd/goimports  && \
    go install ./...

ADD entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
