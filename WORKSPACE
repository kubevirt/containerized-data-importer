register_toolchains("//:python_toolchain")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")

http_archive(
    name = "io_bazel_rules_go",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/rules_go/releases/download/v0.19.5/rules_go-v0.19.5.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.19.5/rules_go-v0.19.5.tar.gz",
        "https://storage.googleapis.com/builddeps/513c12397db1bc9aa46dd62f02dd94b49a9b5d17444d49b5a04c5a89f3053c1c",
    ],
    sha256 = "513c12397db1bc9aa46dd62f02dd94b49a9b5d17444d49b5a04c5a89f3053c1c",
)

load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains")

go_rules_dependencies()

go_register_toolchains()

load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")

git_repository(
    name = "com_google_protobuf",
    commit = "09745575a923640154bcf307fba8aedff47f240a",
    remote = "https://github.com/protocolbuffers/protobuf",
    shallow_since = "1558721209 -0700",
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

# gazelle rules
http_archive(
    name = "bazel_gazelle",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/bazel-gazelle/releases/download/0.18.2/bazel-gazelle-0.18.2.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/0.18.2/bazel-gazelle-0.18.2.tar.gz",
        "https://storage.googleapis.com/builddeps/7fc87f4170011201b1690326e8c16c5d802836e3a0d617d8f75c3af2b23180c4",
    ],
    sha256 = "7fc87f4170011201b1690326e8c16c5d802836e3a0d617d8f75c3af2b23180c4",
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()

# bazel docker rules
http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "4521794f0fba2e20f3bf15846ab5e01d5332e587e9ce81629c7f96c793bb7036",
    strip_prefix = "rules_docker-0.14.4",
    urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.14.4/rules_docker-v0.14.4.tar.gz"],
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

load("@io_bazel_rules_docker//repositories:pip_repositories.bzl", "pip_deps")
pip_deps()

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

# Pull base image fedora31
container_pull(
    name = "fedora",
    digest = "sha256:6c4b03683891b7f8963c0dcc9a654d8a95a441ec20bf9a24a1560ac02004da35",
    registry = "quay.io",
    repository = "awels/fedora-minimal-image",
    tag = "31",
)

container_pull(
    name = "fedora-docker",
    digest = "sha256:d3d106e8f3affb1011b97c2b6ef388430dc1474bf7c7ad05963cff49961edb89",
    registry = "index.docker.io",
    repository = "fedora",
    tag = "31",
)

# Pull base image container registry
container_pull(
    name = "registry",
    digest = "sha256:b1165286043f2745f45ea637873d61939bff6d9a59f76539d6228abf79f87774",
    registry = "index.docker.io",
    repository = "library/registry",
    tag = "2",
)

# RPMS
http_file(
    name = "qemu-img",
    sha256 = "2f6f519bce659ac7e9a5ceab1899fe464dffaa196b9d7603e68aac21657d0096",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/31/Everything/x86_64/Packages/q/qemu-img-4.1.1-1.fc31.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/2f6f519bce659ac7e9a5ceab1899fe464dffaa196b9d7603e68aac21657d0096",
    ],
)

http_file(
    name = "qemu-block-curl",
    sha256 = "084f4df7971c7a624996cf05f43ff60ff1a19fdc7800e338519826edaab3811d",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/31/Everything/x86_64/Packages/q/qemu-block-curl-4.1.1-1.fc31.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/084f4df7971c7a624996cf05f43ff60ff1a19fdc7800e338519826edaab3811d",
    ],
)

http_file(
    name = "nginx",
    sha256 = "3ae196450e27518aca0d89a4e33f1aa45babace90395009aea11401c5e8d50cc",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/n/nginx-1.16.1-1.fc31.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/3ae196450e27518aca0d89a4e33f1aa45babace90395009aea11401c5e8d50cc",
    ],
)

http_file(
    name = "xen-libs",
    sha256 = "06147608a5c32e3678f67dd7ad87abb4cd50b0a234a2bbc3ef643f67eec05e53",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/31/Everything/x86_64/Packages/x/xen-libs-4.12.2-2.fc31.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/06147608a5c32e3678f67dd7ad87abb4cd50b0a234a2bbc3ef643f67eec05e53",
    ],
)

http_file(
    name = "libaio",
    sha256 = "ee6596a5010c2b4a038861828ecca240aa03c592dacd83c3a70d44cb8ee50408",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/l/libaio-0.3.111-6.fc31.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/ee6596a5010c2b4a038861828ecca240aa03c592dacd83c3a70d44cb8ee50408",
    ],
)

http_file(
    name = "capstone",
    sha256 = "4d2671bc78b11650e8ccf75926e34295c641433759eab8f8932b8403bfa15319",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/c/capstone-4.0.1-4.fc31.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/4d2671bc78b11650e8ccf75926e34295c641433759eab8f8932b8403bfa15319",
    ],
)

http_file(
    name = "gperftools-lib",
    sha256 = "e58e5da835e2c8b762fd6ec9968416245a80986d6f6bf3b3f4664c4e63f65eb9",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/g/gperftools-libs-2.7-6.fc31.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e58e5da835e2c8b762fd6ec9968416245a80986d6f6bf3b3f4664c4e63f65eb9",
    ],
)

http_file(
    name = "libunwind",
    sha256 = "b1a86867aa0faa7f1cf9cfef5134f6f27f22ebfa18fe5840f064aaa0c13fc389",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/l/libunwind-1.3.1-3.fc31.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/b1a86867aa0faa7f1cf9cfef5134f6f27f22ebfa18fe5840f064aaa0c13fc389",
    ],
)

http_file(
    name = "nginx-mimetypes",
    sha256 = "f4ef1413fa087ae8630930a1eab67d3cbcf501c39648ffc1a534267f21d38d9e",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/n/nginx-mimetypes-2.1.48-6.fc31.noarch.rpm",
        "https://storage.googleapis.com/builddeps/f4ef1413fa087ae8630930a1eab67d3cbcf501c39648ffc1a534267f21d38d9e",
    ],
)

http_file(
    name = "nginx-filesystem",
    sha256 = "97b13750fe1dfbd00b6cb8fecaf8e7bc7aac4b233a5e430d65fa0e200ef337ea",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/n/nginx-filesystem-1.16.1-1.fc31.noarch.rpm",
        "https://storage.googleapis.com/builddeps/97b13750fe1dfbd00b6cb8fecaf8e7bc7aac4b233a5e430d65fa0e200ef337ea",
    ],
)

http_file(
    name = "buildah",
    sha256 = "b375721a3ee2ea54ff550bffe6d6e5c810dce803dd7b91df4f0a1884d2b0a926",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/31/Everything/x86_64/Packages/b/buildah-1.14.0-2.fc31.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/b375721a3ee2ea54ff550bffe6d6e5c810dce803dd7b91df4f0a1884d2b0a926",
    ],
)

http_file(
    name = "containers-common",
    sha256 = "410eac36fab2b18ea7a959041175e14a6f3530bc995c655d41e6a4c2604a2a9e",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/31/Everything/x86_64/Packages/c/containers-common-0.1.41-1.fc31.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/410eac36fab2b18ea7a959041175e14a6f3530bc995c655d41e6a4c2604a2a9e",
    ],
)

http_file(
    name = "tar",
    sha256 = "9975496f29601a1c2cdb89e63aac698fdd8283ba3a52a9d91ead9473a0e064c8",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/t/tar-1.32-2.fc31.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/9975496f29601a1c2cdb89e63aac698fdd8283ba3a52a9d91ead9473a0e064c8",
    ],
)

http_file(
    name = "ostree-libs",
    sha256 = "4011ad8b367db9d528d47202d07c287a958d4bd11a56b11618818dcb3be55bc6",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/o/ostree-libs-2019.4-3.fc31.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/4011ad8b367db9d528d47202d07c287a958d4bd11a56b11618818dcb3be55bc6",
    ],
)

http_file(
    name = "lvm2",
    sha256 = "790256fe3d3b39700a4345649fcaab1da8dc1d13104577480d1807a108c0273f",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/l/lvm2-2.03.05-2.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "lvm2-libs",
    sha256 = "c8125a5f282f6022a2ca0c287c6804301262432646aba5a5b8ec06eeacb83102",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/l/lvm2-libs-2.03.05-2.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "device-mapper",
    sha256 = "d8fa0b0947084bce50438b7eaf5a5085abd35e36c69cfb13d5f58e98a258e36f",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/d/device-mapper-1.02.163-2.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "device-mapper-libs",
    sha256 = "0ebd37bcd6d2beb5692b7c7e3d94b90a26d45b059696d954b502d85d738b7732",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/d/device-mapper-libs-1.02.163-2.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "device-mapper-event",
    sha256 = "9dfb6c534d23d3058d83dfaf669544b58318c34351ae46e7341cdeee51be2ab8",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/d/device-mapper-event-1.02.163-2.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "device-mapper-event-libs",
    sha256 = "3d9ed59c05d68649e255ab6961ce7b8b758ab82cbe74d6912d0f4395c7ebd4f3",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/d/device-mapper-event-libs-1.02.163-2.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "device-mapper-persistent-data",
    sha256 = "4a3eef2bea1e3a1fe305b9c2acbf46d6bc6063d415fe0c33c25416ed42791cee",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/d/device-mapper-persistent-data-0.8.5-2.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "compat-readline5",
    sha256 = "03179d5423784f6a61d18dcbb35fe986fb318ebac65330c0228bbef4e835c992",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/c/compat-readline5-5.2-34.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "kmod",
    sha256 = "ec22cf64138373b6f28dab0b824fbf9cdec8060bf7b8ce8216a361ab70f0849b",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/k/kmod-26-4.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "basesystem",
    sha256 = "20ef955e5f735233a425725b9af41d960b5602dfb0ae812ae720e37c9bf8a292",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/b/basesystem-11-8.fc31.noarch.rpm",
    ],
)

http_file(
    name = "bash",
    sha256 = "09f5522e833a03fd66e7ea9368331b7f316f494db26decda59cbacb6ea4185b3",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/b/bash-5.0.7-3.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "fedora-gpg-keys",
    sha256 = "f2ae011207332ac90d0cf50b1f0b9eb0ce8be1d4d4c7186463dff38a90af0f3d",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/f/fedora-gpg-keys-31-1.noarch.rpm",
    ],
)

http_file(
    name = "fedora-release",
    sha256 = "40eee4e4234c781277a202aa0e834c2be8afc28a3e4012b07d6c24058b0f4add",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/f/fedora-release-31-1.noarch.rpm",
    ],
)

http_file(
    name = "fedora-release-common",
    sha256 = "e566c03caeeaa58db28c0b257f5d36ea92adfe2a18884208f03611c35397a6a1",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/f/fedora-release-common-31-1.noarch.rpm",
    ],
)

http_file(
    name = "fedora-repos",
    sha256 = "9c9250ccd816e5d8c2bfdee14d16e9e71d2038707009e36e7642c136d7c62e4c",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/f/fedora-repos-31-1.noarch.rpm",
    ],
)

http_file(
    name = "filesystem",
    sha256 = "ce05d442cca1de33cb9b4dfb72b94d8b97a072e2add394e075131d395ef463ff",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/f/filesystem-3.12-2.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "glibc",
    sha256 = "33e0ad9b92d40c4e09d6407df1c8549b3d4d3d64fdd482439e66d12af6004f13",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/g/glibc-2.30-5.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "glibc-all-langpacks",
    sha256 = "f67d5cc67029c6c38185f94b72aaa9034a49f5c4f166066c8268b41e1b18a202",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/g/glibc-all-langpacks-2.30-5.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "glibc-common",
    sha256 = "1098c7738ca3b78a999074fbb93a268acac499ee8994c29757b1b858f59381bb",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/g/glibc-common-2.30-5.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "libgcc",
    sha256 = "4106397648e9ef9ed7de9527f0da24c7e5698baa5bc1961b44707b55730ad5e1",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/l/libgcc-9.2.1-1.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "libselinux",
    sha256 = "b75fe6088e737720ea81a9377655874e6ac6919600a5652576f9ebb0d9232e5e",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/l/libselinux-2.9-5.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "libsepol",
    sha256 = "2ebd4efba62115da56ed54b7f0a5c2817f9acd29242a0334f62e8c645b81534f",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/l/libsepol-2.9-2.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "ncurses-base",
    sha256 = "cbd9d78da00aea6c1e98398fe883d5566971b3bc6764a07c5e945cd317013686",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/n/ncurses-base-6.1-12.20190803.fc31.noarch.rpm",
    ],
)

http_file(
    name = "ncurses-lib",
    sha256 = "7b3ba4cdf8c0f1c4c807435d7b7a4a93ecb02737a95d064f3f20299e5bb3a106",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/n/ncurses-libs-6.1-12.20190803.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "pcre2",
    sha256 = "017d8f5d4abb5f925c1b6d46467020c4fd5e8a8dcb4cc6650cab5627269e99d7",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/p/pcre2-10.33-14.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "setup",
    sha256 = "4c859170bc4705a8ff4592f7376918fd2a97435c13cde79f24475c0a0866251d",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/s/setup-2.13.3-2.fc31.noarch.rpm",
    ],
)

http_file(
    name = "tzdata",
    sha256 = "f0847b05feed5f47260e38b9ea40935644c061ccde2b82da5c68874190d59034",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/t/tzdata-2019c-1.fc31.noarch.rpm",
    ],
)

http_file(
    name = "iscsi-initiator-utils",
    sha256 = "a3fab3da01bfcbeb3cfe223810f55ce6652976d51d07990f59cab2854498d90e",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/i/iscsi-initiator-utils-6.2.0.876-10.gitf3c8e90.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "iscsi-initiator-utils-iscsiuio",
    sha256 = "c557c2145799e5a5e45f8fdab25fd823c63babc36bb131e57c7c0e222ef6a911",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/i/iscsi-initiator-utils-iscsiuio-6.2.0.876-10.gitf3c8e90.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "isns-utils-libs",
    sha256 = "6805cf46806ab4b5975bccdb06bd33612bde50c0e09fbaafdc91d4498a45ea1b",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/i/isns-utils-libs-0.97-9.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "nbdkit-server",
    sha256 = "20289b472fc2f075db4d9e993505ff088852417527a06e29e457778cb7f55183",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/n/nbdkit-server-1.14.2-1.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "nbdkit-vddk-plugin",
    sha256 = "b9b2eb86b0f8d8355be6a1ef42107526aa7f7d35e5b31af7441750e6b770e9b4",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/n/nbdkit-vddk-plugin-1.14.2-1.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "libxcrypt-compat",
    sha256 = "de561ae2ce6394c9b77d5002a52bd9ee9ffea642ffea39bd8fb84d21dce0825c",
    urls = [
        "https://download.fedoraproject.org/pub/fedora/linux/updates/31/Everything/x86_64/Packages/l/libxcrypt-compat-4.4.17-1.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "libxcrypt",
    sha256 = "779d1ba6fc8d794c067679f5cb3762b78afe9e44c203a80424a27f94ed4969b6",
    urls = [
        "https://download.fedoraproject.org/pub/fedora/linux/updates/31/Everything/x86_64/Packages/l/libxcrypt-4.4.17-1.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "mkpasswd",
    sha256 = "cfe92e9aff4080b8eec8fd0668bcd3e12450a05bf503ba13c739f8e0d7893709",
    urls = [
        "https://download.fedoraproject.org/pub/fedora/linux/updates/31/Everything/x86_64/Packages/m/mkpasswd-5.5.6-1.fc31.x86_64.rpm",
    ],
)

http_file(
    name = "whois-nls",
    sha256 = "b345fb463c541c6ea69d5308b41b044fa8ef3739206fb3993c019e5538b449e9",
    urls = [
        "https://download.fedoraproject.org/pub/fedora/linux/updates/31/Everything/x86_64/Packages/w/whois-nls-5.5.6-1.fc31.noarch.rpm",
    ],
)

http_file(
    name = "golang-github-vmware-govmomi",
    sha256 = "9ace85ca6a9a6dfd6a9e621fe9012fadd704ba5c9fbf1d042244eb0f250b3115",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/31/Everything/x86_64/os/Packages/g/golang-github-vmware-govmomi-0.21.0-2.fc31.x86_64.rpm",
    ],
)
