ARG VERSION
FROM fedora:$VERSION

RUN dnf update -y && dnf install -y \
  yum-utils coreutils python3 rsync rsync-daemon

RUN sed -i 's/^metalink/#metalink/g' /etc/yum.repos.d/fedora*.repo \
    && sed -i 's/^#baseurl/baseurl/g' /etc/yum.repos.d/fedora*.repo \
    && sed -i 's/download\.example/download\.fedoraproject\.org/g' /etc/yum.repos.d/fedora*.repo

RUN dnf config-manager --set-disabled fedora-cisco-openh264 \
    && dnf update -y \
    && dnf clean all

COPY rsyncd.conf /etc/rsyncd.conf
COPY ./fetch-repo-workspace.py /fetch/fetch-repo-workspace.py
WORKDIR /fetch
