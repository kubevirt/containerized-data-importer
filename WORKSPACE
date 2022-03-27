register_toolchains("//:python_toolchain")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")
load(
    "@bazel_tools//tools/build_defs/repo:http.bzl",
    "http_archive",
    "http_file",
)
load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")

http_archive(
    name = "rules_python",
    sha256 = "778197e26c5fbeb07ac2a2c5ae405b30f6cb7ad1f5510ea6fdac03bded96cc6f",
    urls = [
        "https://github.com/bazelbuild/rules_python/releases/download/0.2.0/rules_python-0.2.0.tar.gz",
        "https://storage.googleapis.com/builddeps/778197e26c5fbeb07ac2a2c5ae405b30f6cb7ad1f5510ea6fdac03bded96cc6f",
    ],
)

load("//third_party:deps.bzl", "deps")

deps()

# register crosscompiler toolchains
load("//bazel/toolchain:toolchain.bzl", "register_all_toolchains")

register_all_toolchains()

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "69de5c704a05ff37862f7e0f5534d4f479418afc21806c887db544a316f3cb6b",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.27.0/rules_go-v0.27.0.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.27.0/rules_go-v0.27.0.tar.gz",
        "https://storage.googleapis.com/builddeps/69de5c704a05ff37862f7e0f5534d4f479418afc21806c887db544a316f3cb6b",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(
    go_version = "1.17.5",
)

http_archive(
    name = "com_google_protobuf",
    sha256 = "cd218dc003eacc167e51e3ce856f6c2e607857225ef86b938d95650fcbb2f8e4",
    strip_prefix = "protobuf-6d4e7fd7966c989e38024a8ea693db83758944f1",
    # version 3.10.0
    urls = [
        "https://github.com/google/protobuf/archive/6d4e7fd7966c989e38024a8ea693db83758944f1.zip",
        "https://storage.googleapis.com/builddeps/cd218dc003eacc167e51e3ce856f6c2e607857225ef86b938d95650fcbb2f8e4",
    ],
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

# gazelle rules
http_archive(
    name = "bazel_gazelle",
    sha256 = "62ca106be173579c0a167deb23358fdfe71ffa1e4cfdddf5582af26520f1c66f",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.23.0/bazel-gazelle-v0.23.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.23.0/bazel-gazelle-v0.23.0.tar.gz",
        "https://storage.googleapis.com/builddeps/62ca106be173579c0a167deb23358fdfe71ffa1e4cfdddf5582af26520f1c66f",
    ],
)

load(
    "@bazel_gazelle//:deps.bzl",
    "gazelle_dependencies",
    "go_repository",
)

gazelle_dependencies()

#load("@com_github_bazelbuild_buildtools//buildifier:deps.bzl", "buildifier_dependencies")

#buildifier_dependencies()

# bazel docker rules
http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "95d39fd84ff4474babaf190450ee034d958202043e366b9fc38f438c9e6c3334",
    strip_prefix = "rules_docker-0.16.0",
    urls = [
        "https://github.com/bazelbuild/rules_docker/releases/download/v0.16.0/rules_docker-v0.16.0.tar.gz",
        "https://storage.googleapis.com/builddeps/95d39fd84ff4474babaf190450ee034d958202043e366b9fc38f438c9e6c3334",
    ],
)

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_image",
    "container_pull",
)
load(
    "@io_bazel_rules_docker//repositories:repositories.bzl",
    container_repositories = "repositories",
)

container_repositories()

# This is NOT needed when going through the language lang_image
# "repositories" function(s).
load("@io_bazel_rules_docker//repositories:deps.bzl", container_deps = "deps")

container_deps()

# override rules_docker issue with this dependency
# rules_docker 0.16 uses 0.1.4, bit since there the checksum changed, which is very weird, going with 0.1.4.1 to
go_repository(
    name = "com_github_google_go_containerregistry",
    importpath = "github.com/google/go-containerregistry",
    sha256 = "bc0136a33f9c1e4578a700f7afcdaa1241cfff997d6bba695c710d24c5ae26bd",
    strip_prefix = "google-go-containerregistry-efb2d62",
    type = "tar.gz",
    urls = ["https://api.github.com/repos/google/go-containerregistry/tarball/efb2d62d93a7705315b841d0544cb5b13565ff2a"],  # v0.1.4.1
)

# RPM rules
http_archive(
    name = "io_bazel_rules_container_rpm",
    sha256 = "151261f1b81649de6e36f027c945722bff31176f1340682679cade2839e4b1e1",
    strip_prefix = "rules_container_rpm-0.0.5",
    urls = [
        "https://github.com/rmohr/rules_container_rpm/archive/v0.0.5.tar.gz",
        "https://storage.googleapis.com/builddeps/151261f1b81649de6e36f027c945722bff31176f1340682679cade2839e4b1e1",
    ],
)

# Pull base image centos:stream9
container_pull(
    name = "centos",
    registry = "quay.io",
    repository = "centos/centos",
    tag = "stream9",
)

container_pull(
    name = "centos-aarch64",
    registry = "quay.io",
    repository = "centos/centos",
    architecture = "arm64",
    tag = "stream9",
)

# Pull base image container registry
container_pull(
    name = "registry",
    digest = "sha256:b1165286043f2745f45ea637873d61939bff6d9a59f76539d6228abf79f87774",
    registry = "index.docker.io",
    repository = "library/registry",
    tag = "2",
)

container_pull(
    name = "registry-aarch64",
    digest = "sha256:c11a277a91045f91866550314a988f937366bc2743859aa0f6ec8ef57b0458ce",
    registry = "index.docker.io",
    repository = "library/registry",
    tag = "2",
)

# RPMS
http_file(
    name = "qemu-img",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-img-6.2.0-11.el9.x86_64.rpm"],
    sha256 = "0d4d841dc2adf971fbd296eb115559aa6460ccef3ead39e2d17773642d9a68a9",
)

http_file(
    name = "qemu-img-aarch64",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/qemu-img-6.2.0-11.el9.aarch64.rpm"],
    sha256 = "6c86acabac93e4b85d1ee7e8edf9bbb8deb941c46cb4a41d518434dbb7bef50c",
)

http_file(
    name = "nginx",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nginx-1.20.1-8.el9.x86_64.rpm"],
    sha256 = "c5d2144014193f5902a6b89a5f06f5bd51a235aad9c071d724ac13b533fbece1",
)

http_file(
    name = "nginx-aarch64",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nginx-1.20.1-8.el9.aarch64.rpm"],
    sha256 = "6d7ff7ee127c4bd4f6b73f2620c34fdc89149a30a12de0ef26b7163986de42b1",
)

http_file(
    name = "libaio",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libaio-0.3.111-13.el9.x86_64.rpm"],
    sha256 = "7d9d4d37e86ba94bb941e2dad40c90a157aaa0602f02f3f90e76086515f439be",
)

http_file(
    name = "libaio-aarch64",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libaio-0.3.111-13.el9.aarch64.rpm"],
    sha256 = "1730d732818fa2471b5cd461175ceda18e909410db8a32185d8db2aa7461130c",
)

# nginx-filesystem is a noarch rpm which support both x86_64 and aarch64
http_file(
    name = "nginx-filesystem",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nginx-filesystem-1.20.1-8.el9.noarch.rpm"],
    sha256 = "f2f2301df20ec63484f6a67799f29a3d34d77a19c418a27d5370c226a1b87cdd",
)

http_file(
    name = "buildah",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/buildah-1.24.0-0.15.el9.x86_64.rpm"],
    sha256 = "ea1b227d35d822d62f6d323bf201d626cea9c48096dc46fab0b1e2048ddfed78",
)

http_file(
    name = "buildah-aarch64",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/buildah-1.24.0-0.15.el9.aarch64.rpm"],
    sha256 = "8c4781f9d883a542ae8a445ff25171105b47db4b52de36a6666a41dbad84eaea",
)

# containers-common is a noarch rpm which support both x86_64 and aarch64
http_file(
    name = "containers-common",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/containers-common-1-13.el9.noarch.rpm"],
    sha256 = "165c45d9885cb4ffc4830367a15e1cf1720cc923a6b2e686d72c11c09b904027",
)

http_file(
    name = "tar",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/tar-1.34-3.el9.x86_64.rpm"],
    sha256 = "defd09adbb24ed8171521e306b50a8047e4b5d7bf08fa6337751b8e3997dcbe8",
)

http_file(
    name = "ostree-libs",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/ostree-libs-2021.6-1.el9.x86_64.rpm"],
    sha256 = "51215b94203d89d3277871a951c5d49811e064d5685d3e336be818973a39dd67",
)

http_file(
    name = "ostree-libs-aarch64",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/ostree-libs-2021.6-1.el9.aarch64.rpm"],
    sha256 = "df85af8e9a88ae03ca3028a4bd95a72348214df6940ded7ef160e103e7547764",
)

http_file(
    name = "device-mapper-libs",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/device-mapper-libs-1.02.181-1.el9.x86_64.rpm"],
    sha256 = "1a8b1af5dbf1e764a02e1fa5e43fe11e8ca97c161efa808c45c35abd5c91e1cf",
)

http_file(
    name = "device-mapper-event",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/device-mapper-event-1.02.181-1.el9.x86_64.rpm"],
    sha256 = "defd09adbb24ed8171521e306b50a8047e4b5d7bf08fa6337751b8e3997dcbe8",
)

http_file(
    name = "device-mapper-event-aarch64",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/device-mapper-event-1.02.181-1.el9.aarch64.rpm"],
    sha256 = "defd09adbb24ed8171521e306b50a8047e4b5d7bf08fa6337751b8e3997dcbe8",
)

http_file(
    name = "device-mapper-event-libs",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/device-mapper-event-libs-1.02.181-1.el9.x86_64.rpm"],
    sha256 = "defd09adbb24ed8171521e306b50a8047e4b5d7bf08fa6337751b8e3997dcbe8",
)

http_file(
    name = "device-mapper-event-libs-aarch64",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/device-mapper-event-libs-1.02.181-1.el9.aarch64.rpm"],
    sha256 = "defd09adbb24ed8171521e306b50a8047e4b5d7bf08fa6337751b8e3997dcbe8",
)

http_file(
    name = "device-mapper-persistent-data",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/device-mapper-persistent-data-0.9.0-11.el9.x86_64.rpm"],
    sha256 = "defd09adbb24ed8171521e306b50a8047e4b5d7bf08fa6337751b8e3997dcbe8",
)

http_file(
    name = "device-mapper-persistent-data-aarch64",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/device-mapper-persistent-data-0.9.0-11.el9.aarch64.rpm"],
    sha256 = "defd09adbb24ed8171521e306b50a8047e4b5d7bf08fa6337751b8e3997dcbe8",
)

http_file(
    name = "nbdkit-server",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-server-1.28.2-2.el9.x86_64.rpm"],
    sha256 = "d0ff18a0c80f078fe2be429d1a6dc57f4799f71202da171ae7a8de395d4a0e7e",
)

http_file(
    name = "nbdkit-server-aarch64",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-server-1.28.2-2.el9.aarch64.rpm"],
    sha256 = "cda386348687e7933ce497f789d142186d02b9aec215cb5da1139a572732336b",
)

http_file(
    name = "nbdkit-basic-filters",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-basic-filters-1.28.2-2.el9.x86_64.rpm"],
    sha256 = "bfc389aac53dcc7c3e2ff7fc55c311b957b53ee12977dbe214e2141350079251",
)

http_file(
    name = "nbdkit-basic-filters-aarch64",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-basic-filters-1.28.2-2.el9.aarch64.rpm"],
    sha256 = "c5d949608dfdfcb358d323c9727d2c527259412c10c6cbbac07dc4a06b183ff0",
)

http_file(
    name = "nbdkit-vddk-plugin",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-vddk-plugin-1.28.2-2.el9.x86_64.rpm"],
    sha256 = "6b4f2bf4ad89c2dad35c860b03ffebed79cd896a9c7ffcfa3d0254adcd728d82",
)

http_file(
    name = "nbdkit-xz-filter",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-xz-filter-1.28.2-2.el9.x86_64.rpm"],
    sha256 = "a941c019a8d688d53daae067f10d3c7dd341ec948df1cc42fb04c3e2ef92981e",
)

http_file(
    name = "nbdkit-xz-filter-aarch64",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-xz-filter-1.28.2-2.el9.aarch64.rpm"],
    sha256 = "1876ca52e433107b65d1ebf4b471a7dcec9ecc3b19b4af28901b0469058dc32d",
)

http_file(
    name = "nbdkit-gzip-filter",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-gzip-filter-1.28.2-2.el9.x86_64.rpm"],
    sha256 = "b7086c980f11da7ad01e8aec53a7d7c01d519d6920216ac95e040ba13f9d69c9",
)

http_file(
    name = "nbdkit-gzip-filter-aarch64",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-gzip-filter-1.28.2-2.el9.aarch64.rpm"],
    sha256 = "5720fa674b52545c2ec9ea761546f240e7fc97602646be9dc119a35b81fd8bbb",
)

http_file(
    name = "nbdkit-curl-plugin",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-curl-plugin-1.28.2-2.el9.x86_64.rpm"],
    sha256 = "d31387b261a1e1d568349caeaea580932904462788439c4c087d96664c32818d",
)

http_file(
    name = "nbdkit-curl-plugin-aarch64",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-curl-plugin-1.28.2-2.el9.aarch64.rpm"],
    sha256 = "cfcd93018797e4e0a89482e3032a14165cd25849282f10467f0ce328bafba120",
)

http_file(
    name = "libxcrypt",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libxcrypt-4.4.18-3.el9.x86_64.rpm"],
    sha256 = "defd09adbb24ed8171521e306b50a8047e4b5d7bf08fa6337751b8e3997dcbe8",
)

http_file(
    name = "vcenter-govc-tar",
    sha256 = "bfad9df590e061e28cfdd2c321583e96abd43e07687980f5897825ec13ff2cb5",
    urls = ["https://github.com/vmware/govmomi/releases/download/v0.26.1/govc_Linux_x86_64.tar.gz"],
    downloaded_file_path = "govc.tar.gz",
)

http_file(
    name = "vcenter-vcsim-tar",
    sha256 = "b844f6f7645c870a503aa1c5bd23d9a3cb4f5c850505073eef521f2f22a5f2b7",
    urls = ["https://github.com/vmware/govmomi/releases/download/v0.26.1/vcsim_Linux_x86_64.tar.gz"],
    downloaded_file_path = "vcsim.tar.gz",
)

http_file(
    name = "libnbd",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libnbd-1.10.1-1.el9.x86_64.rpm"],
    sha256 = "ff76851f63d3d5a54b482160af1424878e80f06c3de01fff6f3b2c3e8c6b1561",
)

http_file(
    name = "libnbd-aarch64",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/libnbd-1.10.1-1.el9.aarch64.rpm"],
    sha256 = "eb2c09d93281804616d868db4c8cd822acba5d6ddf820d6d3651e6eadc5d91f1",
)

http_file(
    name = "libseccomp",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libseccomp-2.5.0-6.el9.x86_64.rpm"],
    sha256 = "fbb0ee2bca579e3c671a262e02cdcad0b7dee55bc8a73dbcef4c5c6bd5ef788e",
)

http_file(
    name = "libseccomp-aarch64",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libseccomp-2.5.0-6.el9.aarch64.rpm"],
    sha256 = "d7dacf315d019b53dcb05a4b07337968c68ad9b6cb056ba89ead2d699cf13fea",
)

http_file(
    name = "systemd-libs",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-libs-249-9.el9.x86_64.rpm"],
    sha256 = "708fbc3c7fd77a21e0b391e2a80d5c344962de9865e79514b2c89210ef06ba39",
)

http_file(
    name = "systemd-libs-aarch64",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-libs-249-9.el9.aarch64.rpm"],
    sha256 = "683ca2ab7f0aa82baa63fdd248a2cdc13e1dd7ca55294f93e10971c7176ac85d",
)

http_file(
    name = "liburing",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/liburing-0.7-7.el9.x86_64.rpm"],
    sha256 = "a9d32f2a52149dfc94e14e0519ebec709b1607955e57f4b4604807164f0e3850",
)

http_file(
    name = "liburing-aarch64",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/liburing-0.7-7.el9.aarch64.rpm"],
    sha256 = "d42dd2af61c68f2eed3cb4f1cf5af11e2a3b09a1816709b063f5d2c6377a637d",
)

#imageio rpms and dependencies
http_file(
    name = "ovirt-imageio-client",
    sha256 = "5d2eefc623963030ecf4a76473584882056016a28f617a865d813c1bf34d703b",
    urls = ["https://download.copr.fedorainfracloud.org/results/nsoffer/ovirt-imageio-preview/centos-stream-9-x86_64/03803470-ovirt-imageio/ovirt-imageio-client-2.4.2-0.202203211308.git8505899.el9.x86_64.rpm"],
)

http_file(
    name = "ovirt-imageio-client-aarch64",
    sha256 = "ff5aaaa9c725fc1930edf1eb1e5031ef8c37045a0dc38e0b277a6132cf448605",
    urls = ["https://download.copr.fedorainfracloud.org/results/nsoffer/ovirt-imageio-preview/centos-stream-9-aarch64/03803470-ovirt-imageio/ovirt-imageio-client-2.4.2-0.202203211308.git8505899.el9.aarch64.rpm"],
)

http_file(
    name = "ovirt-imageio-common",
    sha256 = "cf34767ec803fa67cc9638d8ee7492e7f3d2f396a0bc1c23b902a9459c8cae77",
    urls = ["https://download.copr.fedorainfracloud.org/results/nsoffer/ovirt-imageio-preview/centos-stream-9-x86_64/03803470-ovirt-imageio/ovirt-imageio-common-2.4.2-0.202203211308.git8505899.el9.x86_64.rpm"],
)

http_file(
    name = "ovirt-imageio-common-aarch64",
    sha256 = "a2e7568f5611b3fefcc4f6560984c36e78c6fe4aff9022f9d14a7806fc8f4f89",
    urls = ["https://download.copr.fedorainfracloud.org/results/nsoffer/ovirt-imageio-preview/centos-stream-9-aarch64/03803470-ovirt-imageio/ovirt-imageio-common-2.4.2-0.202203211308.git8505899.el9.aarch64.rpm"],
)

http_file(
    name = "ovirt-imageio-daemon",
    sha256 = "f12f2be35443079eba32869ed323bc53a1ec42f49a2e416209267bb8caed4be0",
    urls = ["https://download.copr.fedorainfracloud.org/results/nsoffer/ovirt-imageio-preview/centos-stream-9-x86_64/03803470-ovirt-imageio/ovirt-imageio-daemon-2.4.2-0.202203211308.git8505899.el9.x86_64.rpm"],
)

http_file(
    name = "ovirt-imageio-daemon-aarch64",
    sha256 = "74b7c6b2d0fc6a27019e7c2535f0e012bc3db92559c782776acafd5569f866eb",
    urls = ["https://download.copr.fedorainfracloud.org/results/nsoffer/ovirt-imageio-preview/centos-stream-9-aarch64/03803470-ovirt-imageio/ovirt-imageio-daemon-2.4.2-0.202203211308.git8505899.el9.aarch64.rpm"],
)

http_file(
    name = "python3-systemd",
    sha256 = "fafd41778cd2a1f26e3df7e9c395f9a66dc823d1c09a9f29fdf6e591977c318f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/python3-systemd-234-18.el9.x86_64.rpm",
    ],
)

http_file(
    name = "python3-systemd-aarch64",
    sha256 = "1f8ab1b8f5fa235bb75245eab6f5685b4afdfc73aa35b1a9f7df25a4b88a7f69",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/python3-systemd-234-18.el9.aarch64.rpm",
    ],
)

http_file(
    name = "openssl",
    sha256 = "cf0de322d7a4fce445eb66e72589380eecafad1276ceb514ff23a731fff5f9e6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-3.0.1-14.el9.x86_64.rpm",
    ],
)

http_file(
    name = "openssl-aarch64",
    sha256 = "c06021e1a6efefb2c8460ec0bd853ab3a8510a18d51e1beade76aa952f5299f4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openssl-3.0.1-14.el9.aarch64.rpm",
    ],
)
