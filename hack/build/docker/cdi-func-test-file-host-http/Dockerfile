FROM registry.fedoraproject.org/fedora-minimal:30
MAINTAINER "The KubeVirt Project" <kubevirt-dev@googlegroups.com>

COPY fedora.repo /tmp/fedora_ci.dnf.repo

ENV container docker

RUN sed -i 's/proxy = None//gI' /tmp/fedora_ci.dnf.repo && \
    mkdir /etc/yum.repos.d/old && \
	mv /etc/yum.repos.d/*.repo /etc/yum.repos.d/old  && \
	mv /tmp/fedora_ci.dnf.repo /etc/yum.repos.d/fedora.repo && \
	microdnf -y update && microdnf -y install nginx && microdnf clean all -y && \
	mv /etc/yum.repos.d/old/* /etc/yum.repos.d/ && \
	rmdir /etc/yum.repos.d/old

ARG IMAGE_DIR=/usr/share/nginx/html/images

RUN mkdir -p $IMAGE_DIR/priv

RUN mkdir -p $IMAGE_DIR

RUN rm -f /etc/nginx/nginx.conf

COPY nginx.conf /etc/nginx/

COPY htpasswd /etc/nginx/

EXPOSE 80

EXPOSE 81

EXPOSE 82

ENTRYPOINT nginx
