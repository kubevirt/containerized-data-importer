FROM registry.fedoraproject.org/fedora-minimal:30
MAINTAINER "The KubeVirt Project" <kubevirt-dev@googlegroups.com>

RUN mkdir -p /tmp/shared /tmp/source

COPY fedora.repo /tmp/fedora_ci.dnf.repo

RUN sed -i 's/proxy = None//gI' /tmp/fedora_ci.dnf.repo && \
    mkdir /etc/yum.repos.d/old && \
	mv /etc/yum.repos.d/*.repo /etc/yum.repos.d/old  && \
	mv /tmp/fedora_ci.dnf.repo /etc/yum.repos.d/fedora.repo && \
	microdnf -y update && microdnf -y install qemu-img qemu-block-curl && microdnf clean all -y && \
	mv /etc/yum.repos.d/old/* /etc/yum.repos.d/ && \
	rmdir /etc/yum.repos.d/old

COPY cdi-func-test-file-host-init /usr/bin/

RUN chmod u+x /usr/bin/cdi-func-test-file-host-init

COPY tinyCore.iso /tmp/source/tinyCore.iso
COPY archive.tar /tmp/source/archive.tar

ENTRYPOINT ["cdi-func-test-file-host-init", "-alsologtostderr"]
