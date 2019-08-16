FROM registry.fedoraproject.org/fedora-minimal:29
MAINTAINER "The KubeVirt Project" <kubevirt-dev@googlegroups.com>

COPY fedora.repo /tmp/fedora_ci.dnf.repo

RUN sed -i 's/proxy = None//gI' /tmp/fedora_ci.dnf.repo && \
    mkdir /etc/yum.repos.d/old && \
	mv /etc/yum.repos.d/*.repo /etc/yum.repos.d/old  && \
	mv /tmp/fedora_ci.dnf.repo /etc/yum.repos.d/fedora.repo && \
	microdnf update -y && microdnf clean all && \
	mv /etc/yum.repos.d/old/* /etc/yum.repos.d/ && \
	rmdir /etc/yum.repos.d/old

# Create non-root user
RUN useradd -u 1001 --create-home -s /bin/bash cdi-apiserver
WORKDIR /home/cdi-apiserver
USER 1001

COPY ./cdi-apiserver /usr/bin/cdi-apiserver

ENTRYPOINT [ "/usr/bin/cdi-apiserver", "-alsologtostderr"]
