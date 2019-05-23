FROM registry.fedoraproject.org/fedora-minimal:30
MAINTAINER "The KubeVirt Project" <kubevirt-dev@googlegroups.com>

COPY fedora.repo /tmp/fedora_ci.dnf.repo

RUN sed -i 's/proxy = None/proxy =/gI' /tmp/fedora_ci.dnf.repo && \
	mv /etc/yum.repos.d/fedora.repo /etc/yum.repos.d/fedora.repo.orig  && \
	mv /etc/yum.repos.d/fedora-updates.repo /etc/yum.repos.d/fedora-updates.repo.orig && \
	mv /tmp/fedora_ci.dnf.repo /etc/yum.repos.d/fedora.repo && \
	microdnf update -y && microdnf clean all && \
	mv /etc/yum.repos.d/fedora.repo.orig /etc/yum.repos.d/fedora.repo  && \
	mv /etc/yum.repos.d/fedora-updates.repo.orig /etc/yum.repos.d/fedora-updates.repo

COPY ./cdi-uploadproxy /usr/bin/cdi-uploadproxy

ENTRYPOINT [ "/usr/bin/cdi-uploadproxy", "-alsologtostderr"]
