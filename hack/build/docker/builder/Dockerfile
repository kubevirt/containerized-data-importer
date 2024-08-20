FROM quay.io/centos/centos:stream9
LABEL maintainer="The KubeVirt Project <kubevirt-dev@googlegroups.com>"

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

RUN pip3 install --upgrade j2cli operator-courier==2.1.11 && \
	curl -sL https://services.gradle.org/distributions/gradle-6.6-bin.zip -o gradle-6.6-bin.zip && \
	mkdir /opt/gradle && \
	unzip -d /opt/gradle gradle-6.6-bin.zip && \
	ln -s /opt/gradle/gradle-6.6/bin/gradle /usr/local/bin/gradle && \
	rm gradle-6.6-bin.zip

ENV GIMME_GO_VERSION=1.22.3 GOPATH="/go" GO111MODULE="on"

RUN mkdir -p /gimme && curl -sL https://raw.githubusercontent.com/travis-ci/gimme/master/gimme | HOME=/gimme bash >> /etc/profile.d/gimme.sh

RUN \
	source /etc/profile.d/gimme.sh && \
	eval $(go env) && \
	go install mvdan.cc/sh/cmd/shfmt@latest && \
	go install github.com/mattn/goveralls@latest && \
	go install github.com/rmohr/go-swagger-utils/swagger-doc@latest && \
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0 && \
	rm -rf "${GOPATH}/pkg"

ENV BAZEL_VERSION=5.4.1

COPY output-bazel-arch.sh /output-bazel-arch.sh

RUN curl -L -o /usr/bin/bazel https://github.com/bazelbuild/bazel/releases/download/${BAZEL_VERSION}/bazel-${BAZEL_VERSION}-linux-$(sh /output-bazel-arch.sh) && chmod u+x /usr/bin/bazel


RUN if test "$(sh /output-bazel-arch.sh)" = x86_64; then \
      dnf install -y --setopt=install_weak_deps=False  --installroot /usr/s390x-linux-gnu/sys-root --forcearch s390x --releasever 9 \
          glibc-devel \
          glibc-static && \
      dnf install -y epel-release && \
      dnf install -y --setopt=install_weak_deps=False \
          gcc-s390x-linux-gnu && \
      dnf clean -y all; \
    fi

# Until we use a version including the fix for this Bazel issue:

# https://github.com/bazelbuild/bazel/issues/11554
RUN ln -s /usr/bin/python3 /usr/bin/python

ADD entrypoint.sh /entrypoint.sh

ADD entrypoint-bazel.sh /entrypoint-bazel.sh

COPY rsyncd.conf /etc/rsyncd.conf

ENTRYPOINT ["/entrypoint.sh"]
