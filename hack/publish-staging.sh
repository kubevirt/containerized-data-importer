#!/usr/bin/env bash

set -exo pipefail

GITHUB_FQDN=github.com
GO_API_REF_REPO=${GO_API_REF_REPO:-kubevirt/containerized-data-importer-api}
API_REF_DIR=/tmp/containerized-data-importer-api
GITHUB_IO_FQDN="https://kubevirt.github.io/containerized-data-importer-api"

TARGET_BRANCH="$PULL_BASE_REF"
if [ -n "${DOCKER_TAG}" ]; then
    TARGET_TAG="$DOCKER_TAG"
fi

# if we are not on default branch and there is no tag, do nothing
if [ -z "${TARGET_TAG}" ] && [ "${TARGET_BRANCH}" != "main" ]; then
    echo "not on a tag and not on master branch, nothing to do."
    exit 0
fi

rm -rf ${API_REF_DIR}
git clone \
    "https://${GIT_USER_NAME}@${GITHUB_FQDN}/${GO_API_REF_REPO}.git" \
    "${API_REF_DIR}" >/dev/null 2>&1
pushd ${API_REF_DIR}
git checkout -B ${TARGET_BRANCH}-local
git rm -rf .
git clean -fxd
popd
cp -rf staging/src/kubevirt.io/containerized-data-importer-api/. "${API_REF_DIR}/"

# copy files which are the same on both repos
cp -f LICENSE "${API_REF_DIR}/"
cp -f SECURITY.md "${API_REF_DIR}/"

cd "${API_REF_DIR}"

# Generate .gitignore file. We want to keep bazel files in kubevirt/containerized-data-importer, but not in kubevirt/containerized-data-importer-api
cat >.gitignore <<__EOF__
BUILD
BUILD.bazel
__EOF__

git config user.email "${GIT_AUTHOR_NAME:-kubevirt-bot}"
git config user.name "${GIT_AUTHOR_EMAIL:-mhenriks+kubebot@redhat.com}"

git add -A

if [ -n "$(git status --porcelain)" ]; then
    git commit --message "containerized-data-importer-api update by KubeVirt Prow build ${BUILD_ID}"

    # we only push branch changes on master
    if [ "${TARGET_BRANCH}" == "main" ]; then
        git push origin ${TARGET_BRANCH}-local:${TARGET_BRANCH}
        echo "containerized-data-importer-api updated for ${TARGET_BRANCH}."
    fi
else
    echo "containerized-data-importer-api hasn't changed."
fi

if [ -n "${TARGET_TAG}" ]; then
    if [ $(git tag -l "${TARGET_TAG}") ]; then
        # tag already exists
        echo "tag already exists remotely, doing nothing."
        exit 0
    fi
    git tag ${TARGET_TAG}
    git push origin ${TARGET_TAG}
    echo "containerized-data-importer-api updated for tag ${TARGET_TAG}."
fi
