#!/usr/bin/env bash

set -exo pipefail

GITHUB_FQDN=github.com
API_REF_REPO=${API_REF_REPO:-kubevirt/cdi-api-reference}
API_REF_DIR=/tmp/cdi-api-reference
GITHUB_IO_FQDN="https://kubevirt.io/cdi-api-reference"

TARGET_DIR="$PULL_BASE_REF"
if [ -n "${DOCKER_TAG}" ]; then
    TARGET_DIR="$DOCKER_TAG"
fi

rm -rf "${API_REF_DIR}"
git clone \
    "https://${GIT_USER_NAME}@${GITHUB_FQDN}/${API_REF_REPO}.git" \
    "${API_REF_DIR}" >/dev/null 2>&1
mkdir -p ${API_REF_DIR}/${TARGET_DIR}
cp -f _out/apidocs/html/*.html "${API_REF_DIR}/${TARGET_DIR}/"

cd "${API_REF_DIR}"

# Generate README.md file
cat >README.md <<__EOF__
# KubeVirt Containerized Data Importer API Reference

Content of this repository is generated from OpenAPI specification of
[KubeVirt Containerized Data Importer project](https://github.com/kubevirt/containerized-data-importer) .

## KubeVirt Containerized Data Importer API References

* [master](${GITHUB_IO_FQDN}/master/index.html)
__EOF__
find * -type d -regex "^v[0-9.]*" \
    -exec echo "* [{}](${GITHUB_IO_FQDN}/{}/index.html)" \; >>README.md

git config --global user.email "${GIT_AUTHOR_NAME:-kubevirt-bot}"
git config --global user.name "${GIT_AUTHOR_EMAIL:-rmohr+kubebot@redhat.com}"

# NOTE: exclude index.html from match, because it is static except commit hash.
if git status --porcelain | grep -v "index[.]html" | grep --quiet "^ [AM]"; then
    git add -A README.md "${TARGET_DIR}"/*.html
    git commit --message "API Reference update by KubeVirt Prow build ${BUILD_ID}"

    git push origin master >/dev/null 2>&1
    echo "API Reference updated for ${TARGET_DIR}."
else
    echo "API Reference hasn't changed."
fi
