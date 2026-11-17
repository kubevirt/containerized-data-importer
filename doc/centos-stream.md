# Building CDI with CentOS Stream

CDI can be built with CentOS Stream 9 or 10. CentOS Stream 9 is the default.

Set `CDI_CENTOS_STREAM_VERSION` to select the builder image, RPM dependencies
and the corresponding toolchain configuration. Supported values are `9` and `10`.

## Build an image

Run these commands from the repository root.

Build the importer for x86_64 with CentOS Stream 9:

```bash
CDI_CENTOS_STREAM_VERSION=9 ./hack/build/bazel-docker.sh \
  'bazel build --config=x86_64 //cmd/cdi-importer:cdi-importer-image'
```

Build the same image with CentOS Stream 10:

```bash
CDI_CENTOS_STREAM_VERSION=10 ./hack/build/bazel-docker.sh \
  'bazel build --config=x86_64 --config=cs10 //cmd/cdi-importer:cdi-importer-image'
```

When invoking Bazel directly through the wrapper, include `--config=cs10`
for CS10 builds. The environment variable selects the builder; the Bazel
configuration selects the CS10 RPM dependencies and toolchain.

The build scripts that source `hack/build/config.sh` pass this configuration
through `BAZEL_CS_CONFIG`.

## Cross builds

From an x86_64 host, build the importer for aarch64 or s390x with:

```bash
CDI_CENTOS_STREAM_VERSION=10 ./hack/build/bazel-docker.sh \
  'bazel build --config=aarch64 --config=crossbuild-aarch64 --config=cs10 //cmd/cdi-importer:cdi-importer-image'

CDI_CENTOS_STREAM_VERSION=10 ./hack/build/bazel-docker.sh \
  'bazel build --config=s390x --config=crossbuild-s390x --config=cs10 //cmd/cdi-importer:cdi-importer-image'
```

For CS9 cross builds, set `CDI_CENTOS_STREAM_VERSION=9` and omit
`--config=cs10`.

## Builder overrides and persistent state

An explicit `BUILDER_IMAGE` overrides the default image for the selected stream.
The wrappers check that its userspace version matches the selected stream.

CS9 and CS10 use separate persistent builder containers and volumes.
If an existing Bazel server uses a different image from the requested builder,
the wrapper reports an error. Remove that server container before retrying.

See [Build the builder](build-the-builder.md) for instructions on creating
builder images.

## RPM dependencies

CS9 repository definitions are in `repo.yaml`; CS10 definitions are in
`repo-cs10.yaml`. Generate dependencies with the selected stream:

```bash
CDI_CENTOS_STREAM_VERSION=10 make rpm-deps
```

Regeneration can change package versions as repository metadata changes.
Review changes to both `WORKSPACE` and `rpm/BUILD.bazel`.
