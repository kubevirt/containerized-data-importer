FROM fedora:28

RUN dnf install -y curl jq && dnf clean all

COPY ./cdi-uploadproxy-init.sh /usr/bin/cdi-uploadproxy-init.sh

COPY ./cdi-uploadproxy /usr/bin/cdi-uploadproxy

ENTRYPOINT [ "/usr/bin/cdi-uploadproxy", "-alsologtostderr"]
