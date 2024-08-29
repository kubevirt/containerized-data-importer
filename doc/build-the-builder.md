# Build The KubeVirt CDI Builder Container

## Native Build toolchain

1. Install the [prerequisites](https://github.com/kubevirt/kubevirt/blob/main/docs/build-the-builder.md#prerequisites) as in the instructions for building the KubeVirt builder container.
2. Also install `jq` if the docker build utility you use is `podman-buildah`.  
3. Build the containerized data importer builder container if you haven't already


## Environment Variables and defaults
 
 
 | env variable | default | option or example |
 | ------------ | ------- | -------- |
 | BUILD_ARCH | amd64 | s390x arm64 amd64 |
 | DOCKER_PREFIX | quay.io/kubevirt | icr.io/kubevirt, docker.io/ibm, ... |
 | QUAY_REPOSITORY | kubevirt-cdi-bazel-builder |  |
 | UNTAGGED_BUILDER_IMAGE | quay.io/kubevirt/kubevirt-cdi-bazel-builder | ${DOCKER_PREFIX}/${QUAY_REPOSITORY} |
 | BUILDER_TAG | <nothing> | s390xTest01 |


An example of setting these environment variables would be:
```
export BUILD_ARCH=s390x
export DOCKER_PREFIX="icr.io/kubevirt"
export QUAY_REPOSITORY=kubevirt-cdi-bazel-builder
export UNTAGGED_BUILDER_IMAGE=${DOCKER_PREFIX}/${QUAY_REPOSITORY} 
export BUILDER_TAG=s390xTest01
```

## Manual build of the builder container

The point of this step is to build the CDI builder/helper container described in `hack/build/docker/builder/Dockerfile`. 

```
cd hack/build/docker/builder
```

The build can be conducted with either `podman-buildah` or `docker`.  

### With Podman-buildah

With podman-buildah the builder image can be built and pushed with:

```
buildah build --platform linux/${BUILD_ARCH} --manifest ${UNTAGGED_BUILDER_IMAGE}:${BUILDER_TAG} . 
buildah manifest push --all ${UNTAGGED_BUILDER_IMAGE}:${BUILDER_TAG}  docker://${UNTAGGED_BUILDER_IMAGE}:${BUILDER_TAG}
```
and you can check the digest with: 
```
podman inspect $(podman images | grep ${UNTAGGED_BUILDER_IMAGE} | grep ${BUILDER_TAG} | awk '{ print $3 }') |  jq '.[]["Digest"]'
```

### With docker

With docker, the builder image can be built and pushed with:

```
docker build --tag ${UNTAGGED_BUILDER_IMAGE}:${BUILDER_TAG} .
```
and you can check the digest with:
```
docker images --digests | grep ${UNTAGGED_BUILDER_IMAGE} | grep ${BUILDER_TA
G} | awk '{ print $4 }'
```

## Make Target


`make builder-push` both builds the KubeVirt CDI builder image and pushes it to the registry you specified in the environment variables (above); however the script only works when the following condition is false: 
```
git diff-index --quiet HEAD~1 hack/build/docker
```
since the make target to build the builder is only intended to run during a post-submit job where the PR has squashed the candidate into a single commit. 
