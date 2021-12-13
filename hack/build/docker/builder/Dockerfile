FROM quay.io/centos/centos:stream9
MAINTAINER "The KubeVirt Project" <kubevirt-dev@googlegroups.com>

RUN 	dnf -y install dnf-plugins-core && \
	dnf config-manager --set-enable crb && dnf update -y && \
	dnf install -y \
	cpio \
	diffutils \
	git \
	python3-pip \
	python3-devel \
	gcc \
	gcc-c++ \
	glibc-devel \
	findutils \
	autoconf \
	automake \
	libtool \
	jq \
	rsync-daemon \
	rsync \
	patch \
	libnbd-devel \
	nbdkit-devel \
	unzip \
	java-11-openjdk-devel \
	&& dnf clean all

# Necessary for Bazel to find Python inside the container
#
# https://github.com/bazelbuild/bazel/issues/8665
# https://github.com/bazelbuild/bazel/issues/11554
RUN ln -s /usr/bin/python3 /usr/bin/python

RUN pip3 install --upgrade j2cli operator-courier==2.1.11 && \
    curl -sL https://services.gradle.org/distributions/gradle-6.6-bin.zip -o gradle-6.6-bin.zip && \
    mkdir /opt/gradle && \
    unzip -d /opt/gradle gradle-6.6-bin.zip && \
    ln -s /opt/gradle/gradle-6.6/bin/gradle /usr/local/bin/gradle && \
    rm gradle-6.6-bin.zip

ENV GIMME_GO_VERSION=1.16.6 GOPATH="/go" KUBEBUILDER_VERSION="2.3.2" ARCH="amd64" GO111MODULE="on"

RUN mkdir -p /gimme && curl -sL https://raw.githubusercontent.com/travis-ci/gimme/master/gimme | HOME=/gimme bash >> /etc/profile.d/gimme.sh

RUN \
    source /etc/profile.d/gimme.sh && \
    eval $(go env) && \
    go get github.com/onsi/ginkgo/ginkgo && \
    go get golang.org/x/tools/cmd/goimports && \
    go get mvdan.cc/sh/cmd/shfmt && \
    go get github.com/mattn/goveralls && \
    go get -u golang.org/x/lint/golint && \
    go get github.com/rmohr/go-swagger-utils/swagger-doc && \
    go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 && \
    go get github.com/securego/gosec/v2/cmd/gosec@0ce48a5 && \
    rm -rf "${GOPATH}/pkg" && \
    (curl -L -O "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${KUBEBUILDER_VERSION}/kubebuilder_${KUBEBUILDER_VERSION}_linux_${ARCH}.tar.gz" && \
     tar -zxvf kubebuilder_${KUBEBUILDER_VERSION}_linux_${ARCH}.tar.gz && \
     mv kubebuilder_${KUBEBUILDER_VERSION}_linux_${ARCH} /usr/local/kubebuilder && \
     rm kubebuilder_${KUBEBUILDER_VERSION}_linux_${ARCH}.tar.gz )

ENV BAZEL_VERSION 3.7.2

RUN curl -L -o /usr/bin/bazel https://github.com/bazelbuild/bazel/releases/download/${BAZEL_VERSION}/bazel-${BAZEL_VERSION}-linux-x86_64 && chmod u+x /usr/bin/bazel

ADD entrypoint.sh /entrypoint.sh
ADD entrypoint-bazel.sh /entrypoint-bazel.sh

COPY rsyncd.conf /etc/rsyncd.conf

ENTRYPOINT ["/entrypoint.sh"]
