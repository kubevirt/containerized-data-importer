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
    sha256 = "d0b345518236e240d513fe0f59f6d3da274f035480273a7eb00af7d216ae2a06",
    strip_prefix = "rules_docker-0.11.1",
    urls = [
        "https://github.com/bazelbuild/rules_docker/releases/download/v0.11.1/rules_docker-v0.11.1.tar.gz",
        "https://storage.googleapis.com/builddeps/d0b345518236e240d513fe0f59f6d3da274f035480273a7eb00af7d216ae2a06",
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
    name = "skopeo",
    sha256 = "9ae93d78a41face16d842c4da4ffc07bc8b119fbcd23d436b41a44a5643d4dc0",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/31/Everything/x86_64/Packages/s/skopeo-0.1.41-1.fc31.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/9ae93d78a41face16d842c4da4ffc07bc8b119fbcd23d436b41a44a5643d4dc0",
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
