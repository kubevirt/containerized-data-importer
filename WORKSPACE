register_toolchains("//:python_toolchain")

load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")

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

# Bazel buildtools prebuilt binaries
http_archive(
    name = "buildifier_prebuilt",
    sha256 = "7f85b688a4b558e2d9099340cfb510ba7179f829454fba842370bccffb67d6cc",
    strip_prefix = "buildifier-prebuilt-7.3.1",
    urls = [
        "http://github.com/keith/buildifier-prebuilt/archive/7.3.1.tar.gz",
        "https://storage.googleapis.com/builddeps/7f85b688a4b558e2d9099340cfb510ba7179f829454fba842370bccffb67d6cc",
    ],
)

load("@buildifier_prebuilt//:deps.bzl", "buildifier_prebuilt_deps")

buildifier_prebuilt_deps()

load("@buildifier_prebuilt//:defs.bzl", "buildifier_prebuilt_register_toolchains", "buildtools_assets")

buildifier_prebuilt_register_toolchains(
    assets = buildtools_assets(
        arches = [
            "amd64",
            "arm64",
            "s390x",
        ],
        names = ["buildozer"],
        platforms = [
            "darwin",
            "linux",
            "windows",
        ],
        sha256_values = {
            "buildozer_darwin_amd64": "854c9583efc166602276802658cef3f224d60898cfaa60630b33d328db3b0de2",
            "buildozer_darwin_arm64": "31b1bfe20d7d5444be217af78f94c5c43799cdf847c6ce69794b7bf3319c5364",
            "buildozer_linux_amd64": "3305e287b3fcc68b9a35fd8515ee617452cd4e018f9e6886b6c7cdbcba8710d4",
            "buildozer_linux_arm64": "0b5a2a717ac4fc911e1fec8d92af71dbb4fe95b10e5213da0cc3d56cea64a328",
            "buildozer_linux_s390x": "7e28da8722656e800424989f5cdbc095cb29b2d398d33e6b3d04e0f50bc0bb10",
            "buildozer_windows_amd64": "58d41ce53257c5594c9bc86d769f580909269f68de114297f46284fbb9023dcf",
        },
        version = "v7.3.1",
    ),
)

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "86d3dc8f59d253524f933aaf2f3c05896cb0b605fc35b460c0b4b039996124c6",
    urls = [
        "https://mirror.bazel.build/github.com/bazel-contrib/rules_go/releases/download/v0.60.0/rules_go-v0.60.0.zip",
        "https://github.com/bazel-contrib/rules_go/releases/download/v0.60.0/rules_go-v0.60.0.zip",
        "https://storage.googleapis.com/builddeps/86d3dc8f59d253524f933aaf2f3c05896cb0b605fc35b460c0b4b039996124c6",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(
    go_version = "host",
)

# gazelle rules
http_archive(
    name = "bazel_gazelle",
    sha256 = "92329a7dbb26d0beacc43da669211546ea6627582793f4dd5f28837fde3a5c08",
    urls = [
        "https://github.com/bazel-contrib/bazel-gazelle/releases/download/v0.50.0/bazel-gazelle-v0.50.0.tar.gz",
        "https://storage.googleapis.com/builddeps/92329a7dbb26d0beacc43da669211546ea6627582793f4dd5f28837fde3a5c08",
    ],
)

load(
    "@bazel_gazelle//:deps.bzl",
    "gazelle_dependencies",
    "go_repository",
)

gazelle_dependencies()

http_archive(
    name = "bazeldnf",
    sha256 = "03852830a239efaa7ee4ba7d50344b2bfd5b41c604e379011bcadf0477ecc8db",
    strip_prefix = "bazeldnf-v0.99.1",
    urls = [
        "https://github.com/rmohr/bazeldnf/releases/download/v0.99.1/bazeldnf-v0.99.1.tar.gz",
    ],
)

load("@bazeldnf//bazeldnf:defs.bzl", "rpm")
load("@bazeldnf//bazeldnf:deps.bzl", "bazeldnf_dependencies")

bazeldnf_dependencies()

# bazel-lib rules
http_archive(
    name = "aspect_bazel_lib",
    sha256 = "f525668442e4b19ae10d77e0b5ad15de5807025f321954dfb7065c0fe2429ec1",
    strip_prefix = "bazel-lib-2.21.1",
    urls = [
        "https://github.com/bazel-contrib/bazel-lib/releases/download/v2.21.1/bazel-lib-v2.21.1.tar.gz",
        "https://storage.googleapis.com/builddeps/f525668442e4b19ae10d77e0b5ad15de5807025f321954dfb7065c0fe2429ec1",
    ],
)

http_archive(
    name = "tar.bzl",
    sha256 = "a0d64064a598d7a1e58196d17de0deed6d3d2d8bfe1407ed9e68b24c31c38e8d",
    strip_prefix = "tar.bzl-0.7.0",
    urls = [
        "https://github.com/bazel-contrib/tar.bzl/releases/download/v0.7.0/tar.bzl-v0.7.0.tar.gz",
        "https://storage.googleapis.com/builddeps/a0d64064a598d7a1e58196d17de0deed6d3d2d8bfe1407ed9e68b24c31c38e8d",
    ],
)

load("@aspect_bazel_lib//lib:repositories.bzl", "aspect_bazel_lib_dependencies", "aspect_bazel_lib_register_toolchains")

aspect_bazel_lib_dependencies()

aspect_bazel_lib_register_toolchains()

load("@bazel_tools//tools/build_defs/repo:utils.bzl", "maybe")
load("@platforms//host:extension.bzl", "host_platform_repo")

maybe(
    host_platform_repo,
    name = "host_platform",
)

# rules_pkg
http_archive(
    name = "rules_pkg",
    sha256 = "d20c951960ed77cb7b341c2a59488534e494d5ad1d30c4818c736d57772a9fef",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/1.0.1/rules_pkg-1.0.1.tar.gz",
        "https://github.com/bazelbuild/rules_pkg/releases/download/1.0.1/rules_pkg-1.0.1.tar.gz",
        "https://storage.googleapis.com/builddeps/d20c951960ed77cb7b341c2a59488534e494d5ad1d30c4818c736d57772a9fef",
    ],
)

load("@rules_pkg//:deps.bzl", "rules_pkg_dependencies")

rules_pkg_dependencies()

http_archive(
    name = "package_metadata",
    sha256 = "5bd0cc7594ea528fd28f98d82457f157827d48cc20e07bcfdbb56072f35c8f67",
    strip_prefix = "supply-chain-0.0.6/metadata",
    urls = [
        "https://github.com/bazel-contrib/supply-chain/releases/download/v0.0.6/supply-chain-v0.0.6.tar.gz",
        "https://storage.googleapis.com/builddeps/5bd0cc7594ea528fd28f98d82457f157827d48cc20e07bcfdbb56072f35c8f67",
    ],
)

# bazel oci rules
http_archive(
    name = "rules_oci",
    sha256 = "e987cab7a35475cb9c9060fc3f338a1fc8896c240295a3272968b217acefd0cb",
    strip_prefix = "rules_oci-2.3.0",
    urls = [
        "https://github.com/bazel-contrib/rules_oci/releases/download/v2.3.0/rules_oci-v2.3.0.tar.gz",
        "https://storage.googleapis.com/builddeps/e987cab7a35475cb9c9060fc3f338a1fc8896c240295a3272968b217acefd0cb",
    ],
)

load("@rules_oci//oci:dependencies.bzl", "rules_oci_dependencies")

rules_oci_dependencies()

load("@rules_oci//oci:repositories.bzl", "oci_register_toolchains")

oci_register_toolchains(name = "oci")

load("@rules_oci//oci:pull.bzl", "oci_pull")

# Pull base image container registry
oci_pull(
    name = "registry",
    digest = "sha256:5c98b00f91e8daed324cb680661e9d647f09d825778493ffb2618ff36bec2a9e",
    image = "quay.io/libpod/registry",
    tag = "2.8",
)

oci_pull(
    name = "registry-aarch64",
    digest = "sha256:f4e803a2d37afca6d059961f28d73c57cbe6fdb3a44ba6ae7ad463811f43b81c",
    image = "quay.io/libpod/registry",
    tag = "2.8",
)

oci_pull(
    name = "registry-s390x",
    digest = "sha256:7e1926b82e5b862a633b83acf8f456e1619be720aff346e1b634db2f843082b7",
    image = "quay.io/libpod/registry",
    tag = "2.8",
)

http_file(
    name = "vcenter-govc-tar",
    downloaded_file_path = "govc.tar.gz",
    sha256 = "bfad9df590e061e28cfdd2c321583e96abd43e07687980f5897825ec13ff2cb5",
    urls = [
        "https://github.com/vmware/govmomi/releases/download/v0.26.1/govc_Linux_x86_64.tar.gz",
        "https://storage.googleapis.com/builddeps/bfad9df590e061e28cfdd2c321583e96abd43e07687980f5897825ec13ff2cb5",
    ],
)

http_file(
    name = "vcenter-vcsim-tar",
    downloaded_file_path = "vcsim.tar.gz",
    sha256 = "b844f6f7645c870a503aa1c5bd23d9a3cb4f5c850505073eef521f2f22a5f2b7",
    urls = [
        "https://github.com/vmware/govmomi/releases/download/v0.26.1/vcsim_Linux_x86_64.tar.gz",
        "https://storage.googleapis.com/builddeps/b844f6f7645c870a503aa1c5bd23d9a3cb4f5c850505073eef521f2f22a5f2b7",
    ],
)

rpm(
    name = "aardvark-dns-3__1.17.1-1.el9.aarch64",
    sha256 = "bfe3643c0407fdaa9f9e6a8dd80e92c2277ebe5921ce85a06f7bdcf43adae65c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/aardvark-dns-1.17.1-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/bfe3643c0407fdaa9f9e6a8dd80e92c2277ebe5921ce85a06f7bdcf43adae65c",
    ],
)

rpm(
    name = "aardvark-dns-3__1.17.1-1.el9.s390x",
    sha256 = "5d854162e8076e723fdf580e5ca6caffd93eedbe9827a06c9f8b3c9da687205d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/aardvark-dns-1.17.1-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/5d854162e8076e723fdf580e5ca6caffd93eedbe9827a06c9f8b3c9da687205d",
    ],
)

rpm(
    name = "aardvark-dns-3__1.17.1-1.el9.x86_64",
    sha256 = "27406856d46f0b69787b7b6f8fc61d306a2844aa738296032ddc1cdbc74e541d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/aardvark-dns-1.17.1-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/27406856d46f0b69787b7b6f8fc61d306a2844aa738296032ddc1cdbc74e541d",
    ],
)

rpm(
    name = "aardvark-dns-4__2.1.0-1.el10.aarch64",
    sha256 = "cd4d98f8640c48892aee8d17fdcdda1079b9f6c0475ac46b07e8dd2a0395080d",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/aardvark-dns-2.1.0-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "aardvark-dns-4__2.1.0-1.el10.s390x",
    sha256 = "858dcd1501f9e31111ed164aa17a27b261d57207fc708dc4e3efa0cf5352519e",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/aardvark-dns-2.1.0-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "aardvark-dns-4__2.1.0-1.el10.x86_64",
    sha256 = "ab33ade9071ce72422affc73f0d461a96eafd7da11ddcca4d7260fe4701e5b05",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/aardvark-dns-2.1.0-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "acl-0__2.3.1-3.el9.aarch64",
    sha256 = "151d6542a39243b5f65698b31edfe2d9c59e2fd71a7dcaa237442fc5d1d9de1e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/acl-2.3.1-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/151d6542a39243b5f65698b31edfe2d9c59e2fd71a7dcaa237442fc5d1d9de1e",
    ],
)

rpm(
    name = "acl-0__2.3.1-3.el9.x86_64",
    sha256 = "986044c3837eddbc9231d7be5e5fc517e245296978b988a803bc9f9172fe84ea",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/acl-2.3.1-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/986044c3837eddbc9231d7be5e5fc517e245296978b988a803bc9f9172fe84ea",
    ],
)

rpm(
    name = "acl-0__2.4.0-2.el10.aarch64",
    sha256 = "6d65bb2098442639ae5199ec4c8ee342b965dbc31f73277df63f04b4273f0278",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/acl-2.4.0-2.el10.aarch64.rpm",
    ],
)

rpm(
    name = "acl-0__2.4.0-2.el10.s390x",
    sha256 = "153742d20ad378247d8256619d2fca5666565befd8dbf0743966da331c993540",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/acl-2.4.0-2.el10.s390x.rpm",
    ],
)

rpm(
    name = "acl-0__2.4.0-2.el10.x86_64",
    sha256 = "9075286ab6c05da83934a21c4355372172f48c6ce5a566106e20ee84a7284c49",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/acl-2.4.0-2.el10.x86_64.rpm",
    ],
)

rpm(
    name = "acl-0__2.4.0-2.el9.aarch64",
    sha256 = "1d4ca14e2b7f2da10f486efe6d85822491cd961da95ba9218e155d671c095392",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/acl-2.4.0-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/1d4ca14e2b7f2da10f486efe6d85822491cd961da95ba9218e155d671c095392",
    ],
)

rpm(
    name = "acl-0__2.4.0-2.el9.s390x",
    sha256 = "d6199e8e6a09856716edbe6343fefed00d72f9bd7de9974671bc887275490f97",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/acl-2.4.0-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/d6199e8e6a09856716edbe6343fefed00d72f9bd7de9974671bc887275490f97",
    ],
)

rpm(
    name = "acl-0__2.4.0-2.el9.x86_64",
    sha256 = "706c131ab22d2b8f0834d8747a36a06d1c4c905d4e8ba79dd2ebfc33b4d8fa66",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/acl-2.4.0-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/706c131ab22d2b8f0834d8747a36a06d1c4c905d4e8ba79dd2ebfc33b4d8fa66",
    ],
)

rpm(
    name = "alternatives-0__1.20-2.el9.aarch64",
    sha256 = "4d9055232088f1ab181e4741358aa188749b8195f184817c04a61447606cdfb5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/alternatives-1.20-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/4d9055232088f1ab181e4741358aa188749b8195f184817c04a61447606cdfb5",
    ],
)

rpm(
    name = "alternatives-0__1.20-2.el9.x86_64",
    sha256 = "1851d5f64ebaeac67c5c2d9e4adc1e73aa6433b44a167268a3510c3d056062db",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/alternatives-1.20-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/1851d5f64ebaeac67c5c2d9e4adc1e73aa6433b44a167268a3510c3d056062db",
    ],
)

rpm(
    name = "alternatives-0__1.24-2.el9.aarch64",
    sha256 = "3b8d0d6154ccc1047474072afc94cc1f72b7c234d8cd4e50734c67ca67da4161",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/alternatives-1.24-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/3b8d0d6154ccc1047474072afc94cc1f72b7c234d8cd4e50734c67ca67da4161",
    ],
)

rpm(
    name = "alternatives-0__1.24-2.el9.s390x",
    sha256 = "8eb7ef117114059c44818eec88c4ed06c271a1185be1b1178ad096adcc934f11",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/alternatives-1.24-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/8eb7ef117114059c44818eec88c4ed06c271a1185be1b1178ad096adcc934f11",
    ],
)

rpm(
    name = "alternatives-0__1.24-2.el9.x86_64",
    sha256 = "1e9effe6f59312207b55f87eaded01e8f238622ad14018ffd33ef49e9ce8d4c6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/alternatives-1.24-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/1e9effe6f59312207b55f87eaded01e8f238622ad14018ffd33ef49e9ce8d4c6",
    ],
)

rpm(
    name = "alternatives-0__1.30-2.el10.aarch64",
    sha256 = "13d1cae28aecbc13bee2cf23391ec2ee41d39c51c9bb47f466fbad133d38f5c9",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/alternatives-1.30-2.el10.aarch64.rpm",
    ],
)

rpm(
    name = "alternatives-0__1.30-2.el10.s390x",
    sha256 = "ab4f800759f602c25f483681b126b4eced6ba81331c9b613dd47a229379c71e1",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/alternatives-1.30-2.el10.s390x.rpm",
    ],
)

rpm(
    name = "alternatives-0__1.30-2.el10.x86_64",
    sha256 = "1c8b83bf3dd0fa8d998a3c801986f50ea3661c2f8a21c60971c0391c381919c8",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/alternatives-1.30-2.el10.x86_64.rpm",
    ],
)

rpm(
    name = "audit-libs-0__3.1.5-9.el9.aarch64",
    sha256 = "549663937852f4b0d05fe15b58c79b6238a1aad109e0fb7aa942199849a95e6a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/audit-libs-3.1.5-9.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/549663937852f4b0d05fe15b58c79b6238a1aad109e0fb7aa942199849a95e6a",
    ],
)

rpm(
    name = "audit-libs-0__3.1.5-9.el9.s390x",
    sha256 = "19d59ce347764b65d73c8ed04259b04d4584ec7aceb195e48051af871da827c9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/audit-libs-3.1.5-9.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/19d59ce347764b65d73c8ed04259b04d4584ec7aceb195e48051af871da827c9",
    ],
)

rpm(
    name = "audit-libs-0__3.1.5-9.el9.x86_64",
    sha256 = "c405ef1e92bc64031546c0b45a4203e44e719f53ffaa387b04bdf630d47d1f00",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/audit-libs-3.1.5-9.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c405ef1e92bc64031546c0b45a4203e44e719f53ffaa387b04bdf630d47d1f00",
    ],
)

rpm(
    name = "audit-libs-0__4.2.1-3.el10.aarch64",
    sha256 = "6d5f5785f453a38fb9533ae970bf1ca2726febadaff57dc75487fcbee5e212d9",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/audit-libs-4.2.1-3.el10.aarch64.rpm",
    ],
)

rpm(
    name = "audit-libs-0__4.2.1-3.el10.s390x",
    sha256 = "4065f2c1319ebf39882c9ce19f69b74d477f4b0d526985750fc6b80c9622d6fe",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/audit-libs-4.2.1-3.el10.s390x.rpm",
    ],
)

rpm(
    name = "audit-libs-0__4.2.1-3.el10.x86_64",
    sha256 = "1791b8d94e4735a43fba7704097ff4c4b6623349753891d1c8951115c2cf29c2",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/audit-libs-4.2.1-3.el10.x86_64.rpm",
    ],
)

rpm(
    name = "basesystem-0__11-13.el9.aarch64",
    sha256 = "a7a687ef39dd28d01d34fab18ea7e3e87f649f6c202dded82260b7ea625b9973",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/basesystem-11-13.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/a7a687ef39dd28d01d34fab18ea7e3e87f649f6c202dded82260b7ea625b9973",
    ],
)

rpm(
    name = "basesystem-0__11-13.el9.s390x",
    sha256 = "a7a687ef39dd28d01d34fab18ea7e3e87f649f6c202dded82260b7ea625b9973",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/basesystem-11-13.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/a7a687ef39dd28d01d34fab18ea7e3e87f649f6c202dded82260b7ea625b9973",
    ],
)

rpm(
    name = "basesystem-0__11-13.el9.x86_64",
    sha256 = "a7a687ef39dd28d01d34fab18ea7e3e87f649f6c202dded82260b7ea625b9973",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/basesystem-11-13.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/a7a687ef39dd28d01d34fab18ea7e3e87f649f6c202dded82260b7ea625b9973",
    ],
)

rpm(
    name = "basesystem-0__11-22.el10.aarch64",
    sha256 = "76ff57f4d7565cd0e49f5e6dc38f3707dfe6a6b61317d883c2701be4277f2abf",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/basesystem-11-22.el10.noarch.rpm",
    ],
)

rpm(
    name = "basesystem-0__11-22.el10.s390x",
    sha256 = "76ff57f4d7565cd0e49f5e6dc38f3707dfe6a6b61317d883c2701be4277f2abf",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/basesystem-11-22.el10.noarch.rpm",
    ],
)

rpm(
    name = "basesystem-0__11-22.el10.x86_64",
    sha256 = "76ff57f4d7565cd0e49f5e6dc38f3707dfe6a6b61317d883c2701be4277f2abf",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/basesystem-11-22.el10.noarch.rpm",
    ],
)

rpm(
    name = "bash-0__5.1.8-4.el9.aarch64",
    sha256 = "ae6a63071aea7e9f0213abcced27505cc63b92718d68a9f529b5e3ac041fc1fa",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/bash-5.1.8-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ae6a63071aea7e9f0213abcced27505cc63b92718d68a9f529b5e3ac041fc1fa",
    ],
)

rpm(
    name = "bash-0__5.1.8-4.el9.x86_64",
    sha256 = "db30bb69faeb5a47da50d4a02639276ad083e49ca0579fbdd38d21dace0497aa",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/bash-5.1.8-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/db30bb69faeb5a47da50d4a02639276ad083e49ca0579fbdd38d21dace0497aa",
    ],
)

rpm(
    name = "bash-0__5.1.8-9.el9.aarch64",
    sha256 = "acb782e8dacd2f3efb25d0b8b1b64c59b8a60a84fc86a4fca88ede1affc68f4c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/bash-5.1.8-9.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/acb782e8dacd2f3efb25d0b8b1b64c59b8a60a84fc86a4fca88ede1affc68f4c",
    ],
)

rpm(
    name = "bash-0__5.1.8-9.el9.s390x",
    sha256 = "7f69429a343d53be5f3390e0e6032869c33cf1e9e344ee1448a4ec2998dc9d9e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/bash-5.1.8-9.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/7f69429a343d53be5f3390e0e6032869c33cf1e9e344ee1448a4ec2998dc9d9e",
    ],
)

rpm(
    name = "bash-0__5.1.8-9.el9.x86_64",
    sha256 = "823859a9e8fad83004fa0d9f698ff223f6f7d38fd8e7629509d98b5ba6764c03",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/bash-5.1.8-9.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/823859a9e8fad83004fa0d9f698ff223f6f7d38fd8e7629509d98b5ba6764c03",
    ],
)

rpm(
    name = "bash-0__5.2.26-7.el10.aarch64",
    sha256 = "cb75cf49059d0545bc53c1fd3e229b2f0225773ebe121b0ed297aca17dfed44d",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/bash-5.2.26-7.el10.aarch64.rpm",
    ],
)

rpm(
    name = "bash-0__5.2.26-7.el10.s390x",
    sha256 = "50e3f681aa81a9d621b7ad1442378a18684210936fa93cfc54fb29fb5f6492d9",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/bash-5.2.26-7.el10.s390x.rpm",
    ],
)

rpm(
    name = "bash-0__5.2.26-7.el10.x86_64",
    sha256 = "3454da002cbda046a8e49b2c570093cb0d84699dda5a589345dc0b67ddea874f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/bash-5.2.26-7.el10.x86_64.rpm",
    ],
)

rpm(
    name = "buildah-2__1.43.2-3.el9.aarch64",
    sha256 = "18cdf444643c2b03d462c45d99eede4500d67348e825bb04703dffefeffb002d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/buildah-1.43.2-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/18cdf444643c2b03d462c45d99eede4500d67348e825bb04703dffefeffb002d",
    ],
)

rpm(
    name = "buildah-2__1.43.2-3.el9.s390x",
    sha256 = "9ce402039e29359e5ac3145b5c9e96ab51be6f4643a8377ba00d14630b306379",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/buildah-1.43.2-3.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/9ce402039e29359e5ac3145b5c9e96ab51be6f4643a8377ba00d14630b306379",
    ],
)

rpm(
    name = "buildah-2__1.43.2-3.el9.x86_64",
    sha256 = "bb1939879b1813cd2e0876ec37cc1e2635cd9a690eb49a86420f64b0c42c844c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/buildah-1.43.2-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/bb1939879b1813cd2e0876ec37cc1e2635cd9a690eb49a86420f64b0c42c844c",
    ],
)

rpm(
    name = "buildah-2__1.45.0-3.el10.aarch64",
    sha256 = "53a446cda3710c7a9d2ad6e3f5fba6e6bbfb8a8e716e573ba701756656a6b020",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/aarch64/os/Packages/buildah-1.45.0-3.el10.aarch64.rpm",
    ],
)

rpm(
    name = "buildah-2__1.45.0-3.el10.s390x",
    sha256 = "e61a19df7e0e284643e4a022bd2fe6d6644e1229747a615524d4991a618f9ddb",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/s390x/os/Packages/buildah-1.45.0-3.el10.s390x.rpm",
    ],
)

rpm(
    name = "buildah-2__1.45.0-3.el10.x86_64",
    sha256 = "b933c360d1132e2b98a0c2a224842301607c0be0e0663df0fa0990ac12043d4c",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/x86_64/os/Packages/buildah-1.45.0-3.el10.x86_64.rpm",
    ],
)

rpm(
    name = "bzip2-libs-0__1.0.8-11.el9.aarch64",
    sha256 = "fafc0f2b7632774d4c07264c73eebbe52f815b4c81056bd44b944e5255cb20bb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/bzip2-libs-1.0.8-11.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/fafc0f2b7632774d4c07264c73eebbe52f815b4c81056bd44b944e5255cb20bb",
    ],
)

rpm(
    name = "bzip2-libs-0__1.0.8-11.el9.s390x",
    sha256 = "e9746e7bd442b4104b726e239cf3b7b87400824c7094de6d11f356da4c27593f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/bzip2-libs-1.0.8-11.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/e9746e7bd442b4104b726e239cf3b7b87400824c7094de6d11f356da4c27593f",
    ],
)

rpm(
    name = "bzip2-libs-0__1.0.8-11.el9.x86_64",
    sha256 = "e1f4ca1a16276a6ede5f67cab8d8d2920b98531419af7498f5fded85835e0fca",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/bzip2-libs-1.0.8-11.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e1f4ca1a16276a6ede5f67cab8d8d2920b98531419af7498f5fded85835e0fca",
    ],
)

rpm(
    name = "bzip2-libs-0__1.0.8-25.el10.aarch64",
    sha256 = "ac836c2c133077d0e71092f2c21e69d3985ace8458af527440e13b7edf165beb",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/bzip2-libs-1.0.8-25.el10.aarch64.rpm",
    ],
)

rpm(
    name = "bzip2-libs-0__1.0.8-25.el10.s390x",
    sha256 = "219adea56b92ecf22cb63fad38638e16115df270b78ea1fbd3cc1b183caf69a4",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/bzip2-libs-1.0.8-25.el10.s390x.rpm",
    ],
)

rpm(
    name = "bzip2-libs-0__1.0.8-25.el10.x86_64",
    sha256 = "309c7dbb857254655c51c4ab02d8038137c1363058542d8701c9272609f5b433",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/bzip2-libs-1.0.8-25.el10.x86_64.rpm",
    ],
)

rpm(
    name = "ca-certificates-0__2020.2.50-94.el9.aarch64",
    sha256 = "3099471d984fb7d9e1cf42406eb08c154b34b8560742ed1f5eb9139f059c2d09",
    urls = ["https://storage.googleapis.com/builddeps/3099471d984fb7d9e1cf42406eb08c154b34b8560742ed1f5eb9139f059c2d09"],
)

rpm(
    name = "ca-certificates-0__2020.2.50-94.el9.x86_64",
    sha256 = "3099471d984fb7d9e1cf42406eb08c154b34b8560742ed1f5eb9139f059c2d09",
    urls = ["https://storage.googleapis.com/builddeps/3099471d984fb7d9e1cf42406eb08c154b34b8560742ed1f5eb9139f059c2d09"],
)

rpm(
    name = "ca-certificates-0__2026.2.90_v9.0.316-90.0.el9.aarch64",
    sha256 = "bc306dcea17c26ec30a97cdd094d2a74646de77ea5150192bbae33dc1e8daf30",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/ca-certificates-2026.2.90_v9.0.316-90.0.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/bc306dcea17c26ec30a97cdd094d2a74646de77ea5150192bbae33dc1e8daf30",
    ],
)

rpm(
    name = "ca-certificates-0__2026.2.90_v9.0.316-90.0.el9.s390x",
    sha256 = "bc306dcea17c26ec30a97cdd094d2a74646de77ea5150192bbae33dc1e8daf30",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/ca-certificates-2026.2.90_v9.0.316-90.0.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/bc306dcea17c26ec30a97cdd094d2a74646de77ea5150192bbae33dc1e8daf30",
    ],
)

rpm(
    name = "ca-certificates-0__2026.2.90_v9.0.316-90.0.el9.x86_64",
    sha256 = "bc306dcea17c26ec30a97cdd094d2a74646de77ea5150192bbae33dc1e8daf30",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ca-certificates-2026.2.90_v9.0.316-90.0.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/bc306dcea17c26ec30a97cdd094d2a74646de77ea5150192bbae33dc1e8daf30",
    ],
)

rpm(
    name = "ca-certificates-0__2026.2.92_v9.0.319-100.3.el10.aarch64",
    sha256 = "326bcbf781996f904e983967b27c78cf8147079cb2142712ea4a0a2389924f42",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/ca-certificates-2026.2.92_v9.0.319-100.3.el10.noarch.rpm",
    ],
)

rpm(
    name = "ca-certificates-0__2026.2.92_v9.0.319-100.3.el10.s390x",
    sha256 = "326bcbf781996f904e983967b27c78cf8147079cb2142712ea4a0a2389924f42",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/ca-certificates-2026.2.92_v9.0.319-100.3.el10.noarch.rpm",
    ],
)

rpm(
    name = "ca-certificates-0__2026.2.92_v9.0.319-100.3.el10.x86_64",
    sha256 = "326bcbf781996f904e983967b27c78cf8147079cb2142712ea4a0a2389924f42",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/ca-certificates-2026.2.92_v9.0.319-100.3.el10.noarch.rpm",
    ],
)

rpm(
    name = "centos-gpg-keys-0__10.0-23.el10.aarch64",
    sha256 = "f223143fc00d956376d9f6361a538037e738a469ad79d4d82555e99a307c510f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/centos-gpg-keys-10.0-23.el10.noarch.rpm",
    ],
)

rpm(
    name = "centos-gpg-keys-0__10.0-23.el10.s390x",
    sha256 = "f223143fc00d956376d9f6361a538037e738a469ad79d4d82555e99a307c510f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/centos-gpg-keys-10.0-23.el10.noarch.rpm",
    ],
)

rpm(
    name = "centos-gpg-keys-0__10.0-23.el10.x86_64",
    sha256 = "f223143fc00d956376d9f6361a538037e738a469ad79d4d82555e99a307c510f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/centos-gpg-keys-10.0-23.el10.noarch.rpm",
    ],
)

rpm(
    name = "centos-gpg-keys-0__9.0-12.el9.aarch64",
    sha256 = "3af698b9f4dbf5368d1454df4e06cb8ffb75247b7b8385cfb0f7698f3db7d3ab",
    urls = ["https://storage.googleapis.com/builddeps/3af698b9f4dbf5368d1454df4e06cb8ffb75247b7b8385cfb0f7698f3db7d3ab"],
)

rpm(
    name = "centos-gpg-keys-0__9.0-12.el9.x86_64",
    sha256 = "3af698b9f4dbf5368d1454df4e06cb8ffb75247b7b8385cfb0f7698f3db7d3ab",
    urls = ["https://storage.googleapis.com/builddeps/3af698b9f4dbf5368d1454df4e06cb8ffb75247b7b8385cfb0f7698f3db7d3ab"],
)

rpm(
    name = "centos-gpg-keys-0__9.0-38.el9.aarch64",
    sha256 = "b6dcd5a16160ab017bf5e871975aef477d34ce61b660eeb3f4aa6973dcc6f916",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-gpg-keys-9.0-38.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/b6dcd5a16160ab017bf5e871975aef477d34ce61b660eeb3f4aa6973dcc6f916",
    ],
)

rpm(
    name = "centos-gpg-keys-0__9.0-38.el9.s390x",
    sha256 = "b6dcd5a16160ab017bf5e871975aef477d34ce61b660eeb3f4aa6973dcc6f916",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/centos-gpg-keys-9.0-38.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/b6dcd5a16160ab017bf5e871975aef477d34ce61b660eeb3f4aa6973dcc6f916",
    ],
)

rpm(
    name = "centos-gpg-keys-0__9.0-38.el9.x86_64",
    sha256 = "b6dcd5a16160ab017bf5e871975aef477d34ce61b660eeb3f4aa6973dcc6f916",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-gpg-keys-9.0-38.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/b6dcd5a16160ab017bf5e871975aef477d34ce61b660eeb3f4aa6973dcc6f916",
    ],
)

rpm(
    name = "centos-logos-httpd-0__100.5-1.el10.aarch64",
    sha256 = "e465c241d1ee9bf4ae5bf0bdf90d34ccf079b12ac2b746e7d0eb028337c7cbd4",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/aarch64/os/Packages/centos-logos-httpd-100.5-1.el10.noarch.rpm",
    ],
)

rpm(
    name = "centos-logos-httpd-0__100.5-1.el10.s390x",
    sha256 = "e465c241d1ee9bf4ae5bf0bdf90d34ccf079b12ac2b746e7d0eb028337c7cbd4",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/s390x/os/Packages/centos-logos-httpd-100.5-1.el10.noarch.rpm",
    ],
)

rpm(
    name = "centos-logos-httpd-0__100.5-1.el10.x86_64",
    sha256 = "e465c241d1ee9bf4ae5bf0bdf90d34ccf079b12ac2b746e7d0eb028337c7cbd4",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/x86_64/os/Packages/centos-logos-httpd-100.5-1.el10.noarch.rpm",
    ],
)

rpm(
    name = "centos-logos-httpd-0__90.9-1.el9.aarch64",
    sha256 = "0a6e9d58e4941b43b115c90aa468fe3b335a938a805c18676896dc93587b741d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/centos-logos-httpd-90.9-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/0a6e9d58e4941b43b115c90aa468fe3b335a938a805c18676896dc93587b741d",
    ],
)

rpm(
    name = "centos-logos-httpd-0__90.9-1.el9.s390x",
    sha256 = "0a6e9d58e4941b43b115c90aa468fe3b335a938a805c18676896dc93587b741d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/centos-logos-httpd-90.9-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/0a6e9d58e4941b43b115c90aa468fe3b335a938a805c18676896dc93587b741d",
    ],
)

rpm(
    name = "centos-logos-httpd-0__90.9-1.el9.x86_64",
    sha256 = "0a6e9d58e4941b43b115c90aa468fe3b335a938a805c18676896dc93587b741d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/centos-logos-httpd-90.9-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/0a6e9d58e4941b43b115c90aa468fe3b335a938a805c18676896dc93587b741d",
    ],
)

rpm(
    name = "centos-stream-release-0__10.0-23.el10.aarch64",
    sha256 = "3c0ca8ad94e7b4dfdbe3a0c6ed088c560c57815779cdee33b00b8980894d760f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/centos-stream-release-10.0-23.el10.noarch.rpm",
    ],
)

rpm(
    name = "centos-stream-release-0__10.0-23.el10.s390x",
    sha256 = "3c0ca8ad94e7b4dfdbe3a0c6ed088c560c57815779cdee33b00b8980894d760f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/centos-stream-release-10.0-23.el10.noarch.rpm",
    ],
)

rpm(
    name = "centos-stream-release-0__10.0-23.el10.x86_64",
    sha256 = "3c0ca8ad94e7b4dfdbe3a0c6ed088c560c57815779cdee33b00b8980894d760f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/centos-stream-release-10.0-23.el10.noarch.rpm",
    ],
)

rpm(
    name = "centos-stream-release-0__9.0-12.el9.aarch64",
    sha256 = "400b77fb28443d36a6fa3c25c95e84b843ac9ae17b205651f1e2bea32c7289cc",
    urls = ["https://storage.googleapis.com/builddeps/400b77fb28443d36a6fa3c25c95e84b843ac9ae17b205651f1e2bea32c7289cc"],
)

rpm(
    name = "centos-stream-release-0__9.0-12.el9.x86_64",
    sha256 = "400b77fb28443d36a6fa3c25c95e84b843ac9ae17b205651f1e2bea32c7289cc",
    urls = ["https://storage.googleapis.com/builddeps/400b77fb28443d36a6fa3c25c95e84b843ac9ae17b205651f1e2bea32c7289cc"],
)

rpm(
    name = "centos-stream-release-0__9.0-38.el9.aarch64",
    sha256 = "04a24747b2884f59d8ac5583f162dbc5ed043f5ccf602ef6274349f1d7ca9a8e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-stream-release-9.0-38.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/04a24747b2884f59d8ac5583f162dbc5ed043f5ccf602ef6274349f1d7ca9a8e",
    ],
)

rpm(
    name = "centos-stream-release-0__9.0-38.el9.s390x",
    sha256 = "04a24747b2884f59d8ac5583f162dbc5ed043f5ccf602ef6274349f1d7ca9a8e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/centos-stream-release-9.0-38.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/04a24747b2884f59d8ac5583f162dbc5ed043f5ccf602ef6274349f1d7ca9a8e",
    ],
)

rpm(
    name = "centos-stream-release-0__9.0-38.el9.x86_64",
    sha256 = "04a24747b2884f59d8ac5583f162dbc5ed043f5ccf602ef6274349f1d7ca9a8e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-stream-release-9.0-38.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/04a24747b2884f59d8ac5583f162dbc5ed043f5ccf602ef6274349f1d7ca9a8e",
    ],
)

rpm(
    name = "centos-stream-repos-0__10.0-23.el10.aarch64",
    sha256 = "f08be10e29ff190f941fad1335dd0528f264ee1afaf1836758609a554c028d00",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/centos-stream-repos-10.0-23.el10.noarch.rpm",
    ],
)

rpm(
    name = "centos-stream-repos-0__10.0-23.el10.s390x",
    sha256 = "f08be10e29ff190f941fad1335dd0528f264ee1afaf1836758609a554c028d00",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/centos-stream-repos-10.0-23.el10.noarch.rpm",
    ],
)

rpm(
    name = "centos-stream-repos-0__10.0-23.el10.x86_64",
    sha256 = "f08be10e29ff190f941fad1335dd0528f264ee1afaf1836758609a554c028d00",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/centos-stream-repos-10.0-23.el10.noarch.rpm",
    ],
)

rpm(
    name = "centos-stream-repos-0__9.0-12.el9.aarch64",
    sha256 = "d093d9f9021a8edc28843f61059a94bd8aa0109f6a9a865c2a1560cf6602a2ab",
    urls = ["https://storage.googleapis.com/builddeps/d093d9f9021a8edc28843f61059a94bd8aa0109f6a9a865c2a1560cf6602a2ab"],
)

rpm(
    name = "centos-stream-repos-0__9.0-12.el9.x86_64",
    sha256 = "d093d9f9021a8edc28843f61059a94bd8aa0109f6a9a865c2a1560cf6602a2ab",
    urls = ["https://storage.googleapis.com/builddeps/d093d9f9021a8edc28843f61059a94bd8aa0109f6a9a865c2a1560cf6602a2ab"],
)

rpm(
    name = "centos-stream-repos-0__9.0-38.el9.aarch64",
    sha256 = "ae98322c35b3eb7b013c8989a8b318993e3bec71018e213f729a156c2bbd508d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-stream-repos-9.0-38.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/ae98322c35b3eb7b013c8989a8b318993e3bec71018e213f729a156c2bbd508d",
    ],
)

rpm(
    name = "centos-stream-repos-0__9.0-38.el9.s390x",
    sha256 = "ae98322c35b3eb7b013c8989a8b318993e3bec71018e213f729a156c2bbd508d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/centos-stream-repos-9.0-38.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/ae98322c35b3eb7b013c8989a8b318993e3bec71018e213f729a156c2bbd508d",
    ],
)

rpm(
    name = "centos-stream-repos-0__9.0-38.el9.x86_64",
    sha256 = "ae98322c35b3eb7b013c8989a8b318993e3bec71018e213f729a156c2bbd508d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-stream-repos-9.0-38.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/ae98322c35b3eb7b013c8989a8b318993e3bec71018e213f729a156c2bbd508d",
    ],
)

rpm(
    name = "containers-common-5__5.8-2.el9.aarch64",
    sha256 = "59b32953d12fc9cd2d2f4212831c4e250c3dc5865ca096983d3b8ae2bae4ac93",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/containers-common-5.8-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/59b32953d12fc9cd2d2f4212831c4e250c3dc5865ca096983d3b8ae2bae4ac93",
    ],
)

rpm(
    name = "containers-common-5__5.8-2.el9.s390x",
    sha256 = "a76ba8b0ddd8d83124e14aa8ddeb248d28b1d9fcc56164c869d188c449071358",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/containers-common-5.8-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/a76ba8b0ddd8d83124e14aa8ddeb248d28b1d9fcc56164c869d188c449071358",
    ],
)

rpm(
    name = "containers-common-5__5.8-2.el9.x86_64",
    sha256 = "1f79332e4adb327d9e9c0a8340f26bfe42240a6f0839b71525b6ee70d30ebe59",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/containers-common-5.8-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/1f79332e4adb327d9e9c0a8340f26bfe42240a6f0839b71525b6ee70d30ebe59",
    ],
)

rpm(
    name = "containers-common-6__6.0-3.el10.aarch64",
    sha256 = "7e3ef0bead7cb7942c5757f2dff62b8cfc8f12442e1b86bd8e33c3dfff19a1b9",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/containers-common-6.0-3.el10.noarch.rpm",
    ],
)

rpm(
    name = "containers-common-6__6.0-3.el10.s390x",
    sha256 = "7e3ef0bead7cb7942c5757f2dff62b8cfc8f12442e1b86bd8e33c3dfff19a1b9",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/containers-common-6.0-3.el10.noarch.rpm",
    ],
)

rpm(
    name = "containers-common-6__6.0-3.el10.x86_64",
    sha256 = "7e3ef0bead7cb7942c5757f2dff62b8cfc8f12442e1b86bd8e33c3dfff19a1b9",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/containers-common-6.0-3.el10.noarch.rpm",
    ],
)

rpm(
    name = "containers-common-extra-5__5.8-2.el9.aarch64",
    sha256 = "707de6e031e0c86b7161657110e86823171609ddbc83e11cd2d1ee46d8d78a63",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/containers-common-extra-5.8-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/707de6e031e0c86b7161657110e86823171609ddbc83e11cd2d1ee46d8d78a63",
    ],
)

rpm(
    name = "containers-common-extra-5__5.8-2.el9.s390x",
    sha256 = "15b5d88f322b13f99dab297741737056955a14adc26eaf0b8e86349f3c3e8a8a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/containers-common-extra-5.8-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/15b5d88f322b13f99dab297741737056955a14adc26eaf0b8e86349f3c3e8a8a",
    ],
)

rpm(
    name = "containers-common-extra-5__5.8-2.el9.x86_64",
    sha256 = "3d4d0d9a32f1a37b4170894e67a3b1b18da45a098d9c3e2307f3fdccfd8c1cdb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/containers-common-extra-5.8-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/3d4d0d9a32f1a37b4170894e67a3b1b18da45a098d9c3e2307f3fdccfd8c1cdb",
    ],
)

rpm(
    name = "containers-common-extra-6__6.0-3.el10.aarch64",
    sha256 = "7e20d7c7a54b0353664ab0b03dbe2b86cecffe56febd7cc8b035d69efbaa5e9a",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/containers-common-extra-6.0-3.el10.noarch.rpm",
    ],
)

rpm(
    name = "containers-common-extra-6__6.0-3.el10.s390x",
    sha256 = "7e20d7c7a54b0353664ab0b03dbe2b86cecffe56febd7cc8b035d69efbaa5e9a",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/containers-common-extra-6.0-3.el10.noarch.rpm",
    ],
)

rpm(
    name = "containers-common-extra-6__6.0-3.el10.x86_64",
    sha256 = "7e20d7c7a54b0353664ab0b03dbe2b86cecffe56febd7cc8b035d69efbaa5e9a",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/containers-common-extra-6.0-3.el10.noarch.rpm",
    ],
)

rpm(
    name = "coreutils-single-0__8.32-31.el9.aarch64",
    sha256 = "e2d2e94d4322f41cb7331b0e8c23f937b08f37514826d78fb4ed4d1bbea3ef5b",
    urls = ["https://storage.googleapis.com/builddeps/e2d2e94d4322f41cb7331b0e8c23f937b08f37514826d78fb4ed4d1bbea3ef5b"],
)

rpm(
    name = "coreutils-single-0__8.32-31.el9.x86_64",
    sha256 = "fcae4e00df1cb3d0eb214d166045150aede7262559bd03fc585610fe1ea59c08",
    urls = ["https://storage.googleapis.com/builddeps/fcae4e00df1cb3d0eb214d166045150aede7262559bd03fc585610fe1ea59c08"],
)

rpm(
    name = "coreutils-single-0__8.32-43.el9.aarch64",
    sha256 = "b0a8cc4a393db9851ce5efc9fe58557a71ed9d0d92a4619ae31d951b6db2dcff",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/coreutils-single-8.32-43.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/b0a8cc4a393db9851ce5efc9fe58557a71ed9d0d92a4619ae31d951b6db2dcff",
    ],
)

rpm(
    name = "coreutils-single-0__8.32-43.el9.s390x",
    sha256 = "2032300685afbeacc818cfd43a1e9c1ae991e4d0674a9dd0475a5ffcd7ceba16",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/coreutils-single-8.32-43.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/2032300685afbeacc818cfd43a1e9c1ae991e4d0674a9dd0475a5ffcd7ceba16",
    ],
)

rpm(
    name = "coreutils-single-0__8.32-43.el9.x86_64",
    sha256 = "7cb048fc00e364bfbc1c12f26b9b39a062e426b0e45cfb4421ccf5493ec8a2c8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/coreutils-single-8.32-43.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/7cb048fc00e364bfbc1c12f26b9b39a062e426b0e45cfb4421ccf5493ec8a2c8",
    ],
)

rpm(
    name = "coreutils-single-0__9.5-14.el10.aarch64",
    sha256 = "496744727e47d17fc2303a43b75d5b63cbfbd082df852ecf20a350ba9a8dcc06",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/coreutils-single-9.5-14.el10.aarch64.rpm",
    ],
)

rpm(
    name = "coreutils-single-0__9.5-14.el10.s390x",
    sha256 = "158076f774143f235b28df1a0cd4d0371ea01b609dc106badab8a5c7ec48331b",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/coreutils-single-9.5-14.el10.s390x.rpm",
    ],
)

rpm(
    name = "coreutils-single-0__9.5-14.el10.x86_64",
    sha256 = "6897457347ce21d8c5bbd8296131a5eb5a76c5e38816eb4d4c8992864038ba68",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/coreutils-single-9.5-14.el10.x86_64.rpm",
    ],
)

rpm(
    name = "cracklib-0__2.9.6-28.el9.aarch64",
    sha256 = "78dbd83e4de7c011dedc8071af056989dece25dae7605eb60703b219ebbeadc1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/cracklib-2.9.6-28.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/78dbd83e4de7c011dedc8071af056989dece25dae7605eb60703b219ebbeadc1",
    ],
)

rpm(
    name = "cracklib-0__2.9.6-28.el9.s390x",
    sha256 = "14006fd9132581ca7ab86b87eb4751efd25279bc60df48aced985002e401112d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/cracklib-2.9.6-28.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/14006fd9132581ca7ab86b87eb4751efd25279bc60df48aced985002e401112d",
    ],
)

rpm(
    name = "cracklib-0__2.9.6-28.el9.x86_64",
    sha256 = "aa659fc5fc1f40d9301850411e1e4cfb9351175e1879a1d404292cbd909982f0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/cracklib-2.9.6-28.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/aa659fc5fc1f40d9301850411e1e4cfb9351175e1879a1d404292cbd909982f0",
    ],
)

rpm(
    name = "cracklib-dicts-0__2.9.6-28.el9.aarch64",
    sha256 = "3b449db83d1a649b93eff386e098ab01f24028b106827d9fef899abc99818b15",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/cracklib-dicts-2.9.6-28.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/3b449db83d1a649b93eff386e098ab01f24028b106827d9fef899abc99818b15",
    ],
)

rpm(
    name = "cracklib-dicts-0__2.9.6-28.el9.s390x",
    sha256 = "a0ac88ff592620ae37ea0826d59874f0f5a08828c02fcd514473302d15cf6c03",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/cracklib-dicts-2.9.6-28.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/a0ac88ff592620ae37ea0826d59874f0f5a08828c02fcd514473302d15cf6c03",
    ],
)

rpm(
    name = "cracklib-dicts-0__2.9.6-28.el9.x86_64",
    sha256 = "b0e372c09e6eb01d2de1316b7e59c79178c0eaee6d713004d7fe5fbc7e718603",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/cracklib-dicts-2.9.6-28.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/b0e372c09e6eb01d2de1316b7e59c79178c0eaee6d713004d7fe5fbc7e718603",
    ],
)

rpm(
    name = "crun-0__1.29.1-1.el9.aarch64",
    sha256 = "63bcc7f8566b73e49a971a66ebde7c5828949b75524b46483e6f560a59792679",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/crun-1.29.1-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/63bcc7f8566b73e49a971a66ebde7c5828949b75524b46483e6f560a59792679",
    ],
)

rpm(
    name = "crun-0__1.29.1-1.el9.s390x",
    sha256 = "42561cda461ca088ea1c67e1de624a67c5a22a398486803f1d3bb9a8d6713925",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/crun-1.29.1-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/42561cda461ca088ea1c67e1de624a67c5a22a398486803f1d3bb9a8d6713925",
    ],
)

rpm(
    name = "crun-0__1.29.1-1.el9.x86_64",
    sha256 = "f6b0cd9b313e286cee2264a85946a99fd37556c54ec0da05f5ae757d2e5db866",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/crun-1.29.1-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f6b0cd9b313e286cee2264a85946a99fd37556c54ec0da05f5ae757d2e5db866",
    ],
)

rpm(
    name = "crun-0__1.30.1-1.el10.aarch64",
    sha256 = "c0fe3a22027b49cd6c7b73e94a1a43ae5538a0f79c3b34f97097cdc13552bd79",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/aarch64/os/Packages/crun-1.30.1-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "crun-0__1.30.1-1.el10.s390x",
    sha256 = "0f9d8c3b11bccb2347aaf9fc6d95965b38693c9879ec33f2ab653e1406b055f8",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/s390x/os/Packages/crun-1.30.1-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "crun-0__1.30.1-1.el10.x86_64",
    sha256 = "ea5c83a6dc397bd0da24957e84a1a3cae684dfa4e5b00af690778fd22166403b",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/x86_64/os/Packages/crun-1.30.1-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "crypto-policies-0__20220223-1.git5203b41.el9.aarch64",
    sha256 = "9912a52ab2fcb33f39a574a84f5ca6ced9426536d4e025c29702886419a12c8f",
    urls = ["https://storage.googleapis.com/builddeps/9912a52ab2fcb33f39a574a84f5ca6ced9426536d4e025c29702886419a12c8f"],
)

rpm(
    name = "crypto-policies-0__20220223-1.git5203b41.el9.x86_64",
    sha256 = "9912a52ab2fcb33f39a574a84f5ca6ced9426536d4e025c29702886419a12c8f",
    urls = ["https://storage.googleapis.com/builddeps/9912a52ab2fcb33f39a574a84f5ca6ced9426536d4e025c29702886419a12c8f"],
)

rpm(
    name = "crypto-policies-0__20260714-1.git65b74f7.el9.aarch64",
    sha256 = "6946742c90dbbebb74840fd0b35101144b51bd7a93aaef3943a2ec87fcdafdfe",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/crypto-policies-20260714-1.git65b74f7.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/6946742c90dbbebb74840fd0b35101144b51bd7a93aaef3943a2ec87fcdafdfe",
    ],
)

rpm(
    name = "crypto-policies-0__20260714-1.git65b74f7.el9.s390x",
    sha256 = "6946742c90dbbebb74840fd0b35101144b51bd7a93aaef3943a2ec87fcdafdfe",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/crypto-policies-20260714-1.git65b74f7.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/6946742c90dbbebb74840fd0b35101144b51bd7a93aaef3943a2ec87fcdafdfe",
    ],
)

rpm(
    name = "crypto-policies-0__20260714-1.git65b74f7.el9.x86_64",
    sha256 = "6946742c90dbbebb74840fd0b35101144b51bd7a93aaef3943a2ec87fcdafdfe",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/crypto-policies-20260714-1.git65b74f7.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/6946742c90dbbebb74840fd0b35101144b51bd7a93aaef3943a2ec87fcdafdfe",
    ],
)

rpm(
    name = "crypto-policies-0__20260811-1.git4dcd294.el10.aarch64",
    sha256 = "6935c44a0cce68eddfe12c7e84f09263ae0a270f9834d9bf2b3d9c538af79007",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/crypto-policies-20260811-1.git4dcd294.el10.noarch.rpm",
    ],
)

rpm(
    name = "crypto-policies-0__20260811-1.git4dcd294.el10.s390x",
    sha256 = "6935c44a0cce68eddfe12c7e84f09263ae0a270f9834d9bf2b3d9c538af79007",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/crypto-policies-20260811-1.git4dcd294.el10.noarch.rpm",
    ],
)

rpm(
    name = "crypto-policies-0__20260811-1.git4dcd294.el10.x86_64",
    sha256 = "6935c44a0cce68eddfe12c7e84f09263ae0a270f9834d9bf2b3d9c538af79007",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/crypto-policies-20260811-1.git4dcd294.el10.noarch.rpm",
    ],
)

rpm(
    name = "crypto-policies-scripts-0__20260714-1.git65b74f7.el9.aarch64",
    sha256 = "8d148a30a8a089ff8b2dfca2e1e520f08d6d8ba5b1bcddeb49629bc8de3423d3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/crypto-policies-scripts-20260714-1.git65b74f7.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/8d148a30a8a089ff8b2dfca2e1e520f08d6d8ba5b1bcddeb49629bc8de3423d3",
    ],
)

rpm(
    name = "crypto-policies-scripts-0__20260714-1.git65b74f7.el9.s390x",
    sha256 = "8d148a30a8a089ff8b2dfca2e1e520f08d6d8ba5b1bcddeb49629bc8de3423d3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/crypto-policies-scripts-20260714-1.git65b74f7.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/8d148a30a8a089ff8b2dfca2e1e520f08d6d8ba5b1bcddeb49629bc8de3423d3",
    ],
)

rpm(
    name = "crypto-policies-scripts-0__20260714-1.git65b74f7.el9.x86_64",
    sha256 = "8d148a30a8a089ff8b2dfca2e1e520f08d6d8ba5b1bcddeb49629bc8de3423d3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/crypto-policies-scripts-20260714-1.git65b74f7.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/8d148a30a8a089ff8b2dfca2e1e520f08d6d8ba5b1bcddeb49629bc8de3423d3",
    ],
)

rpm(
    name = "crypto-policies-scripts-0__20260811-1.git4dcd294.el10.aarch64",
    sha256 = "fd8bca6bd7aa2f124760d3892ebbf66b04da93ac8220aa1ceff55d29cb0265a0",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/crypto-policies-scripts-20260811-1.git4dcd294.el10.noarch.rpm",
    ],
)

rpm(
    name = "crypto-policies-scripts-0__20260811-1.git4dcd294.el10.s390x",
    sha256 = "fd8bca6bd7aa2f124760d3892ebbf66b04da93ac8220aa1ceff55d29cb0265a0",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/crypto-policies-scripts-20260811-1.git4dcd294.el10.noarch.rpm",
    ],
)

rpm(
    name = "crypto-policies-scripts-0__20260811-1.git4dcd294.el10.x86_64",
    sha256 = "fd8bca6bd7aa2f124760d3892ebbf66b04da93ac8220aa1ceff55d29cb0265a0",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/crypto-policies-scripts-20260811-1.git4dcd294.el10.noarch.rpm",
    ],
)

rpm(
    name = "curl-0__7.76.1-14.el9.aarch64",
    sha256 = "c1ddc1be37854be9c97f0351aa585809e9d2e54c0dcbf77dbb33d85b29bc10e6",
    urls = ["https://storage.googleapis.com/builddeps/c1ddc1be37854be9c97f0351aa585809e9d2e54c0dcbf77dbb33d85b29bc10e6"],
)

rpm(
    name = "curl-0__7.76.1-14.el9.x86_64",
    sha256 = "9fb98bd7ebb8d210b77bca1c70aac00b0f0dfc6f776157e9c7f64fd7339bff3c",
    urls = ["https://storage.googleapis.com/builddeps/9fb98bd7ebb8d210b77bca1c70aac00b0f0dfc6f776157e9c7f64fd7339bff3c"],
)

rpm(
    name = "curl-0__7.76.1-43.el9.aarch64",
    sha256 = "6fd913acecfaef1a57ec3e9055398c31613773d44aa07670dd7be0fb4ccd119e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/curl-7.76.1-43.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/6fd913acecfaef1a57ec3e9055398c31613773d44aa07670dd7be0fb4ccd119e",
    ],
)

rpm(
    name = "curl-0__7.76.1-43.el9.s390x",
    sha256 = "adbad3914af90435ea1843e68d26f217e32a0ec9cb2d754afd2ad89101aced8a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/curl-7.76.1-43.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/adbad3914af90435ea1843e68d26f217e32a0ec9cb2d754afd2ad89101aced8a",
    ],
)

rpm(
    name = "curl-0__7.76.1-43.el9.x86_64",
    sha256 = "a17d00a4bc40bf90f060c607a6ec4d2459c640a84a706b01da3ce0855be9ef4a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/curl-7.76.1-43.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/a17d00a4bc40bf90f060c607a6ec4d2459c640a84a706b01da3ce0855be9ef4a",
    ],
)

rpm(
    name = "curl-0__8.12.1-9.el10.aarch64",
    sha256 = "edd7711a295a404e5ec0825e87c26d789735110b03235e032249ea5a50f6a8d1",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/curl-8.12.1-9.el10.aarch64.rpm",
    ],
)

rpm(
    name = "curl-0__8.12.1-9.el10.s390x",
    sha256 = "ffc52cc633e1bc2c0b9cce63ec07fb0a945f15db5d0b90d9c7b6d5e693ec9358",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/curl-8.12.1-9.el10.s390x.rpm",
    ],
)

rpm(
    name = "curl-0__8.12.1-9.el10.x86_64",
    sha256 = "449b3f46bc301de65ae64b5340920d46b1a302106efb3cd9351cf710c5ef9ec7",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/curl-8.12.1-9.el10.x86_64.rpm",
    ],
)

rpm(
    name = "curl-minimal-0__7.76.1-43.el9.x86_64",
    sha256 = "0ddf97bb566dd3c6c877b2a2fb895b252d13982a12f1cd407c9ca21a53ad0777",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/curl-minimal-7.76.1-43.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/0ddf97bb566dd3c6c877b2a2fb895b252d13982a12f1cd407c9ca21a53ad0777",
    ],
)

rpm(
    name = "cyrus-sasl-lib-0__2.1.27-21.el9.aarch64",
    sha256 = "898d7094964022ca527a6596550b8d46499b3274f8c6a1ee632a98961012d80c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/cyrus-sasl-lib-2.1.27-21.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/898d7094964022ca527a6596550b8d46499b3274f8c6a1ee632a98961012d80c",
    ],
)

rpm(
    name = "cyrus-sasl-lib-0__2.1.27-21.el9.s390x",
    sha256 = "e8954c3d19fc3aa905d09488c111df37bd5b9fe9c1eeec314420b3be2e75a74f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/cyrus-sasl-lib-2.1.27-21.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/e8954c3d19fc3aa905d09488c111df37bd5b9fe9c1eeec314420b3be2e75a74f",
    ],
)

rpm(
    name = "cyrus-sasl-lib-0__2.1.27-21.el9.x86_64",
    sha256 = "fd4292a29759f9531bbc876d1818e7a83ccac76907234002f598671d7b338469",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/cyrus-sasl-lib-2.1.27-21.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/fd4292a29759f9531bbc876d1818e7a83ccac76907234002f598671d7b338469",
    ],
)

rpm(
    name = "cyrus-sasl-lib-0__2.1.28-27.el10.aarch64",
    sha256 = "917d6b8d2eff0dd71b55646c758b938ac7b9f0a298f2dffae5948c9865215067",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/cyrus-sasl-lib-2.1.28-27.el10.aarch64.rpm",
    ],
)

rpm(
    name = "cyrus-sasl-lib-0__2.1.28-27.el10.s390x",
    sha256 = "b40557a0d21461db27adf093fe6a72ec17a243f6743a3d1e26c32601753e97ee",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/cyrus-sasl-lib-2.1.28-27.el10.s390x.rpm",
    ],
)

rpm(
    name = "cyrus-sasl-lib-0__2.1.28-27.el10.x86_64",
    sha256 = "ea78a83980b03f3709266f5e4c96b41699fe8d5f7003fb9503c3a7529c6ca46a",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/cyrus-sasl-lib-2.1.28-27.el10.x86_64.rpm",
    ],
)

rpm(
    name = "dbus-1__1.12.20-10.el9.aarch64",
    sha256 = "3628a340989bddcecd2c490ad9a7ce3fecb85a1ca28d154bf4c82db804eac6b0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/dbus-1.12.20-10.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/3628a340989bddcecd2c490ad9a7ce3fecb85a1ca28d154bf4c82db804eac6b0",
    ],
)

rpm(
    name = "dbus-1__1.12.20-10.el9.s390x",
    sha256 = "83a12ddb6d5c2d2f9f4f1a9e2f0e21848188177089fa57405b24644e218ac96d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/dbus-1.12.20-10.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/83a12ddb6d5c2d2f9f4f1a9e2f0e21848188177089fa57405b24644e218ac96d",
    ],
)

rpm(
    name = "dbus-1__1.12.20-10.el9.x86_64",
    sha256 = "c36fc19c6d1cd8d9d10499f5c9b6ac42db27b5cd525cb0b6480b16a476f2df5d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-1.12.20-10.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c36fc19c6d1cd8d9d10499f5c9b6ac42db27b5cd525cb0b6480b16a476f2df5d",
    ],
)

rpm(
    name = "dbus-1__1.14.10-6.el10.aarch64",
    sha256 = "b7d8f5dbdcb3977b2041f1c6f545d6720e6dbc29e4753c4bce11592cc55fadf4",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/dbus-1.14.10-6.el10.aarch64.rpm",
    ],
)

rpm(
    name = "dbus-1__1.14.10-6.el10.s390x",
    sha256 = "a3ec7863145c8e7fa999d671d7059c0dd815dab7370fefe7d9461b90d081ee2b",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/dbus-1.14.10-6.el10.s390x.rpm",
    ],
)

rpm(
    name = "dbus-1__1.14.10-6.el10.x86_64",
    sha256 = "019154fd71f6a8d9ad1f66de4e91e55ac02abdbe3553b6e3ec0ed42fc3dba557",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/dbus-1.14.10-6.el10.x86_64.rpm",
    ],
)

rpm(
    name = "dbus-broker-0__28-9.el9.aarch64",
    sha256 = "724c1f8142deda976f1b5c9a714e4e49864e61544b4ff507fc5078d416db6acc",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/dbus-broker-28-9.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/724c1f8142deda976f1b5c9a714e4e49864e61544b4ff507fc5078d416db6acc",
    ],
)

rpm(
    name = "dbus-broker-0__28-9.el9.s390x",
    sha256 = "83140ba4112cd7b62ebd9673d74af5d11153428652eacf84a2b254f4597b174e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/dbus-broker-28-9.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/83140ba4112cd7b62ebd9673d74af5d11153428652eacf84a2b254f4597b174e",
    ],
)

rpm(
    name = "dbus-broker-0__28-9.el9.x86_64",
    sha256 = "2b0215cb658182a774d7eb1e965c12d0512fddb1be983d8da746620dc183d0c2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-broker-28-9.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/2b0215cb658182a774d7eb1e965c12d0512fddb1be983d8da746620dc183d0c2",
    ],
)

rpm(
    name = "dbus-broker-0__36-5.el10.aarch64",
    sha256 = "2d489cfc06417c232a9f32a0cc06bdea5162225c8c9e0733f948bb00139ad2c9",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/dbus-broker-36-5.el10.aarch64.rpm",
    ],
)

rpm(
    name = "dbus-broker-0__36-5.el10.s390x",
    sha256 = "8de4794ea4f84b53b63857b34cb4a471d370d94292611f19d3fd4dc2c9a35740",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/dbus-broker-36-5.el10.s390x.rpm",
    ],
)

rpm(
    name = "dbus-broker-0__36-5.el10.x86_64",
    sha256 = "c60a8e71fe0ada8f9f69cf0d3ed80e484d71d21163836fdb82655be13544d91b",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/dbus-broker-36-5.el10.x86_64.rpm",
    ],
)

rpm(
    name = "dbus-common-1__1.12.20-10.el9.aarch64",
    sha256 = "6e522cb85f206cdac3e1c6d9ed7400a35364c7e014687c17b17431a8d10d07e2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/dbus-common-1.12.20-10.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/6e522cb85f206cdac3e1c6d9ed7400a35364c7e014687c17b17431a8d10d07e2",
    ],
)

rpm(
    name = "dbus-common-1__1.12.20-10.el9.s390x",
    sha256 = "6e522cb85f206cdac3e1c6d9ed7400a35364c7e014687c17b17431a8d10d07e2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/dbus-common-1.12.20-10.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/6e522cb85f206cdac3e1c6d9ed7400a35364c7e014687c17b17431a8d10d07e2",
    ],
)

rpm(
    name = "dbus-common-1__1.12.20-10.el9.x86_64",
    sha256 = "6e522cb85f206cdac3e1c6d9ed7400a35364c7e014687c17b17431a8d10d07e2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-common-1.12.20-10.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/6e522cb85f206cdac3e1c6d9ed7400a35364c7e014687c17b17431a8d10d07e2",
    ],
)

rpm(
    name = "dbus-common-1__1.14.10-6.el10.aarch64",
    sha256 = "46da64dfece403446ed8d674cfd0bccd979b2311069ac4facc3125847bf12bd9",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/dbus-common-1.14.10-6.el10.noarch.rpm",
    ],
)

rpm(
    name = "dbus-common-1__1.14.10-6.el10.s390x",
    sha256 = "46da64dfece403446ed8d674cfd0bccd979b2311069ac4facc3125847bf12bd9",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/dbus-common-1.14.10-6.el10.noarch.rpm",
    ],
)

rpm(
    name = "dbus-common-1__1.14.10-6.el10.x86_64",
    sha256 = "46da64dfece403446ed8d674cfd0bccd979b2311069ac4facc3125847bf12bd9",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/dbus-common-1.14.10-6.el10.noarch.rpm",
    ],
)

rpm(
    name = "expat-0__2.5.0-7.el9.aarch64",
    sha256 = "432b77a643134c60c91ebba495de17258686ec8a3deb74687e62207e4301fb2d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/expat-2.5.0-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/432b77a643134c60c91ebba495de17258686ec8a3deb74687e62207e4301fb2d",
    ],
)

rpm(
    name = "expat-0__2.5.0-7.el9.s390x",
    sha256 = "2b3de0a1d73d5698ff01838ddaa11ca9375036e07d5a8c21e02518f50728a339",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/expat-2.5.0-7.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/2b3de0a1d73d5698ff01838ddaa11ca9375036e07d5a8c21e02518f50728a339",
    ],
)

rpm(
    name = "expat-0__2.5.0-7.el9.x86_64",
    sha256 = "91f7f3ca1a349fefc44a35229bdb9796a6fec6b717591ee537059039b2c68024",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/expat-2.5.0-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/91f7f3ca1a349fefc44a35229bdb9796a6fec6b717591ee537059039b2c68024",
    ],
)

rpm(
    name = "expat-0__2.7.3-4.el10.aarch64",
    sha256 = "2ece9f8afb4da5aaeeab829adf68ffc30b9790c8adf7b8001342d4b74d06172a",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/expat-2.7.3-4.el10.aarch64.rpm",
    ],
)

rpm(
    name = "expat-0__2.7.3-4.el10.s390x",
    sha256 = "a59ce76f4704604dd2852bc6c7f7e0391a8c5fbc755b263bf066400fd0d97d78",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/expat-2.7.3-4.el10.s390x.rpm",
    ],
)

rpm(
    name = "expat-0__2.7.3-4.el10.x86_64",
    sha256 = "a34b186c0bafcd7c00ce9a99e51dd96a27c53399839546c85500975cfdccb51b",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/expat-2.7.3-4.el10.x86_64.rpm",
    ],
)

rpm(
    name = "filesystem-0__3.16-2.el9.aarch64",
    sha256 = "0afb1f7582830fa9c8c58a6679ab3b4ccf8bbdf1c0c76908fea1429eec8b8a53",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/filesystem-3.16-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/0afb1f7582830fa9c8c58a6679ab3b4ccf8bbdf1c0c76908fea1429eec8b8a53",
    ],
)

rpm(
    name = "filesystem-0__3.16-2.el9.x86_64",
    sha256 = "b69a472751268a1b9acd566dc7aa486fc1d6c8cb6d23f36d6a6dfead62e71475",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/filesystem-3.16-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/b69a472751268a1b9acd566dc7aa486fc1d6c8cb6d23f36d6a6dfead62e71475",
    ],
)

rpm(
    name = "filesystem-0__3.16-5.el9.aarch64",
    sha256 = "c20f1ab9760a8ba5f2d9cb37d7e8fa27f49f91a21a46fe7ad648ff6caf237013",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/filesystem-3.16-5.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c20f1ab9760a8ba5f2d9cb37d7e8fa27f49f91a21a46fe7ad648ff6caf237013",
    ],
)

rpm(
    name = "filesystem-0__3.16-5.el9.s390x",
    sha256 = "67a733fe124cda9da89f6946757800c0fe73b918a477adcf67dfbef15c995729",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/filesystem-3.16-5.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/67a733fe124cda9da89f6946757800c0fe73b918a477adcf67dfbef15c995729",
    ],
)

rpm(
    name = "filesystem-0__3.16-5.el9.x86_64",
    sha256 = "da7750fc31248ecc606016391c3f570e1abe7422f812b29a49d830c71884e6dc",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/filesystem-3.16-5.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/da7750fc31248ecc606016391c3f570e1abe7422f812b29a49d830c71884e6dc",
    ],
)

rpm(
    name = "filesystem-0__3.18-17.el10.aarch64",
    sha256 = "6c4d8ecaf8b45c8d7d588c6ebe368a77805ed84830d0bc3b38e4c8e499514aba",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/filesystem-3.18-17.el10.aarch64.rpm",
    ],
)

rpm(
    name = "filesystem-0__3.18-17.el10.s390x",
    sha256 = "087e8def18ded2dd2a96f7a4292a3654704807d05f4424c43c0f5c873d7f9cb5",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/filesystem-3.18-17.el10.s390x.rpm",
    ],
)

rpm(
    name = "filesystem-0__3.18-17.el10.x86_64",
    sha256 = "bcfb13f67c813d645f47e0a56d4bb76c0863deaf64ba93be8e0c30eecdc1e45e",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/filesystem-3.18-17.el10.x86_64.rpm",
    ],
)

rpm(
    name = "findutils-1__4.10.0-5.el10.aarch64",
    sha256 = "f0e4db5b6e713c75e097e80218c592de4e6cb85d353f0933f64714df11b178b2",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/findutils-4.10.0-5.el10.aarch64.rpm",
    ],
)

rpm(
    name = "findutils-1__4.10.0-5.el10.s390x",
    sha256 = "da20bdfeb9053ac3a1689d2ee2281298ee119175a8d486e4bb3eed1bc2857a94",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/findutils-4.10.0-5.el10.s390x.rpm",
    ],
)

rpm(
    name = "findutils-1__4.10.0-5.el10.x86_64",
    sha256 = "c646c7c108a007d62792aa66e0bc9326312089a0f8bc1c9e9300b301fd2e4276",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/findutils-4.10.0-5.el10.x86_64.rpm",
    ],
)

rpm(
    name = "findutils-1__4.8.0-7.el9.aarch64",
    sha256 = "de9914a265a46cc629f7423ef5f53deefc7044a9c46acb941d9ca0dc6bfc73f8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/findutils-4.8.0-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/de9914a265a46cc629f7423ef5f53deefc7044a9c46acb941d9ca0dc6bfc73f8",
    ],
)

rpm(
    name = "findutils-1__4.8.0-7.el9.s390x",
    sha256 = "627204a8e5a95bde190b1755dacfd72ffe66862438a6e9878d0d0fec90cf5097",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/findutils-4.8.0-7.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/627204a8e5a95bde190b1755dacfd72ffe66862438a6e9878d0d0fec90cf5097",
    ],
)

rpm(
    name = "findutils-1__4.8.0-7.el9.x86_64",
    sha256 = "393fc651dddb826521d528d78819515c09b93e551701cafb62b672c2c4701d04",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/findutils-4.8.0-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/393fc651dddb826521d528d78819515c09b93e551701cafb62b672c2c4701d04",
    ],
)

rpm(
    name = "gawk-0__5.1.0-6.el9.aarch64",
    sha256 = "656d23c583b0705eaad75cffbe880f2ec39c7d5b7a756c6a8853c2977eec331b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gawk-5.1.0-6.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/656d23c583b0705eaad75cffbe880f2ec39c7d5b7a756c6a8853c2977eec331b",
    ],
)

rpm(
    name = "gawk-0__5.1.0-6.el9.x86_64",
    sha256 = "6e6d77b76b1e89fe6f012cdc16111bea35eb4ceedac5040e5d81b5a066429af8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gawk-5.1.0-6.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/6e6d77b76b1e89fe6f012cdc16111bea35eb4ceedac5040e5d81b5a066429af8",
    ],
)

rpm(
    name = "gawk-0__5.1.0-7.el9.aarch64",
    sha256 = "fdeb8f68f67ca5ecdd080f761398df27c596510c02d2987f294444d61dd3868b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gawk-5.1.0-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/fdeb8f68f67ca5ecdd080f761398df27c596510c02d2987f294444d61dd3868b",
    ],
)

rpm(
    name = "gawk-0__5.1.0-7.el9.s390x",
    sha256 = "a3eb63ca88209b7395f3b72f6d3b16843566a08570b16b0ba030593af5ef51bf",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gawk-5.1.0-7.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/a3eb63ca88209b7395f3b72f6d3b16843566a08570b16b0ba030593af5ef51bf",
    ],
)

rpm(
    name = "gawk-0__5.1.0-7.el9.x86_64",
    sha256 = "1d3eb37bb494a30427658c55163200c5c056368cda68faf416bccbb276763ce2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gawk-5.1.0-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/1d3eb37bb494a30427658c55163200c5c056368cda68faf416bccbb276763ce2",
    ],
)

rpm(
    name = "gawk-0__5.3.0-7.el10.aarch64",
    sha256 = "d40ca9237f1c138edebf15b8b0674ec5871b0622d4ff04e12e1417d5ba593c63",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/gawk-5.3.0-7.el10.aarch64.rpm",
    ],
)

rpm(
    name = "gawk-0__5.3.0-7.el10.s390x",
    sha256 = "47136d2056847be001d5b99acd623ad2598f5d607163c84634fb1677f0b7cdf3",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/gawk-5.3.0-7.el10.s390x.rpm",
    ],
)

rpm(
    name = "gawk-0__5.3.0-7.el10.x86_64",
    sha256 = "f58ace50ed0504823a8d42ec9a848068a6c45ed8b760d49dc25fbf79332343a1",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/gawk-5.3.0-7.el10.x86_64.rpm",
    ],
)

rpm(
    name = "gdbm-libs-1__1.23-1.el9.aarch64",
    sha256 = "69754627d810b252c6202f2ef8765ca39b9c8a0b0fd6da0325a9e492dbf88f96",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gdbm-libs-1.23-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/69754627d810b252c6202f2ef8765ca39b9c8a0b0fd6da0325a9e492dbf88f96",
    ],
)

rpm(
    name = "gdbm-libs-1__1.23-1.el9.s390x",
    sha256 = "29c9ab72536be72b9c78285ef12117633cf3e2dfd18757bcf7587cd94eb9e055",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gdbm-libs-1.23-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/29c9ab72536be72b9c78285ef12117633cf3e2dfd18757bcf7587cd94eb9e055",
    ],
)

rpm(
    name = "gdbm-libs-1__1.23-1.el9.x86_64",
    sha256 = "cada66331cc07a4f8a0701fc1ad13c346913a0d6f913e35c0257a68b6a1e6ce0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gdbm-libs-1.23-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/cada66331cc07a4f8a0701fc1ad13c346913a0d6f913e35c0257a68b6a1e6ce0",
    ],
)

rpm(
    name = "gdbm-libs-1__1.26-15.el10.aarch64",
    sha256 = "29a10774869b5e9d61e9b518c7e9d56df7916024a264c858c2f3baeb876a7274",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/gdbm-libs-1.26-15.el10.aarch64.rpm",
    ],
)

rpm(
    name = "gdbm-libs-1__1.26-15.el10.s390x",
    sha256 = "2bafc5ee3afe597e12fc5f2fc3f7fc6da223605e8fbd84534847ebdbe233768f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/gdbm-libs-1.26-15.el10.s390x.rpm",
    ],
)

rpm(
    name = "gdbm-libs-1__1.26-15.el10.x86_64",
    sha256 = "2f2b3b795509d3c43c9623a0e388ea0bc402e41ab949a7e4f731734cde2ac327",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/gdbm-libs-1.26-15.el10.x86_64.rpm",
    ],
)

rpm(
    name = "glib2-0__2.68.4-29.el9.aarch64",
    sha256 = "9a88b2b2c7de9db0045e863c7e9f2e293be367fbfba5ef5fd270da75de367a89",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glib2-2.68.4-29.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/9a88b2b2c7de9db0045e863c7e9f2e293be367fbfba5ef5fd270da75de367a89",
    ],
)

rpm(
    name = "glib2-0__2.68.4-29.el9.s390x",
    sha256 = "9df54ea33807398d6e89f2ee8952ab718e95fa450a822396a1c4bd352feff105",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glib2-2.68.4-29.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/9df54ea33807398d6e89f2ee8952ab718e95fa450a822396a1c4bd352feff105",
    ],
)

rpm(
    name = "glib2-0__2.68.4-29.el9.x86_64",
    sha256 = "5f1f234fae95dbc35d383b0c37d0d249da5d9aaed57e92915da50edeb5ac22a5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glib2-2.68.4-29.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/5f1f234fae95dbc35d383b0c37d0d249da5d9aaed57e92915da50edeb5ac22a5",
    ],
)

rpm(
    name = "glib2-0__2.68.4-5.el9.aarch64",
    sha256 = "fa9e25b82015b5d2023d9f71582e2dc0ed13ce7fc70c29ee49797713a88b46db",
    urls = ["https://storage.googleapis.com/builddeps/fa9e25b82015b5d2023d9f71582e2dc0ed13ce7fc70c29ee49797713a88b46db"],
)

rpm(
    name = "glib2-0__2.68.4-5.el9.x86_64",
    sha256 = "34bc8c6f001daa8dba60aee15956d7ac124e71bd7c5c99039245a4bf6e61a8f5",
    urls = ["https://storage.googleapis.com/builddeps/34bc8c6f001daa8dba60aee15956d7ac124e71bd7c5c99039245a4bf6e61a8f5"],
)

rpm(
    name = "glib2-0__2.80.4-22.el10.aarch64",
    sha256 = "29ddd01832c35270cf47a9e6f99f1050c2389592088b26e6fa45ff1535704240",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/glib2-2.80.4-22.el10.aarch64.rpm",
    ],
)

rpm(
    name = "glib2-0__2.80.4-22.el10.s390x",
    sha256 = "1da22758283017a4a0b82a24614f6b9601027e49cd69d12cf2ea1fe964328314",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/glib2-2.80.4-22.el10.s390x.rpm",
    ],
)

rpm(
    name = "glib2-0__2.80.4-22.el10.x86_64",
    sha256 = "93173b7a11fac9ae6b0106374f67aabf243b4b7125e7b58ae57a0c5c4a2d42d5",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/glib2-2.80.4-22.el10.x86_64.rpm",
    ],
)

rpm(
    name = "glibc-0__2.34-277.el9.aarch64",
    sha256 = "6614c1e4fd142b8edfc3a2f115b9a8c8c2e8fb1dc31c37be2610c62258f055d9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-2.34-277.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/6614c1e4fd142b8edfc3a2f115b9a8c8c2e8fb1dc31c37be2610c62258f055d9",
    ],
)

rpm(
    name = "glibc-0__2.34-277.el9.s390x",
    sha256 = "2c3e0517c5591292e2a1dc46f768791bbe37fc8e0d1fba1e06b0baa15b2db7f6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glibc-2.34-277.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/2c3e0517c5591292e2a1dc46f768791bbe37fc8e0d1fba1e06b0baa15b2db7f6",
    ],
)

rpm(
    name = "glibc-0__2.34-277.el9.x86_64",
    sha256 = "c787e1da27f37917d23e2028ca87d87f9ed9ee78f5a43d7d417f7d45a2b19c26",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-2.34-277.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c787e1da27f37917d23e2028ca87d87f9ed9ee78f5a43d7d417f7d45a2b19c26",
    ],
)

rpm(
    name = "glibc-0__2.34-29.el9.aarch64",
    sha256 = "6c8ec68d34d1abc0c8438ef1db2e77f5decee74869a1116766ed44c03690a234",
    urls = ["https://storage.googleapis.com/builddeps/6c8ec68d34d1abc0c8438ef1db2e77f5decee74869a1116766ed44c03690a234"],
)

rpm(
    name = "glibc-0__2.34-29.el9.x86_64",
    sha256 = "900ac0b0ffe6dec1167f3b67335b811c9d95a2f50885b980950f4b527c500b67",
    urls = ["https://storage.googleapis.com/builddeps/900ac0b0ffe6dec1167f3b67335b811c9d95a2f50885b980950f4b527c500b67"],
)

rpm(
    name = "glibc-0__2.39-148.el10.aarch64",
    sha256 = "51b6ec73f5affbea82cb248af56aebff2a1994f906cccc4346e43eae416c9bbe",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/glibc-2.39-148.el10.aarch64.rpm",
    ],
)

rpm(
    name = "glibc-0__2.39-148.el10.s390x",
    sha256 = "dea6c14ff604d37ad0a1045a1a86c26a4503a0b33792fcaea8170cb8e8f50ebb",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/glibc-2.39-148.el10.s390x.rpm",
    ],
)

rpm(
    name = "glibc-0__2.39-148.el10.x86_64",
    sha256 = "6ddbb36f27e948b7092cd443e08bec75e4a7498d6f6f02203f223fe92c3226a5",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/glibc-2.39-148.el10.x86_64.rpm",
    ],
)

rpm(
    name = "glibc-common-0__2.34-277.el9.aarch64",
    sha256 = "cdb3e54a992e8708b6b407211f2c410c8644d03f26e509ca149a73f57cfaf9b4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-common-2.34-277.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/cdb3e54a992e8708b6b407211f2c410c8644d03f26e509ca149a73f57cfaf9b4",
    ],
)

rpm(
    name = "glibc-common-0__2.34-277.el9.s390x",
    sha256 = "56795eb955440904f0e6256f4bdb74b2fc7b13b67fd040691fa9e4b3f74cc98d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glibc-common-2.34-277.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/56795eb955440904f0e6256f4bdb74b2fc7b13b67fd040691fa9e4b3f74cc98d",
    ],
)

rpm(
    name = "glibc-common-0__2.34-277.el9.x86_64",
    sha256 = "1216f2f12e687ed00b4930bdc7d7f08d0a5e6acc0f29cb827fda335a44230314",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-common-2.34-277.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/1216f2f12e687ed00b4930bdc7d7f08d0a5e6acc0f29cb827fda335a44230314",
    ],
)

rpm(
    name = "glibc-common-0__2.34-29.el9.aarch64",
    sha256 = "e34e7a2e2767ff4d68866064f980600f2fedaeb5232aec960de45a02b37f8406",
    urls = ["https://storage.googleapis.com/builddeps/e34e7a2e2767ff4d68866064f980600f2fedaeb5232aec960de45a02b37f8406"],
)

rpm(
    name = "glibc-common-0__2.34-29.el9.x86_64",
    sha256 = "da8be2ae89b50cf060786b8338430f6260c69f3afda1afea43ba99cb9c6f5b3a",
    urls = ["https://storage.googleapis.com/builddeps/da8be2ae89b50cf060786b8338430f6260c69f3afda1afea43ba99cb9c6f5b3a"],
)

rpm(
    name = "glibc-common-0__2.39-148.el10.aarch64",
    sha256 = "b6c748fc1ab8ed538fc10c27ec165be3bc4efd30301cce66b13aefb579525a8c",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/glibc-common-2.39-148.el10.aarch64.rpm",
    ],
)

rpm(
    name = "glibc-common-0__2.39-148.el10.s390x",
    sha256 = "b287859bea74ad9b2b475219a7fcb3719a4618606acd02fde0d660ac22f9765c",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/glibc-common-2.39-148.el10.s390x.rpm",
    ],
)

rpm(
    name = "glibc-common-0__2.39-148.el10.x86_64",
    sha256 = "68f1a2b3fae9b4beae2a6bb7dc2100ac9206d33a48146312b7a88dcd6cb01daa",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/glibc-common-2.39-148.el10.x86_64.rpm",
    ],
)

rpm(
    name = "glibc-minimal-langpack-0__2.34-277.el9.aarch64",
    sha256 = "8c068eb8b0924084eb7e8ae5cf0bd19fbec8afda9a9741bb7a8103254152bd52",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-minimal-langpack-2.34-277.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/8c068eb8b0924084eb7e8ae5cf0bd19fbec8afda9a9741bb7a8103254152bd52",
    ],
)

rpm(
    name = "glibc-minimal-langpack-0__2.34-277.el9.s390x",
    sha256 = "a214ad7e2df501045e10e1d09d4558f873cb90dd1968b06baaadc86c70e96b47",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glibc-minimal-langpack-2.34-277.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/a214ad7e2df501045e10e1d09d4558f873cb90dd1968b06baaadc86c70e96b47",
    ],
)

rpm(
    name = "glibc-minimal-langpack-0__2.34-277.el9.x86_64",
    sha256 = "b6995d3e05bfdd2b131a4de91a3887f1ec861422d57ab1dd8734be6fa71a43e5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-minimal-langpack-2.34-277.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/b6995d3e05bfdd2b131a4de91a3887f1ec861422d57ab1dd8734be6fa71a43e5",
    ],
)

rpm(
    name = "glibc-minimal-langpack-0__2.34-29.el9.aarch64",
    sha256 = "b5958ea033b10b6c571f81a8b8c9f7f1619c72c2f4e910f677df625df32170d6",
    urls = ["https://storage.googleapis.com/builddeps/b5958ea033b10b6c571f81a8b8c9f7f1619c72c2f4e910f677df625df32170d6"],
)

rpm(
    name = "glibc-minimal-langpack-0__2.34-29.el9.x86_64",
    sha256 = "5ffe9c07ee24f50d6c94a574ca5e89fffe336a7ee004ba362e8ebaff62f47186",
    urls = ["https://storage.googleapis.com/builddeps/5ffe9c07ee24f50d6c94a574ca5e89fffe336a7ee004ba362e8ebaff62f47186"],
)

rpm(
    name = "glibc-minimal-langpack-0__2.39-148.el10.aarch64",
    sha256 = "97488d0f468f5471215188dccebb04579663eabf7b9a2fd3d30c385f4832af1b",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/glibc-minimal-langpack-2.39-148.el10.aarch64.rpm",
    ],
)

rpm(
    name = "glibc-minimal-langpack-0__2.39-148.el10.s390x",
    sha256 = "19ba1c46a0bcbfa01c3d6feae24a2328fd2122ab3a04f2a18cb7e49bc86ed86c",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/glibc-minimal-langpack-2.39-148.el10.s390x.rpm",
    ],
)

rpm(
    name = "glibc-minimal-langpack-0__2.39-148.el10.x86_64",
    sha256 = "5fd3ccf55943c5504b75202adc191c82ac21beec83c37d413c573d421f663adf",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/glibc-minimal-langpack-2.39-148.el10.x86_64.rpm",
    ],
)

rpm(
    name = "gmp-1__6.2.0-10.el9.aarch64",
    sha256 = "1fe837ca20f20f8291a32c0f4673ea2560f94d75d25ab5131f6ae271694a4b44",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gmp-6.2.0-10.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/1fe837ca20f20f8291a32c0f4673ea2560f94d75d25ab5131f6ae271694a4b44",
    ],
)

rpm(
    name = "gmp-1__6.2.0-10.el9.x86_64",
    sha256 = "1a6ededc80029ef258288ddbf24bcce7c6228647841416950c88e3f14b7258a2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gmp-6.2.0-10.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/1a6ededc80029ef258288ddbf24bcce7c6228647841416950c88e3f14b7258a2",
    ],
)

rpm(
    name = "gmp-1__6.2.0-13.el9.aarch64",
    sha256 = "01716c2de2af5ddce80cfc2f81fbcabe50670583f8d3ebf8af1058982edb9c70",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gmp-6.2.0-13.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/01716c2de2af5ddce80cfc2f81fbcabe50670583f8d3ebf8af1058982edb9c70",
    ],
)

rpm(
    name = "gmp-1__6.2.0-13.el9.s390x",
    sha256 = "c26b4f2d1e2c6a9a3b683d1909df8f788a261fcc8e766ded00a96681e5dc62d2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gmp-6.2.0-13.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/c26b4f2d1e2c6a9a3b683d1909df8f788a261fcc8e766ded00a96681e5dc62d2",
    ],
)

rpm(
    name = "gmp-1__6.2.0-13.el9.x86_64",
    sha256 = "b6d592895ccc0fcad6106cd41800cd9d68e5384c418e53a2c3ff2ac8c8b15a33",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gmp-6.2.0-13.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/b6d592895ccc0fcad6106cd41800cd9d68e5384c418e53a2c3ff2ac8c8b15a33",
    ],
)

rpm(
    name = "gmp-1__6.2.1-12.el10.aarch64",
    sha256 = "9bbe58df2a29320daf9b4c36305fcc7f781ab0bdd486736c6d8c685838141a41",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/gmp-6.2.1-12.el10.aarch64.rpm",
    ],
)

rpm(
    name = "gmp-1__6.2.1-12.el10.s390x",
    sha256 = "54d437788539933aa6de0963c6b1303e50b07f17db9ea847a71e19d1b4ef6a66",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/gmp-6.2.1-12.el10.s390x.rpm",
    ],
)

rpm(
    name = "gmp-1__6.2.1-12.el10.x86_64",
    sha256 = "6678824b5d45f9b66e8bfeb8f32736e0d710e3b38531a85548f55702d96b63a8",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/gmp-6.2.1-12.el10.x86_64.rpm",
    ],
)

rpm(
    name = "gnupg2-0__2.3.3-5.el9.aarch64",
    sha256 = "5fd008231e14128555e5eb997ae57e3f82fc8ac28b0bfb2b9e961f8a8bdf9937",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gnupg2-2.3.3-5.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/5fd008231e14128555e5eb997ae57e3f82fc8ac28b0bfb2b9e961f8a8bdf9937",
    ],
)

rpm(
    name = "gnupg2-0__2.3.3-5.el9.s390x",
    sha256 = "9cbb342b46df96e85e55919bee459b2fd5023642494eeb2466344b765c1802d3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gnupg2-2.3.3-5.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/9cbb342b46df96e85e55919bee459b2fd5023642494eeb2466344b765c1802d3",
    ],
)

rpm(
    name = "gnupg2-0__2.3.3-5.el9.x86_64",
    sha256 = "5628444d9a62a7b6b46951c5187ccf43cb4d9254a45ae225808c6ef7d28c027f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gnupg2-2.3.3-5.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/5628444d9a62a7b6b46951c5187ccf43cb4d9254a45ae225808c6ef7d28c027f",
    ],
)

rpm(
    name = "gnupg2-0__2.4.5-5.el10.aarch64",
    sha256 = "ad7c22047b5dc00bb679bbfad6b4d6415fcd4d816c0726d7cb96c3632cb941b0",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/gnupg2-2.4.5-5.el10.aarch64.rpm",
    ],
)

rpm(
    name = "gnupg2-0__2.4.5-5.el10.s390x",
    sha256 = "c3d06bb7f237857b892d2563f8b98c7e9ee1464c79815b68f024ee4d954cb9c2",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/gnupg2-2.4.5-5.el10.s390x.rpm",
    ],
)

rpm(
    name = "gnupg2-0__2.4.5-5.el10.x86_64",
    sha256 = "b91831f705df214ca8fb10a5d9d649e13453445a6e7b59758ff646f6b1dbc72c",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/gnupg2-2.4.5-5.el10.x86_64.rpm",
    ],
)

rpm(
    name = "gnutls-0__3.7.3-9.el9.aarch64",
    sha256 = "0f608bc35b5ec94c3b2512731089d7c8ab416499aa9840093a0ee41b6418f29c",
    urls = ["https://storage.googleapis.com/builddeps/0f608bc35b5ec94c3b2512731089d7c8ab416499aa9840093a0ee41b6418f29c"],
)

rpm(
    name = "gnutls-0__3.7.3-9.el9.x86_64",
    sha256 = "f6781dc8504214040301843ccd95e2e43351208092d5c01587463d3065efc4b3",
    urls = ["https://storage.googleapis.com/builddeps/f6781dc8504214040301843ccd95e2e43351208092d5c01587463d3065efc4b3"],
)

rpm(
    name = "gnutls-0__3.8.10-7.el10.aarch64",
    sha256 = "8f3a58f346cdff38c1a1da05cbc704835da1ba9878dc137ed6014919cc925607",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/gnutls-3.8.10-7.el10.aarch64.rpm",
    ],
)

rpm(
    name = "gnutls-0__3.8.10-7.el10.s390x",
    sha256 = "20bb9d47c655506278917057a117b53c2d44aed5a7aab3177022a7f93972a119",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/gnutls-3.8.10-7.el10.s390x.rpm",
    ],
)

rpm(
    name = "gnutls-0__3.8.10-7.el10.x86_64",
    sha256 = "7cb047e3132f5cdce9c534eab74dabab4e657f527f193fc723dc47aa66ad5d77",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/gnutls-3.8.10-7.el10.x86_64.rpm",
    ],
)

rpm(
    name = "gnutls-0__3.8.10-8.el9.aarch64",
    sha256 = "530f18b1cdf0a56ff8ce81d5d9e1617f026224f39a9f494df73cd0b88e6f7774",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gnutls-3.8.10-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/530f18b1cdf0a56ff8ce81d5d9e1617f026224f39a9f494df73cd0b88e6f7774",
    ],
)

rpm(
    name = "gnutls-0__3.8.10-8.el9.s390x",
    sha256 = "0d3a7b77a92bc9da0594366818e6bb70db3e6e01a7491565dff1143dbf3d9145",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gnutls-3.8.10-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/0d3a7b77a92bc9da0594366818e6bb70db3e6e01a7491565dff1143dbf3d9145",
    ],
)

rpm(
    name = "gnutls-0__3.8.10-8.el9.x86_64",
    sha256 = "7995375d84e6592cdba3d94ba01e47f5607aecb2ff73b7af67a756def78e8a31",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gnutls-3.8.10-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/7995375d84e6592cdba3d94ba01e47f5607aecb2ff73b7af67a756def78e8a31",
    ],
)

rpm(
    name = "gpgme-0__1.15.1-6.el9.aarch64",
    sha256 = "590f495d6b2176f692038dae2a8a80b6edcc9294574f9ba16cb0713829b137a2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gpgme-1.15.1-6.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/590f495d6b2176f692038dae2a8a80b6edcc9294574f9ba16cb0713829b137a2",
    ],
)

rpm(
    name = "gpgme-0__1.15.1-6.el9.s390x",
    sha256 = "76e6cd72d0203e559e10c1e8f62f2eee4d53e7be767108cf973cb260fab2b3a1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gpgme-1.15.1-6.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/76e6cd72d0203e559e10c1e8f62f2eee4d53e7be767108cf973cb260fab2b3a1",
    ],
)

rpm(
    name = "gpgme-0__1.15.1-6.el9.x86_64",
    sha256 = "c5afb08432a50112929dafd7430e6af28fbad3273a6ba81571ed1dbf37d83cf7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gpgme-1.15.1-6.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c5afb08432a50112929dafd7430e6af28fbad3273a6ba81571ed1dbf37d83cf7",
    ],
)

rpm(
    name = "gpgme-0__1.23.2-6.el10.aarch64",
    sha256 = "2f18ca6c6d7825be2530598360e98027acf6ed39de1d0bdec131a44e7b50e2b2",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/gpgme-1.23.2-6.el10.aarch64.rpm",
    ],
)

rpm(
    name = "gpgme-0__1.23.2-6.el10.s390x",
    sha256 = "3274cc8f9c6b1b0c61cb3424cfa1b7da403518625f9d01df791eb77ab70c38f8",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/gpgme-1.23.2-6.el10.s390x.rpm",
    ],
)

rpm(
    name = "gpgme-0__1.23.2-6.el10.x86_64",
    sha256 = "e7633adeb20a8fca833b363e16dd0fe01fd4729e43fa0beacde751e79d112d05",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/gpgme-1.23.2-6.el10.x86_64.rpm",
    ],
)

rpm(
    name = "grep-0__3.11-10.el10.aarch64",
    sha256 = "d797740f7c738e5e7729949bde3d82274c5c6422242a82c1058fbe71ea0c37e9",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/grep-3.11-10.el10.aarch64.rpm",
    ],
)

rpm(
    name = "grep-0__3.11-10.el10.s390x",
    sha256 = "d30a1ab1991131978b67f26d6c119f97bb5408a4bebac0294f2ac5417fe12276",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/grep-3.11-10.el10.s390x.rpm",
    ],
)

rpm(
    name = "grep-0__3.11-10.el10.x86_64",
    sha256 = "a0eb701c640cd0a0c9195493a8fc9206fff62174d958ba4af2d92527191f803f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/grep-3.11-10.el10.x86_64.rpm",
    ],
)

rpm(
    name = "grep-0__3.6-5.el9.aarch64",
    sha256 = "33bdf571a62cb8b7d659617e9278e46043aa936f8e963202750d19463a805f60",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/grep-3.6-5.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/33bdf571a62cb8b7d659617e9278e46043aa936f8e963202750d19463a805f60",
    ],
)

rpm(
    name = "grep-0__3.6-5.el9.s390x",
    sha256 = "b6b83738fc6afb9ba28d0c2c57eaf17cdbe5b26ff89a8da17812dd261045df3e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/grep-3.6-5.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/b6b83738fc6afb9ba28d0c2c57eaf17cdbe5b26ff89a8da17812dd261045df3e",
    ],
)

rpm(
    name = "grep-0__3.6-5.el9.x86_64",
    sha256 = "10a41b66b1fbd6eb055178e22c37199e5b49b4852e77c806f7af7211044a4a55",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/grep-3.6-5.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/10a41b66b1fbd6eb055178e22c37199e5b49b4852e77c806f7af7211044a4a55",
    ],
)

rpm(
    name = "gzip-0__1.12-2.el9.aarch64",
    sha256 = "81572274cc28e7ea447d0768384f2540bbbbb709f13ceb829014e85f612655ca",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gzip-1.12-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/81572274cc28e7ea447d0768384f2540bbbbb709f13ceb829014e85f612655ca",
    ],
)

rpm(
    name = "gzip-0__1.12-2.el9.s390x",
    sha256 = "c61e628ecdf982657ba6897d171b3e090d0b719f5bb3b51a94bbc6854907eaae",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gzip-1.12-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/c61e628ecdf982657ba6897d171b3e090d0b719f5bb3b51a94bbc6854907eaae",
    ],
)

rpm(
    name = "gzip-0__1.12-2.el9.x86_64",
    sha256 = "2ee6442b3fda80cf356d62d8537f30a189196faa1d2a8325ae6c1e303608ce00",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gzip-1.12-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/2ee6442b3fda80cf356d62d8537f30a189196faa1d2a8325ae6c1e303608ce00",
    ],
)

rpm(
    name = "gzip-0__1.13-4.el10.aarch64",
    sha256 = "ed08d2a13297935666032b580e7e1fba7e615af1db3373e91b077cd7493ccffc",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/gzip-1.13-4.el10.aarch64.rpm",
    ],
)

rpm(
    name = "gzip-0__1.13-4.el10.s390x",
    sha256 = "b2d5747f8b483bb680184de57bd2dd9e756f492eb03d68ea919b6197afdbf870",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/gzip-1.13-4.el10.s390x.rpm",
    ],
)

rpm(
    name = "gzip-0__1.13-4.el10.x86_64",
    sha256 = "c0fc5ead1e9f597dfb0fe5400d82c19eab62c72f70007fce03961ca687ab9e9c",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/gzip-1.13-4.el10.x86_64.rpm",
    ],
)

rpm(
    name = "iptables-libs-0__1.8.10-11.el9.aarch64",
    sha256 = "097df125f6836f5dbdce2f3e961a649cd2e15b5f2a8164267c7c98b281ab60e4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/iptables-libs-1.8.10-11.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/097df125f6836f5dbdce2f3e961a649cd2e15b5f2a8164267c7c98b281ab60e4",
    ],
)

rpm(
    name = "iptables-libs-0__1.8.10-11.el9.s390x",
    sha256 = "469bd3ae07fb31f648a81d8ffa6b5053ee647b4c5dffcbcfbf11081921231715",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/iptables-libs-1.8.10-11.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/469bd3ae07fb31f648a81d8ffa6b5053ee647b4c5dffcbcfbf11081921231715",
    ],
)

rpm(
    name = "iptables-libs-0__1.8.10-11.el9.x86_64",
    sha256 = "7ffd51ff29c86e31d36ff9518dead9fd403034824e874b069a24c6587d4e1084",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/iptables-libs-1.8.10-11.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/7ffd51ff29c86e31d36ff9518dead9fd403034824e874b069a24c6587d4e1084",
    ],
)

rpm(
    name = "iptables-libs-0__1.8.13-1.el10.aarch64",
    sha256 = "b1e17feff83e9d4ee9d7b7dd4973ee406162097270200f8e8cc3cdd9ee384437",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/iptables-libs-1.8.13-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "iptables-libs-0__1.8.13-1.el10.s390x",
    sha256 = "0884316298b93df0f9ad9339ae48de282c1acda7465ea56fa217d0f4d7005fe5",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/iptables-libs-1.8.13-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "iptables-libs-0__1.8.13-1.el10.x86_64",
    sha256 = "96c523fc020379a815452824a22955b168e096e0a7f5de378c57afdf5e2d2091",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/iptables-libs-1.8.13-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "iptables-nft-0__1.8.10-11.el9.aarch64",
    sha256 = "f6a8ddd687f1af180d4a7a24557b209952b393e279ba36443d4a5daeb7cd11aa",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/iptables-nft-1.8.10-11.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f6a8ddd687f1af180d4a7a24557b209952b393e279ba36443d4a5daeb7cd11aa",
    ],
)

rpm(
    name = "iptables-nft-0__1.8.10-11.el9.s390x",
    sha256 = "25b42aa1f8d225271d4e21e6e35c494454f6a09663ac8ecc29c9b5b0c00c6742",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/iptables-nft-1.8.10-11.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/25b42aa1f8d225271d4e21e6e35c494454f6a09663ac8ecc29c9b5b0c00c6742",
    ],
)

rpm(
    name = "iptables-nft-0__1.8.10-11.el9.x86_64",
    sha256 = "e87505a08fc8cf99f8de8e235ab3bc339048815e9550b45557a659aeb76789ac",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/iptables-nft-1.8.10-11.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e87505a08fc8cf99f8de8e235ab3bc339048815e9550b45557a659aeb76789ac",
    ],
)

rpm(
    name = "jansson-0__2.14-1.el9.aarch64",
    sha256 = "23a8033dae909a6b87db199e04ecbc9798820b1b939e12d51733fed4554b9279",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/jansson-2.14-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/23a8033dae909a6b87db199e04ecbc9798820b1b939e12d51733fed4554b9279",
    ],
)

rpm(
    name = "jansson-0__2.14-1.el9.s390x",
    sha256 = "ec1863fd2bd9672ecb0bd4f77d929dad04f253330a41307300f485ae13d017e5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/jansson-2.14-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/ec1863fd2bd9672ecb0bd4f77d929dad04f253330a41307300f485ae13d017e5",
    ],
)

rpm(
    name = "jansson-0__2.14-1.el9.x86_64",
    sha256 = "c3fb9f8020f978f9b392709996e62e4ddb6cb19074635af3338487195b688f66",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/jansson-2.14-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c3fb9f8020f978f9b392709996e62e4ddb6cb19074635af3338487195b688f66",
    ],
)

rpm(
    name = "jansson-0__2.14-3.el10.aarch64",
    sha256 = "a838d217420f9f10eb80a221b6cda50ff65e729c15be94f33cbb420f206ee880",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/jansson-2.14-3.el10.aarch64.rpm",
    ],
)

rpm(
    name = "jansson-0__2.14-3.el10.s390x",
    sha256 = "cc054f4efd4b779ec708061759de28acd9eb9df0ad8f3b32f9fe4752b1dcb06c",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/jansson-2.14-3.el10.s390x.rpm",
    ],
)

rpm(
    name = "jansson-0__2.14-3.el10.x86_64",
    sha256 = "25d2ef852d5941b27ae105ec780aa367605a6f8b86e6c6a13abdee1c1065979f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/jansson-2.14-3.el10.x86_64.rpm",
    ],
)

rpm(
    name = "json-c-0__0.14-11.el9.aarch64",
    sha256 = "65a68a23f33540b4d7cd2d9227a63d7eda1a7ab7cdd52457fee9662c06731cfa",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/json-c-0.14-11.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/65a68a23f33540b4d7cd2d9227a63d7eda1a7ab7cdd52457fee9662c06731cfa",
    ],
)

rpm(
    name = "json-c-0__0.14-11.el9.s390x",
    sha256 = "224d820ba796088e5742a550fe7add8accf6bae309f154b4589bc11628edbcc4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/json-c-0.14-11.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/224d820ba796088e5742a550fe7add8accf6bae309f154b4589bc11628edbcc4",
    ],
)

rpm(
    name = "json-c-0__0.14-11.el9.x86_64",
    sha256 = "1a75404c6bc8c1369914077dc99480e73bf13a40f15fd1cd8afc792b8600adf8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/json-c-0.14-11.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/1a75404c6bc8c1369914077dc99480e73bf13a40f15fd1cd8afc792b8600adf8",
    ],
)

rpm(
    name = "json-c-0__0.18-3.el10.aarch64",
    sha256 = "d3ecfebff7515c94e971c9584b0815202712cc2642526ee4fe5e424ec8ff2fae",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/json-c-0.18-3.el10.aarch64.rpm",
    ],
)

rpm(
    name = "json-c-0__0.18-3.el10.s390x",
    sha256 = "d4bc7597af6496e70ffa04858c8d2418267b302959db9da3f89c6edfc723ccd4",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/json-c-0.18-3.el10.s390x.rpm",
    ],
)

rpm(
    name = "json-c-0__0.18-3.el10.x86_64",
    sha256 = "e73ae01d509fb9bef1bbd675be1c0003b0ee942a4187e9b14ef43e56e508245b",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/json-c-0.18-3.el10.x86_64.rpm",
    ],
)

rpm(
    name = "keyutils-libs-0__1.6.1-4.el9.aarch64",
    sha256 = "bb0cc6cde590e58d76610c5d0d0811f20603758f63a604f10289a170bcde4e0f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/keyutils-libs-1.6.1-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/bb0cc6cde590e58d76610c5d0d0811f20603758f63a604f10289a170bcde4e0f",
    ],
)

rpm(
    name = "keyutils-libs-0__1.6.1-4.el9.x86_64",
    sha256 = "56c94b7b30b5e5b1411b0053fd62edf408d59fc2260d7d31883a97a667342d6f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/keyutils-libs-1.6.1-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/56c94b7b30b5e5b1411b0053fd62edf408d59fc2260d7d31883a97a667342d6f",
    ],
)

rpm(
    name = "keyutils-libs-0__1.6.3-1.el9.aarch64",
    sha256 = "5d97ee3ed28533eb2ea01a6be97696fbbbc72f8178dcf7f1acf30e674a298a6e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/keyutils-libs-1.6.3-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/5d97ee3ed28533eb2ea01a6be97696fbbbc72f8178dcf7f1acf30e674a298a6e",
    ],
)

rpm(
    name = "keyutils-libs-0__1.6.3-1.el9.s390x",
    sha256 = "954b22cc636f29363edc7a29c24cb05039929ca71780174b8ec4dc495af314ef",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/keyutils-libs-1.6.3-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/954b22cc636f29363edc7a29c24cb05039929ca71780174b8ec4dc495af314ef",
    ],
)

rpm(
    name = "keyutils-libs-0__1.6.3-1.el9.x86_64",
    sha256 = "aef982501694486a27411c68698886d76ec70c5cd10bfe619501e7e4c36f50a9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/keyutils-libs-1.6.3-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/aef982501694486a27411c68698886d76ec70c5cd10bfe619501e7e4c36f50a9",
    ],
)

rpm(
    name = "keyutils-libs-0__1.6.3-5.el10.aarch64",
    sha256 = "a6ff394736256d5c2317ab5503a056d0f60155a92090853179506358bfd2333f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/keyutils-libs-1.6.3-5.el10.aarch64.rpm",
    ],
)

rpm(
    name = "keyutils-libs-0__1.6.3-5.el10.s390x",
    sha256 = "f2ed690c8ec6ef2a0b912ba324268d354d282f437b30044a44304543e78d9238",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/keyutils-libs-1.6.3-5.el10.s390x.rpm",
    ],
)

rpm(
    name = "keyutils-libs-0__1.6.3-5.el10.x86_64",
    sha256 = "312e0bf42841bb330f7721012d1ee5816e5ea223e54fc5dfd1a95c6f1d7516b0",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/keyutils-libs-1.6.3-5.el10.x86_64.rpm",
    ],
)

rpm(
    name = "kmod-libs-0__28-11.el9.aarch64",
    sha256 = "68bd119a65b2d37388623c0e4a0a717b74787e1243244c8ffa0a448f42718ee4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/kmod-libs-28-11.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/68bd119a65b2d37388623c0e4a0a717b74787e1243244c8ffa0a448f42718ee4",
    ],
)

rpm(
    name = "kmod-libs-0__28-11.el9.s390x",
    sha256 = "e04b90f099224b2cb1dd28df4ff45aaa1982d26b2e2f04cb7bdcdf9b5a1306c4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/kmod-libs-28-11.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/e04b90f099224b2cb1dd28df4ff45aaa1982d26b2e2f04cb7bdcdf9b5a1306c4",
    ],
)

rpm(
    name = "kmod-libs-0__28-11.el9.x86_64",
    sha256 = "29d2fd267498f3e12d420a3d867483d32ce97d544327de983872f8ee89ec02b3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/kmod-libs-28-11.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/29d2fd267498f3e12d420a3d867483d32ce97d544327de983872f8ee89ec02b3",
    ],
)

rpm(
    name = "krb5-libs-0__1.19.1-15.el9.aarch64",
    sha256 = "a8fdd4663601dc6713469d8c03daa9e77bcb32e2d82bc139e02797236005bb84",
    urls = ["https://storage.googleapis.com/builddeps/a8fdd4663601dc6713469d8c03daa9e77bcb32e2d82bc139e02797236005bb84"],
)

rpm(
    name = "krb5-libs-0__1.19.1-15.el9.x86_64",
    sha256 = "d474a74d1902ee733799e50519bca7cc430e67f15fdc91a264a0d34e87ebc5a5",
    urls = ["https://storage.googleapis.com/builddeps/d474a74d1902ee733799e50519bca7cc430e67f15fdc91a264a0d34e87ebc5a5"],
)

rpm(
    name = "krb5-libs-0__1.21.1-10.el9.aarch64",
    sha256 = "02c094878ceb99014307c07aee6a95422d67b856571ee1f2c65b67f556b0a008",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/krb5-libs-1.21.1-10.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/02c094878ceb99014307c07aee6a95422d67b856571ee1f2c65b67f556b0a008",
    ],
)

rpm(
    name = "krb5-libs-0__1.21.1-10.el9.s390x",
    sha256 = "7f79794f0adc0b7f0ede5dd6d8536068c7f8de948d947e42ce1cdafeb96fe8e3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/krb5-libs-1.21.1-10.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/7f79794f0adc0b7f0ede5dd6d8536068c7f8de948d947e42ce1cdafeb96fe8e3",
    ],
)

rpm(
    name = "krb5-libs-0__1.21.1-10.el9.x86_64",
    sha256 = "55f585ca5ceb611bcd44ce845179769fa42a2316fe23b83b1e13947fd54b7e0d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/krb5-libs-1.21.1-10.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/55f585ca5ceb611bcd44ce845179769fa42a2316fe23b83b1e13947fd54b7e0d",
    ],
)

rpm(
    name = "krb5-libs-0__1.21.3-11.el10.aarch64",
    sha256 = "a41b1ba1b82b994022ca5d6426e44aac92db1ec36123ddbfc8740ef58c9191ce",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/krb5-libs-1.21.3-11.el10.aarch64.rpm",
    ],
)

rpm(
    name = "krb5-libs-0__1.21.3-11.el10.s390x",
    sha256 = "f61dcd7aad1df818e151d2db57321c719455b6a50b2dd189c5a58ad6d24ff966",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/krb5-libs-1.21.3-11.el10.s390x.rpm",
    ],
)

rpm(
    name = "krb5-libs-0__1.21.3-11.el10.x86_64",
    sha256 = "0423bea388f0f3e8a9b08f9bb3cd07c6fdd9303a8602e8c1d58b5ca5f32b8d65",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/krb5-libs-1.21.3-11.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libacl-0__2.3.1-3.el9.aarch64",
    sha256 = "4975593414dfa1e822cd108e988d18453c2ff036b03e4cdbf38db0afb45e0c92",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libacl-2.3.1-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/4975593414dfa1e822cd108e988d18453c2ff036b03e4cdbf38db0afb45e0c92",
    ],
)

rpm(
    name = "libacl-0__2.3.1-3.el9.x86_64",
    sha256 = "fd829e9a03f6d321313002d6fcb37ee0434f548aa75fcd3ecdbdd891115de6a7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libacl-2.3.1-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/fd829e9a03f6d321313002d6fcb37ee0434f548aa75fcd3ecdbdd891115de6a7",
    ],
)

rpm(
    name = "libacl-0__2.4.0-2.el10.aarch64",
    sha256 = "3a666e5a2151fbf10257ffc23939308e1c5fc3e9837fd8716c702c2a5b725a4b",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libacl-2.4.0-2.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libacl-0__2.4.0-2.el10.s390x",
    sha256 = "dfe7a51f3a4742386b21946a2945fbf788c5d567079653ceb01d22b0b30349f3",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libacl-2.4.0-2.el10.s390x.rpm",
    ],
)

rpm(
    name = "libacl-0__2.4.0-2.el10.x86_64",
    sha256 = "481580835e68a512240222c1350f37814965018d26d38083638869fb43d8968d",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libacl-2.4.0-2.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libacl-0__2.4.0-2.el9.aarch64",
    sha256 = "4b3f377d2ba55efbdc6c959e8216b383ce0b70fb2bf9f4b92776138f61ea47a4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libacl-2.4.0-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/4b3f377d2ba55efbdc6c959e8216b383ce0b70fb2bf9f4b92776138f61ea47a4",
    ],
)

rpm(
    name = "libacl-0__2.4.0-2.el9.s390x",
    sha256 = "b73a437fdca38c419089d8bdec96a6717c93c10cf2d435ea2e1d12674c22ae27",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libacl-2.4.0-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/b73a437fdca38c419089d8bdec96a6717c93c10cf2d435ea2e1d12674c22ae27",
    ],
)

rpm(
    name = "libacl-0__2.4.0-2.el9.x86_64",
    sha256 = "2d928a27d32720058d90b415dcf24a9df070393a5a5947117fb45524a8a9654a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libacl-2.4.0-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/2d928a27d32720058d90b415dcf24a9df070393a5a5947117fb45524a8a9654a",
    ],
)

rpm(
    name = "libaio-0__0.3.111-13.el9.aarch64",
    sha256 = "1730d732818fa2471b5cd461175ceda18e909410db8a32185d8db2aa7461130c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libaio-0.3.111-13.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/1730d732818fa2471b5cd461175ceda18e909410db8a32185d8db2aa7461130c",
    ],
)

rpm(
    name = "libaio-0__0.3.111-13.el9.x86_64",
    sha256 = "7d9d4d37e86ba94bb941e2dad40c90a157aaa0602f02f3f90e76086515f439be",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libaio-0.3.111-13.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/7d9d4d37e86ba94bb941e2dad40c90a157aaa0602f02f3f90e76086515f439be",
    ],
)

rpm(
    name = "libaio-0__0.3.112-1.el10.aarch64",
    sha256 = "d20625b3950196395eb38bbcac752ab15148c1b69c5f8e083c723734e55781f2",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libaio-0.3.112-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libaio-0__0.3.112-1.el10.s390x",
    sha256 = "8695595aed474e03111d276eedc317dbb641156a2c6b1d5c6adca9e347b7a401",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libaio-0.3.112-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "libaio-0__0.3.112-1.el10.x86_64",
    sha256 = "09eada913d3550a99da630d2471e2babf4490e603b99e5c5b86f505cd7d652d3",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libaio-0.3.112-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libaio-0__0.3.112-1.el9.aarch64",
    sha256 = "c047909c71a961ab8c245a39acc0ff444c442f5c85d8d25e9a3456cc2860d09c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libaio-0.3.112-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c047909c71a961ab8c245a39acc0ff444c442f5c85d8d25e9a3456cc2860d09c",
    ],
)

rpm(
    name = "libaio-0__0.3.112-1.el9.s390x",
    sha256 = "7bedb4d49ec750cdd791ca9e4f68652eda2d2076ed3df66a06bdd47d1368bba2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libaio-0.3.112-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/7bedb4d49ec750cdd791ca9e4f68652eda2d2076ed3df66a06bdd47d1368bba2",
    ],
)

rpm(
    name = "libaio-0__0.3.112-1.el9.x86_64",
    sha256 = "adca0bb84c8da2fdedfa49119e6071a9652e84c0c1a3abbc8d9d69bbc5ba1e83",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libaio-0.3.112-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/adca0bb84c8da2fdedfa49119e6071a9652e84c0c1a3abbc8d9d69bbc5ba1e83",
    ],
)

rpm(
    name = "libassuan-0__2.5.5-3.el9.aarch64",
    sha256 = "3efd507e48ef013bba5ca3c36a1c99923ded4f498827f927298d69f9fd06b1d0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libassuan-2.5.5-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/3efd507e48ef013bba5ca3c36a1c99923ded4f498827f927298d69f9fd06b1d0",
    ],
)

rpm(
    name = "libassuan-0__2.5.5-3.el9.s390x",
    sha256 = "56a2e5e9e6c2fde071486b174eeecec2631d3b40a6bfc036019e5cd6e590a49c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libassuan-2.5.5-3.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/56a2e5e9e6c2fde071486b174eeecec2631d3b40a6bfc036019e5cd6e590a49c",
    ],
)

rpm(
    name = "libassuan-0__2.5.5-3.el9.x86_64",
    sha256 = "3f7ab80145768029619033b31406a9aeef8c8f0d42a0c94ad464d8a3405e12b0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libassuan-2.5.5-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/3f7ab80145768029619033b31406a9aeef8c8f0d42a0c94ad464d8a3405e12b0",
    ],
)

rpm(
    name = "libassuan-0__2.5.6-6.el10.aarch64",
    sha256 = "661329502d6ab89de91da9cdc7f5d4568f8faa88141600237f444d29491d1e96",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libassuan-2.5.6-6.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libassuan-0__2.5.6-6.el10.s390x",
    sha256 = "d31b659dd6036b990ea71c10c426df13f2f395685ea8674fcca49d6a5fbeb580",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libassuan-2.5.6-6.el10.s390x.rpm",
    ],
)

rpm(
    name = "libassuan-0__2.5.6-6.el10.x86_64",
    sha256 = "5cb1eff4efadf906bb8060bb41c205bf77eabe73142599053ae892ee7d85e9c0",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libassuan-2.5.6-6.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libatomic-0__11.5.0-15.el9.aarch64",
    sha256 = "71a203d4137dfe5794be5b272b1ffe1fe2ed00be9269223b8ac4e0d360e87fbc",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libatomic-11.5.0-15.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/71a203d4137dfe5794be5b272b1ffe1fe2ed00be9269223b8ac4e0d360e87fbc",
    ],
)

rpm(
    name = "libatomic-0__14.4.1-4.el10.aarch64",
    sha256 = "b109b12189346ce53fb0a0a9204f6f94ed061ef829c6f9527178d4e67441783b",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libatomic-14.4.1-4.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libattr-0__2.5.1-3.el9.aarch64",
    sha256 = "a0101ccea66aef376f4067c1002ebdfb5dbeeecd334047459b3855eff17a6fda",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libattr-2.5.1-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/a0101ccea66aef376f4067c1002ebdfb5dbeeecd334047459b3855eff17a6fda",
    ],
)

rpm(
    name = "libattr-0__2.5.1-3.el9.x86_64",
    sha256 = "d4db095a015e84065f27a642ee7829cd1690041ba8c51501f908cc34760c9409",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libattr-2.5.1-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/d4db095a015e84065f27a642ee7829cd1690041ba8c51501f908cc34760c9409",
    ],
)

rpm(
    name = "libattr-0__2.6.0-2.el10.aarch64",
    sha256 = "e0884432ed6ac6f470ccec6a9b5df8fd7d68d6500d405dd5b18554591c3f1658",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libattr-2.6.0-2.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libattr-0__2.6.0-2.el10.s390x",
    sha256 = "93013cf4f414137aa616bad0375afaa39a477b4b958797d15973a0fc205c8ca1",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libattr-2.6.0-2.el10.s390x.rpm",
    ],
)

rpm(
    name = "libattr-0__2.6.0-2.el10.x86_64",
    sha256 = "14c7ef9d633773d4b50c23af7c1181a47c48bb999d25f0213cc5be2f92f6fbc6",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libattr-2.6.0-2.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libattr-0__2.6.0-2.el9.aarch64",
    sha256 = "111a4f7ffe93fc2dd3b3155146d31045b04a1a74d1a4e58d56386db414d28c05",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libattr-2.6.0-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/111a4f7ffe93fc2dd3b3155146d31045b04a1a74d1a4e58d56386db414d28c05",
    ],
)

rpm(
    name = "libattr-0__2.6.0-2.el9.s390x",
    sha256 = "8aceb6196eb757cf3810e0dd864b4bb1866b50dea1f9231bdb9c06e077bcd14b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libattr-2.6.0-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/8aceb6196eb757cf3810e0dd864b4bb1866b50dea1f9231bdb9c06e077bcd14b",
    ],
)

rpm(
    name = "libattr-0__2.6.0-2.el9.x86_64",
    sha256 = "4077e93ea08373cc9c65bcf6cb7cad8e88831c3a91d5814af238dfb3c9b54d10",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libattr-2.6.0-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/4077e93ea08373cc9c65bcf6cb7cad8e88831c3a91d5814af238dfb3c9b54d10",
    ],
)

rpm(
    name = "libblkid-0__2.37.2-1.el9.aarch64",
    sha256 = "32dc0d2954245d958516ef05860485d2360e0eb697abada4968953b501dfcc7a",
    urls = ["https://storage.googleapis.com/builddeps/32dc0d2954245d958516ef05860485d2360e0eb697abada4968953b501dfcc7a"],
)

rpm(
    name = "libblkid-0__2.37.2-1.el9.x86_64",
    sha256 = "f5cf36e8081c2d72e9dd64dd1614155857dd6e71ebb2237e5b0e11ace5481bac",
    urls = ["https://storage.googleapis.com/builddeps/f5cf36e8081c2d72e9dd64dd1614155857dd6e71ebb2237e5b0e11ace5481bac"],
)

rpm(
    name = "libblkid-0__2.37.4-27.el9.aarch64",
    sha256 = "5a132373b0063af29b055bafc540625545d5ed01e9e6962fb399a6921caf7a6a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libblkid-2.37.4-27.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/5a132373b0063af29b055bafc540625545d5ed01e9e6962fb399a6921caf7a6a",
    ],
)

rpm(
    name = "libblkid-0__2.37.4-27.el9.s390x",
    sha256 = "691133649b80bf613f823a4973b6f1a854d7eadab20dc4ffc0457070e7463aa0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libblkid-2.37.4-27.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/691133649b80bf613f823a4973b6f1a854d7eadab20dc4ffc0457070e7463aa0",
    ],
)

rpm(
    name = "libblkid-0__2.37.4-27.el9.x86_64",
    sha256 = "66d81b0a5892865d91612988b444c47c46599c91800ea5ab6938b977565b973a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libblkid-2.37.4-27.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/66d81b0a5892865d91612988b444c47c46599c91800ea5ab6938b977565b973a",
    ],
)

rpm(
    name = "libblkid-0__2.40.2-23.el10.aarch64",
    sha256 = "7aadce0a278c85060ed54a06ac61b86c8f42f2341ebc59c871a18e5c4676a2c5",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libblkid-2.40.2-23.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libblkid-0__2.40.2-23.el10.s390x",
    sha256 = "fa0d0f305db2e9c4f32e4f868f7a1483372cf8c029ada75df9d7a919d595118d",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libblkid-2.40.2-23.el10.s390x.rpm",
    ],
)

rpm(
    name = "libblkid-0__2.40.2-23.el10.x86_64",
    sha256 = "1409177199c0ac4f52fa9e91de9b5c8acea0e01a58f2c537c4d2f5dfe3c53fea",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libblkid-2.40.2-23.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libcap-0__2.48-10.el9.aarch64",
    sha256 = "7159fe4c1e6be9c8324632bfabcbc86ad8b7cb5105acb0b8a5c35774c93470f2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libcap-2.48-10.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/7159fe4c1e6be9c8324632bfabcbc86ad8b7cb5105acb0b8a5c35774c93470f2",
    ],
)

rpm(
    name = "libcap-0__2.48-10.el9.s390x",
    sha256 = "2883f350016ef87b8f6aa33966023cb0f3c789bdcb36374037fc94096ee61bf7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libcap-2.48-10.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/2883f350016ef87b8f6aa33966023cb0f3c789bdcb36374037fc94096ee61bf7",
    ],
)

rpm(
    name = "libcap-0__2.48-10.el9.x86_64",
    sha256 = "bda5d981249ac16603228a4f544a15a140e1eed105ab1206da6bef9705cddee7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcap-2.48-10.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/bda5d981249ac16603228a4f544a15a140e1eed105ab1206da6bef9705cddee7",
    ],
)

rpm(
    name = "libcap-0__2.48-8.el9.aarch64",
    sha256 = "881d4e7729633ce71b1a6bab3a84c1f79d5e7c49ef3ffdc1bc703cdd7ae3cd81",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libcap-2.48-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/881d4e7729633ce71b1a6bab3a84c1f79d5e7c49ef3ffdc1bc703cdd7ae3cd81",
    ],
)

rpm(
    name = "libcap-0__2.48-8.el9.x86_64",
    sha256 = "c41f91075ee8ca480c2631a485bcc74876b9317b4dc9bd66566da32313621bd7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcap-2.48-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c41f91075ee8ca480c2631a485bcc74876b9317b4dc9bd66566da32313621bd7",
    ],
)

rpm(
    name = "libcap-0__2.69-7.el10.aarch64",
    sha256 = "38c8ab1a8883b39bf46006ed39b7834cfa0df2ca0a4825908f7da4ed631c8fc6",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libcap-2.69-7.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libcap-0__2.69-7.el10.s390x",
    sha256 = "3f7016928e759a177e4103d6c31dc0833ec43eae5836d871bc5ea540fd3d0c7b",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libcap-2.69-7.el10.s390x.rpm",
    ],
)

rpm(
    name = "libcap-0__2.69-7.el10.x86_64",
    sha256 = "54c14cb5c8dc3536f43d632d766ed302a8ecad2ad8efd6aa2d079dafc11d1cd9",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libcap-2.69-7.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libcap-ng-0__0.8.2-7.el9.aarch64",
    sha256 = "1dfa7208abe1af5522523cabdabb73783ed1df4424dc8846eab8a570d010deaa",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libcap-ng-0.8.2-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/1dfa7208abe1af5522523cabdabb73783ed1df4424dc8846eab8a570d010deaa",
    ],
)

rpm(
    name = "libcap-ng-0__0.8.2-7.el9.s390x",
    sha256 = "9b68fda78e685d347ae1b9e937613125d01d7c8cdb06226e3c57e6cb08b9f306",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libcap-ng-0.8.2-7.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/9b68fda78e685d347ae1b9e937613125d01d7c8cdb06226e3c57e6cb08b9f306",
    ],
)

rpm(
    name = "libcap-ng-0__0.8.2-7.el9.x86_64",
    sha256 = "62429b788acfb40dbc9da9951690c11e907e230879c790d139f73d0e85dd76f4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcap-ng-0.8.2-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/62429b788acfb40dbc9da9951690c11e907e230879c790d139f73d0e85dd76f4",
    ],
)

rpm(
    name = "libcap-ng-0__0.9.3-2.el10.aarch64",
    sha256 = "c2233c781e7cfc9173f4a94f97c152ffdabf8e47e4fe188619abb04ffe9e9f7c",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libcap-ng-0.9.3-2.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libcap-ng-0__0.9.3-2.el10.s390x",
    sha256 = "b6b2a24c6f4199e1f211017534e56db80027077613b122e8b2f2f77438bb248f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libcap-ng-0.9.3-2.el10.s390x.rpm",
    ],
)

rpm(
    name = "libcap-ng-0__0.9.3-2.el10.x86_64",
    sha256 = "c55f364f28b60772a15c9d8996d86f755afbf58de45f1597102e7b94836e1a1a",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libcap-ng-0.9.3-2.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libcom_err-0__1.46.5-2.el9.aarch64",
    sha256 = "77acee74fb925c5dc291691b23179a5b508372328696b8881627cc64f16bb2b5",
    urls = ["https://storage.googleapis.com/builddeps/77acee74fb925c5dc291691b23179a5b508372328696b8881627cc64f16bb2b5"],
)

rpm(
    name = "libcom_err-0__1.46.5-2.el9.x86_64",
    sha256 = "579ca33574aca28a1c0de7951f6b183b5f2567cb01dfc40185e7b1f14da7f2c2",
    urls = ["https://storage.googleapis.com/builddeps/579ca33574aca28a1c0de7951f6b183b5f2567cb01dfc40185e7b1f14da7f2c2"],
)

rpm(
    name = "libcom_err-0__1.46.5-8.el9.aarch64",
    sha256 = "7bf194e4f69e548566ff21b178ae1f47d5e00f064bfa492616e4dd42f812f2a7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libcom_err-1.46.5-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/7bf194e4f69e548566ff21b178ae1f47d5e00f064bfa492616e4dd42f812f2a7",
    ],
)

rpm(
    name = "libcom_err-0__1.46.5-8.el9.s390x",
    sha256 = "b8aa8922757718f85c31dfc7c333434e576a52f9425e91f51db8fb082661c3ff",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libcom_err-1.46.5-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/b8aa8922757718f85c31dfc7c333434e576a52f9425e91f51db8fb082661c3ff",
    ],
)

rpm(
    name = "libcom_err-0__1.46.5-8.el9.x86_64",
    sha256 = "ef43794f39d49b69e12506722e432a497e7f96038e26cab2c34476aad4b3d413",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcom_err-1.46.5-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/ef43794f39d49b69e12506722e432a497e7f96038e26cab2c34476aad4b3d413",
    ],
)

rpm(
    name = "libcom_err-0__1.47.1-5.el10.aarch64",
    sha256 = "97380e5fe0fce42be70418333bae2d5d9044c5f7fbda30c9b28f7776718e76a2",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libcom_err-1.47.1-5.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libcom_err-0__1.47.1-5.el10.s390x",
    sha256 = "09f3032ebafe4e8d93152c940f160b68df98b1a1c978fae86b8524a8d7713467",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libcom_err-1.47.1-5.el10.s390x.rpm",
    ],
)

rpm(
    name = "libcom_err-0__1.47.1-5.el10.x86_64",
    sha256 = "37b036aa4cb44adade9c4206f2eea389035387082be0d2eda0249d9fd07fb842",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libcom_err-1.47.1-5.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libcurl-minimal-0__7.76.1-14.el9.aarch64",
    sha256 = "7e50f6b6f25c0855a9a509d5b205795ee4e73b18c5f8e7732f072f43d1a6714f",
    urls = ["https://storage.googleapis.com/builddeps/7e50f6b6f25c0855a9a509d5b205795ee4e73b18c5f8e7732f072f43d1a6714f"],
)

rpm(
    name = "libcurl-minimal-0__7.76.1-14.el9.x86_64",
    sha256 = "c3de56deffbd012d1b0069d1f41593d9d1414de15ea04074a0f0749884690e67",
    urls = ["https://storage.googleapis.com/builddeps/c3de56deffbd012d1b0069d1f41593d9d1414de15ea04074a0f0749884690e67"],
)

rpm(
    name = "libcurl-minimal-0__7.76.1-43.el9.aarch64",
    sha256 = "dde117f183a44553b98c14ac3ed29bf6c7a302522e436eda909cdb44980afe66",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libcurl-minimal-7.76.1-43.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/dde117f183a44553b98c14ac3ed29bf6c7a302522e436eda909cdb44980afe66",
    ],
)

rpm(
    name = "libcurl-minimal-0__7.76.1-43.el9.s390x",
    sha256 = "c2807b0788883480e4c1ecae130f66e1463672461d1ca33bee6160be5e7fe2b8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libcurl-minimal-7.76.1-43.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/c2807b0788883480e4c1ecae130f66e1463672461d1ca33bee6160be5e7fe2b8",
    ],
)

rpm(
    name = "libcurl-minimal-0__7.76.1-43.el9.x86_64",
    sha256 = "ca12a88c313df73ce0e8f5a652b57daded8733183c0d44f85f3dca780b356c67",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcurl-minimal-7.76.1-43.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/ca12a88c313df73ce0e8f5a652b57daded8733183c0d44f85f3dca780b356c67",
    ],
)

rpm(
    name = "libcurl-minimal-0__8.12.1-9.el10.aarch64",
    sha256 = "929ae81872e02bc7e77dca00cccc4920cca9d6b8cb3f601d0a80cb482ed8e898",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libcurl-minimal-8.12.1-9.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libcurl-minimal-0__8.12.1-9.el10.s390x",
    sha256 = "e3f555c91c4b16753ab4d2058b90d8549affc3e66c3543ecbd34d9cf16bcdeaa",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libcurl-minimal-8.12.1-9.el10.s390x.rpm",
    ],
)

rpm(
    name = "libcurl-minimal-0__8.12.1-9.el10.x86_64",
    sha256 = "0efc3867d333d4283b5db28aa80c9fcb10d9031d46cc5dfc68f270e0cd7a27fc",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libcurl-minimal-8.12.1-9.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libdb-0__5.3.28-57.el9.aarch64",
    sha256 = "32cfcb3dbd040c206ead6aae6bb3378246af95ab2c7ba18a9db7ec0cec649f34",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libdb-5.3.28-57.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/32cfcb3dbd040c206ead6aae6bb3378246af95ab2c7ba18a9db7ec0cec649f34",
    ],
)

rpm(
    name = "libdb-0__5.3.28-57.el9.s390x",
    sha256 = "5bae96e362fb4731b841f84d22b8ec876eeca2519404625afc51b5ae9fcd6326",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libdb-5.3.28-57.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/5bae96e362fb4731b841f84d22b8ec876eeca2519404625afc51b5ae9fcd6326",
    ],
)

rpm(
    name = "libdb-0__5.3.28-57.el9.x86_64",
    sha256 = "17f7fd8c15436826da5ac9d0428ecb83feec18c01b6c5057ab9b85ab97314c96",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libdb-5.3.28-57.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/17f7fd8c15436826da5ac9d0428ecb83feec18c01b6c5057ab9b85ab97314c96",
    ],
)

rpm(
    name = "libeconf-0__0.4.1-7.el9.aarch64",
    sha256 = "d2adf4f7d6c66c2962c1b7024d0b9514895d813aa50010ca6d1d652f3f73a87f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libeconf-0.4.1-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/d2adf4f7d6c66c2962c1b7024d0b9514895d813aa50010ca6d1d652f3f73a87f",
    ],
)

rpm(
    name = "libeconf-0__0.4.1-7.el9.s390x",
    sha256 = "19b54d80020f15ff5753d0d116faa4dd2b358f1a55c4854ea7843aa89379954a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libeconf-0.4.1-7.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/19b54d80020f15ff5753d0d116faa4dd2b358f1a55c4854ea7843aa89379954a",
    ],
)

rpm(
    name = "libeconf-0__0.4.1-7.el9.x86_64",
    sha256 = "5d852e2a7fbb298efeb05303c783afcebb369021337ca934df518362618de8f3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libeconf-0.4.1-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/5d852e2a7fbb298efeb05303c783afcebb369021337ca934df518362618de8f3",
    ],
)

rpm(
    name = "libeconf-0__0.6.2-4.el10.aarch64",
    sha256 = "1bb73420b4f72fb200ccc560224107e1ae62b8a7156051a88c9239c9def47983",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libeconf-0.6.2-4.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libeconf-0__0.6.2-4.el10.s390x",
    sha256 = "eb314cc56ffe80a641f97664b4d5a7313e3be7f1bef79e6636bcaab5751b350f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libeconf-0.6.2-4.el10.s390x.rpm",
    ],
)

rpm(
    name = "libeconf-0__0.6.2-4.el10.x86_64",
    sha256 = "1cdb8e5bf4d7680e41ebb2b76da3aab34c1ece4bba2fed952d8f49da69117dfa",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libeconf-0.6.2-4.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libevent-0__2.1.12-8.el9.aarch64",
    sha256 = "abea343484ceb42612ce394cf7cf0a191ae7d6ea93391fa32721ff7e04b0bb28",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libevent-2.1.12-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/abea343484ceb42612ce394cf7cf0a191ae7d6ea93391fa32721ff7e04b0bb28",
    ],
)

rpm(
    name = "libevent-0__2.1.12-8.el9.s390x",
    sha256 = "5c1bdffe7f5dfc8175e2b06acbb4154b272205c40d3c19b88a0d1fde095728b0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libevent-2.1.12-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/5c1bdffe7f5dfc8175e2b06acbb4154b272205c40d3c19b88a0d1fde095728b0",
    ],
)

rpm(
    name = "libevent-0__2.1.12-8.el9.x86_64",
    sha256 = "5683f51c9b02d5f4a3324dc6dacb3a84f0c3710cdc46fa7f04df64b60d38a62b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libevent-2.1.12-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/5683f51c9b02d5f4a3324dc6dacb3a84f0c3710cdc46fa7f04df64b60d38a62b",
    ],
)

rpm(
    name = "libevent-0__2.1.13-1.el10.aarch64",
    sha256 = "3c51c7b188142238d173cd34535a1f3966d9b2b38c1839606e03566d43a3e249",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libevent-2.1.13-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libevent-0__2.1.13-1.el10.s390x",
    sha256 = "143fa4f06526244c33f4532119580351a41206b8b286f8c0ee3c4da2c8e70d4b",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libevent-2.1.13-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "libevent-0__2.1.13-1.el10.x86_64",
    sha256 = "7777403e2f41ede258477a99e35e972c7b9167bce334e2901981b15f4261da69",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libevent-2.1.13-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libfdisk-0__2.37.4-27.el9.aarch64",
    sha256 = "2ae747206879c75f0b14e08eda7460eee4f87c347b5b8933a031b75c6ee27c8b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libfdisk-2.37.4-27.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/2ae747206879c75f0b14e08eda7460eee4f87c347b5b8933a031b75c6ee27c8b",
    ],
)

rpm(
    name = "libfdisk-0__2.37.4-27.el9.s390x",
    sha256 = "667fa0a54e62528a3f2b1abdba3f3d1072e17df8aad2a05476c6fe7f7422b259",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libfdisk-2.37.4-27.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/667fa0a54e62528a3f2b1abdba3f3d1072e17df8aad2a05476c6fe7f7422b259",
    ],
)

rpm(
    name = "libfdisk-0__2.37.4-27.el9.x86_64",
    sha256 = "b548802a8ca756bd0968969beeb5c1bb7836ec22755faed921b748b8c3b6828a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libfdisk-2.37.4-27.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/b548802a8ca756bd0968969beeb5c1bb7836ec22755faed921b748b8c3b6828a",
    ],
)

rpm(
    name = "libfdisk-0__2.40.2-23.el10.aarch64",
    sha256 = "5386e32287c0ce528287d86e06b5f46386e2af55bd6ceb8f8017873bafc95f76",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libfdisk-2.40.2-23.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libfdisk-0__2.40.2-23.el10.s390x",
    sha256 = "a4a157152161e521d3495a5b6e418b5391b1d62128db06d6db130c4748da7f7d",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libfdisk-2.40.2-23.el10.s390x.rpm",
    ],
)

rpm(
    name = "libfdisk-0__2.40.2-23.el10.x86_64",
    sha256 = "9d2665bdd7fb43b4115ef471153c4367f30f89e9e4dd5e33e481449ea5b75710",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libfdisk-2.40.2-23.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libffi-0__3.4.2-7.el9.aarch64",
    sha256 = "6a42002c0b63a3c4d1e8da5cdf4822f442a7b458d80e69673715715d38ea977d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libffi-3.4.2-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/6a42002c0b63a3c4d1e8da5cdf4822f442a7b458d80e69673715715d38ea977d",
    ],
)

rpm(
    name = "libffi-0__3.4.2-7.el9.x86_64",
    sha256 = "f0ac4b6454d4018833dd10e3f437d8271c7c6a628d99b37e75b83af890b86bc4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libffi-3.4.2-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f0ac4b6454d4018833dd10e3f437d8271c7c6a628d99b37e75b83af890b86bc4",
    ],
)

rpm(
    name = "libffi-0__3.4.2-8.el9.aarch64",
    sha256 = "da6d3f1b21c23a97e61c35fde044aca5bc9f1097ffdcb387759f544c61548301",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libffi-3.4.2-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/da6d3f1b21c23a97e61c35fde044aca5bc9f1097ffdcb387759f544c61548301",
    ],
)

rpm(
    name = "libffi-0__3.4.2-8.el9.s390x",
    sha256 = "25556c4a1bdb85f426595faa76996616a45986c93cac4361c2371f2e9b737304",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libffi-3.4.2-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/25556c4a1bdb85f426595faa76996616a45986c93cac4361c2371f2e9b737304",
    ],
)

rpm(
    name = "libffi-0__3.4.2-8.el9.x86_64",
    sha256 = "110d5008364a65b38b832949970886fdccb97762b0cdb257571cc0c84182d7d0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libffi-3.4.2-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/110d5008364a65b38b832949970886fdccb97762b0cdb257571cc0c84182d7d0",
    ],
)

rpm(
    name = "libffi-0__3.4.4-10.el10.aarch64",
    sha256 = "87b620ad4069f0a9623913acc568a2659bcee3695293b275a9f09b809437bf6e",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libffi-3.4.4-10.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libffi-0__3.4.4-10.el10.s390x",
    sha256 = "592be60a3f4ee70236cc254894f587012ce533a6f4fc74031bf6674792782338",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libffi-3.4.4-10.el10.s390x.rpm",
    ],
)

rpm(
    name = "libffi-0__3.4.4-10.el10.x86_64",
    sha256 = "72aff2f3b4291f5418491e612be4f92d65a9239224a4906c0c63dbc4fc668e73",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libffi-3.4.4-10.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libgcc-0__11.2.1-9.4.el9.aarch64",
    sha256 = "83553d747fbbe61a2e5a22604f15d38e366bf4b453c99947bc1253ddec6b5049",
    urls = ["https://storage.googleapis.com/builddeps/83553d747fbbe61a2e5a22604f15d38e366bf4b453c99947bc1253ddec6b5049"],
)

rpm(
    name = "libgcc-0__11.2.1-9.4.el9.x86_64",
    sha256 = "34443f5befca73364cc7db887c4a95a254ba662cd45d80765a77a84e3a5da59f",
    urls = ["https://storage.googleapis.com/builddeps/34443f5befca73364cc7db887c4a95a254ba662cd45d80765a77a84e3a5da59f"],
)

rpm(
    name = "libgcc-0__11.5.0-15.el9.aarch64",
    sha256 = "9fa96a86f8bfe2a03390874f4e794cac76a82edbd47d6bfe881ab0de0a59efb1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libgcc-11.5.0-15.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/9fa96a86f8bfe2a03390874f4e794cac76a82edbd47d6bfe881ab0de0a59efb1",
    ],
)

rpm(
    name = "libgcc-0__11.5.0-15.el9.s390x",
    sha256 = "330c7cf21f7b3adb9f48ffc732d5c7470abc68a33775e9b99da7070e241e1b46",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libgcc-11.5.0-15.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/330c7cf21f7b3adb9f48ffc732d5c7470abc68a33775e9b99da7070e241e1b46",
    ],
)

rpm(
    name = "libgcc-0__11.5.0-15.el9.x86_64",
    sha256 = "522e07f3a8a09a6d5c9174340031d3002d319d4f6ecad8a483d30d68f02fc36d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libgcc-11.5.0-15.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/522e07f3a8a09a6d5c9174340031d3002d319d4f6ecad8a483d30d68f02fc36d",
    ],
)

rpm(
    name = "libgcc-0__14.4.1-4.el10.aarch64",
    sha256 = "56b464029169e9dbed732d2cc76425548b90ad78726b93e9a171a654c02bb2a2",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libgcc-14.4.1-4.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libgcc-0__14.4.1-4.el10.s390x",
    sha256 = "256c8a4a6f000d9d111af59955ec87a3eaa835753a7388901ccc76fb7e4b8e47",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libgcc-14.4.1-4.el10.s390x.rpm",
    ],
)

rpm(
    name = "libgcc-0__14.4.1-4.el10.x86_64",
    sha256 = "89db753f56c336f5a7da684ea8dc42c85aedd436f9a1b7a839abf6e1168f9580",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libgcc-14.4.1-4.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libgcrypt-0__1.10.0-13.el9.aarch64",
    sha256 = "dd1b8da929a138573303d096391893f6ebf72a2964771d793b2c65334dbefe0f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libgcrypt-1.10.0-13.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/dd1b8da929a138573303d096391893f6ebf72a2964771d793b2c65334dbefe0f",
    ],
)

rpm(
    name = "libgcrypt-0__1.10.0-13.el9.s390x",
    sha256 = "46bc0123d8d988b3cd91c76cf6ef7c4c4c61482a83ad919bb001a6f7809ce69e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libgcrypt-1.10.0-13.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/46bc0123d8d988b3cd91c76cf6ef7c4c4c61482a83ad919bb001a6f7809ce69e",
    ],
)

rpm(
    name = "libgcrypt-0__1.10.0-13.el9.x86_64",
    sha256 = "71026b5a461fc0c4777db81a529dea0f9205f74c175bbdfbc0f5bab8475d05c9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libgcrypt-1.10.0-13.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/71026b5a461fc0c4777db81a529dea0f9205f74c175bbdfbc0f5bab8475d05c9",
    ],
)

rpm(
    name = "libgcrypt-0__1.10.0-2.el9.aarch64",
    sha256 = "4728173b70ec6a491c42bcceaac35666a5725a9f87ad01d2571bf85f5beb8d60",
    urls = ["https://storage.googleapis.com/builddeps/4728173b70ec6a491c42bcceaac35666a5725a9f87ad01d2571bf85f5beb8d60"],
)

rpm(
    name = "libgcrypt-0__1.10.0-2.el9.x86_64",
    sha256 = "b0766b669c0b236676777c91bcd0d22cc6412155583085c2bd62e84e4b42865b",
    urls = ["https://storage.googleapis.com/builddeps/b0766b669c0b236676777c91bcd0d22cc6412155583085c2bd62e84e4b42865b"],
)

rpm(
    name = "libgcrypt-0__1.11.0-7.el10.aarch64",
    sha256 = "0e92e3cd6cfb466a10a82cdca7590801e4394e13842df3150b15f3f8168f2929",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libgcrypt-1.11.0-7.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libgcrypt-0__1.11.0-7.el10.s390x",
    sha256 = "a00dbc7273e8a5c342307957b810914bbc4a4d3ccf5e1de20a17309a8e0a4309",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libgcrypt-1.11.0-7.el10.s390x.rpm",
    ],
)

rpm(
    name = "libgcrypt-0__1.11.0-7.el10.x86_64",
    sha256 = "53d46313b1566f2ce977add8fbd4bc08f6a74e2ae684549b3315fea3ee73b1e2",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libgcrypt-1.11.0-7.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libgpg-error-0__1.42-5.el9.aarch64",
    sha256 = "ffeb04823b5317c7e016542c8ecc5180c7824f8b59a180f2434fd096a34a9105",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libgpg-error-1.42-5.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ffeb04823b5317c7e016542c8ecc5180c7824f8b59a180f2434fd096a34a9105",
    ],
)

rpm(
    name = "libgpg-error-0__1.42-5.el9.s390x",
    sha256 = "655367cd72f1908dbc2e42fee35974447d33eae7ec07249d3df098a6512d4601",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libgpg-error-1.42-5.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/655367cd72f1908dbc2e42fee35974447d33eae7ec07249d3df098a6512d4601",
    ],
)

rpm(
    name = "libgpg-error-0__1.42-5.el9.x86_64",
    sha256 = "a1883804c376f737109f4dff06077d1912b90150a732d11be7bc5b3b67e512fe",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libgpg-error-1.42-5.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/a1883804c376f737109f4dff06077d1912b90150a732d11be7bc5b3b67e512fe",
    ],
)

rpm(
    name = "libgpg-error-0__1.50-2.el10.aarch64",
    sha256 = "c673cfa8104bd94555b70c9883960f54a7d15428fe2e4222f2cc8eeb3f0c202e",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libgpg-error-1.50-2.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libgpg-error-0__1.50-2.el10.s390x",
    sha256 = "d2ae277868010dcf3003a59234b70743f2ff0846e0f8aba6cf349de19d6c2173",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libgpg-error-1.50-2.el10.s390x.rpm",
    ],
)

rpm(
    name = "libgpg-error-0__1.50-2.el10.x86_64",
    sha256 = "b7d74c79f82abf581fdb5b9fbd0b3792640c26780652036be284347b7b339fff",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libgpg-error-1.50-2.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libidn2-0__2.3.0-7.el9.aarch64",
    sha256 = "6ed96112059449aa37b99d4d4e3b5d089c34afefbd9b618691bed8c206c4d441",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libidn2-2.3.0-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/6ed96112059449aa37b99d4d4e3b5d089c34afefbd9b618691bed8c206c4d441",
    ],
)

rpm(
    name = "libidn2-0__2.3.0-7.el9.s390x",
    sha256 = "716716b688d4b702cee523a82d4ee035675f01ee404eb7dd7f2ef63d3389bb66",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libidn2-2.3.0-7.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/716716b688d4b702cee523a82d4ee035675f01ee404eb7dd7f2ef63d3389bb66",
    ],
)

rpm(
    name = "libidn2-0__2.3.0-7.el9.x86_64",
    sha256 = "f7fa1ad2fcd86beea5d4d965994c21dc98f47871faff14f73940190c754ab244",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libidn2-2.3.0-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f7fa1ad2fcd86beea5d4d965994c21dc98f47871faff14f73940190c754ab244",
    ],
)

rpm(
    name = "libidn2-0__2.3.7-3.el10.aarch64",
    sha256 = "947248aeedd08f88d9490f3020dee6416595cf8d25e15738c306d55e9cae8bfc",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libidn2-2.3.7-3.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libidn2-0__2.3.7-3.el10.s390x",
    sha256 = "7d75542211c9b7b8e53aaf6aee0dc430f091414fb2ca755baadc35f844bded75",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libidn2-2.3.7-3.el10.s390x.rpm",
    ],
)

rpm(
    name = "libidn2-0__2.3.7-3.el10.x86_64",
    sha256 = "04ae61bbe2cc0db7581d6f96a562b9b87a8a4dba714a0cf2c73bba6306e94c27",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libidn2-2.3.7-3.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libksba-0__1.5.1-7.el9.aarch64",
    sha256 = "48fca9ffafad57ad6b021261e7998b97e56a63fd79344f8540c61411bf4cda90",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libksba-1.5.1-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/48fca9ffafad57ad6b021261e7998b97e56a63fd79344f8540c61411bf4cda90",
    ],
)

rpm(
    name = "libksba-0__1.5.1-7.el9.s390x",
    sha256 = "10e17f1f886f90259f915e855389f3e3852fddd52be35110ebe0d0f4b9b4f51a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libksba-1.5.1-7.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/10e17f1f886f90259f915e855389f3e3852fddd52be35110ebe0d0f4b9b4f51a",
    ],
)

rpm(
    name = "libksba-0__1.5.1-7.el9.x86_64",
    sha256 = "8c2a4312f0a700286e1c3630f62dba6d06e7a4c07a17182ca97f2d40d0b4c6a0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libksba-1.5.1-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/8c2a4312f0a700286e1c3630f62dba6d06e7a4c07a17182ca97f2d40d0b4c6a0",
    ],
)

rpm(
    name = "libksba-0__1.6.7-2.el10.aarch64",
    sha256 = "031a9c99556ab18893d46a5d21054c8033e7d472fa2ed9e790d580f34e05abe3",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libksba-1.6.7-2.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libksba-0__1.6.7-2.el10.s390x",
    sha256 = "f4a0e294968ce54ad30e4de5baabcdebd7a9db7900266e4feb7cadecd7f18cba",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libksba-1.6.7-2.el10.s390x.rpm",
    ],
)

rpm(
    name = "libksba-0__1.6.7-2.el10.x86_64",
    sha256 = "2bfa8330ad9c63eaecd2bd1d0989625e812d853a85c505cb759a2d1c06750607",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libksba-1.6.7-2.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libmnl-0__1.0.4-16.el9.aarch64",
    sha256 = "c4d87c6439aa762891b024c0213df47af50e5b0683ffd827013bd02882d7d9b3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libmnl-1.0.4-16.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c4d87c6439aa762891b024c0213df47af50e5b0683ffd827013bd02882d7d9b3",
    ],
)

rpm(
    name = "libmnl-0__1.0.4-16.el9.s390x",
    sha256 = "344f21dedaaad1ddc5279e31a4dafd9354662a61f010249d86a424c903c4415a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libmnl-1.0.4-16.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/344f21dedaaad1ddc5279e31a4dafd9354662a61f010249d86a424c903c4415a",
    ],
)

rpm(
    name = "libmnl-0__1.0.4-16.el9.x86_64",
    sha256 = "e60f3be453b44ea04bb596594963be1e1b3f4377f87b4ff923d612eae15740ce",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libmnl-1.0.4-16.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e60f3be453b44ea04bb596594963be1e1b3f4377f87b4ff923d612eae15740ce",
    ],
)

rpm(
    name = "libmnl-0__1.0.5-7.el10.aarch64",
    sha256 = "786a9caea9de8f4529e5ec07ac24c9cfccd50ee5f4b045c37dbd4eb074b34f34",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libmnl-1.0.5-7.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libmnl-0__1.0.5-7.el10.s390x",
    sha256 = "bb6229f5c62e69828f94cd16f0086115e17ea9c9ab831ad14781f9b9807cd3d8",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libmnl-1.0.5-7.el10.s390x.rpm",
    ],
)

rpm(
    name = "libmnl-0__1.0.5-7.el10.x86_64",
    sha256 = "1e1d36725d958fc3f9016cc85b238bf5462a43ae7be144d0a4a72be0982d68ce",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libmnl-1.0.5-7.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libmount-0__2.37.2-1.el9.aarch64",
    sha256 = "7ae3f2c10203d0fb0b76d3abd7f58197a62b8898572add7488de1a7570ea407d",
    urls = ["https://storage.googleapis.com/builddeps/7ae3f2c10203d0fb0b76d3abd7f58197a62b8898572add7488de1a7570ea407d"],
)

rpm(
    name = "libmount-0__2.37.2-1.el9.x86_64",
    sha256 = "26191af0cc7acf9bb335ebd8b4ed357582165ee3be78fce9f4395f84ad2805ce",
    urls = ["https://storage.googleapis.com/builddeps/26191af0cc7acf9bb335ebd8b4ed357582165ee3be78fce9f4395f84ad2805ce"],
)

rpm(
    name = "libmount-0__2.37.4-27.el9.aarch64",
    sha256 = "6beb74f9495aaf4bd05becb9341ec4c1abae1f61fa83f4ffb85f9d98d0819a76",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libmount-2.37.4-27.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/6beb74f9495aaf4bd05becb9341ec4c1abae1f61fa83f4ffb85f9d98d0819a76",
    ],
)

rpm(
    name = "libmount-0__2.37.4-27.el9.s390x",
    sha256 = "11e43d8647f79a6aa0928cb49c2f167de5783aa484d184fbe541405d699e4a11",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libmount-2.37.4-27.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/11e43d8647f79a6aa0928cb49c2f167de5783aa484d184fbe541405d699e4a11",
    ],
)

rpm(
    name = "libmount-0__2.37.4-27.el9.x86_64",
    sha256 = "c9a9db91ef7911a1a88b2716cbde213c2a627f0d8ac5444438b03808afbabfdf",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libmount-2.37.4-27.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c9a9db91ef7911a1a88b2716cbde213c2a627f0d8ac5444438b03808afbabfdf",
    ],
)

rpm(
    name = "libmount-0__2.40.2-23.el10.aarch64",
    sha256 = "ecaaa3e4df045a52224b72003df2ba8749140b72fc73c9e4732d4f59a0cf620f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libmount-2.40.2-23.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libmount-0__2.40.2-23.el10.s390x",
    sha256 = "67607da2535b080075f8b826abb65db333dd3e3934c88e22a2669e2dc697460b",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libmount-2.40.2-23.el10.s390x.rpm",
    ],
)

rpm(
    name = "libmount-0__2.40.2-23.el10.x86_64",
    sha256 = "4e0aea814e36596ef88451b3e47ed203d83ce6e8b58a8291133653b64e04e2ca",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libmount-2.40.2-23.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libnbd-0__1.20.3-4.el9.aarch64",
    sha256 = "7c9bb6872b93d95b2a2bf729793b50848cde216089293010197471146d23d9a4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/libnbd-1.20.3-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/7c9bb6872b93d95b2a2bf729793b50848cde216089293010197471146d23d9a4",
    ],
)

rpm(
    name = "libnbd-0__1.20.3-4.el9.s390x",
    sha256 = "d73945914b3ea835369f64416cf111fcf527775d70e35109f2a270763328e6ce",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/libnbd-1.20.3-4.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/d73945914b3ea835369f64416cf111fcf527775d70e35109f2a270763328e6ce",
    ],
)

rpm(
    name = "libnbd-0__1.20.3-4.el9.x86_64",
    sha256 = "d74d51b389dcf44bd2e10e76085dc41db925debee2ce33b721c554a9dd1f40af",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libnbd-1.20.3-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/d74d51b389dcf44bd2e10e76085dc41db925debee2ce33b721c554a9dd1f40af",
    ],
)

rpm(
    name = "libnbd-0__1.25.8-1.el10.aarch64",
    sha256 = "973de49c01a31aad3cbe0c2b0d985857c9dd8fec2ea07729247762463959a427",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/aarch64/os/Packages/libnbd-1.25.8-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libnbd-0__1.25.8-1.el10.s390x",
    sha256 = "6181ce4fbe8e0edc26098a00e3706da9375aed59a0a32a8d9159ec23e3287a7f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/s390x/os/Packages/libnbd-1.25.8-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "libnbd-0__1.25.8-1.el10.x86_64",
    sha256 = "3064e10c09125acbf77b1efd29790469cdecba6265163644b9ac4e6b2e9081ca",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/x86_64/os/Packages/libnbd-1.25.8-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libnetfilter_conntrack-0__1.0.9-1.el9.aarch64",
    sha256 = "6871a3371b5a9a8239606efd453b59b274040e9d8d8f0c18bdffa7264db64264",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libnetfilter_conntrack-1.0.9-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/6871a3371b5a9a8239606efd453b59b274040e9d8d8f0c18bdffa7264db64264",
    ],
)

rpm(
    name = "libnetfilter_conntrack-0__1.0.9-1.el9.s390x",
    sha256 = "803ecb7d6e42554735836a113b61e8501e952a715c754b76cec90631926e4830",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libnetfilter_conntrack-1.0.9-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/803ecb7d6e42554735836a113b61e8501e952a715c754b76cec90631926e4830",
    ],
)

rpm(
    name = "libnetfilter_conntrack-0__1.0.9-1.el9.x86_64",
    sha256 = "f81a0188964268ae9e1d53d99dba3ef96a65fe2fb00bc8fe6c39cedfdd364f44",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnetfilter_conntrack-1.0.9-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f81a0188964268ae9e1d53d99dba3ef96a65fe2fb00bc8fe6c39cedfdd364f44",
    ],
)

rpm(
    name = "libnetfilter_conntrack-0__1.0.9-12.el10.aarch64",
    sha256 = "53c1b45e66ef040f6486052395cfda198d9b8b3058834ae7e8b7864b04f9c766",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libnetfilter_conntrack-1.0.9-12.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libnetfilter_conntrack-0__1.0.9-12.el10.s390x",
    sha256 = "f7f15dc88c380db9cbf6e62c1a04b872867d7f5a4f2cc0b2f9988db963d8401d",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libnetfilter_conntrack-1.0.9-12.el10.s390x.rpm",
    ],
)

rpm(
    name = "libnetfilter_conntrack-0__1.0.9-12.el10.x86_64",
    sha256 = "71af0b9fb8b790e3d471a74ef463dc3cbb0267c9bdbaf876160fa9821f63200f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libnetfilter_conntrack-1.0.9-12.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libnfnetlink-0__1.0.1-23.el9.aarch64",
    sha256 = "8b261a1555fd3b299c8b16d7c1159c726ec17dbd78d5217dbc6e69099f01c6cb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libnfnetlink-1.0.1-23.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/8b261a1555fd3b299c8b16d7c1159c726ec17dbd78d5217dbc6e69099f01c6cb",
    ],
)

rpm(
    name = "libnfnetlink-0__1.0.1-23.el9.s390x",
    sha256 = "1d092de5c4fde5b75011185bda315959d01994c162009b63373e901e72e42769",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libnfnetlink-1.0.1-23.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/1d092de5c4fde5b75011185bda315959d01994c162009b63373e901e72e42769",
    ],
)

rpm(
    name = "libnfnetlink-0__1.0.1-23.el9.x86_64",
    sha256 = "c920598cb4dab7c5b6b00af9f09c21f89b23c4e12729016fd892d6d7e1291615",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnfnetlink-1.0.1-23.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c920598cb4dab7c5b6b00af9f09c21f89b23c4e12729016fd892d6d7e1291615",
    ],
)

rpm(
    name = "libnfnetlink-0__1.0.2-3.el10.aarch64",
    sha256 = "0ac9c6ebd2c5652bea632435fd73bfb36785d2f15eb840ca53049d6bb3abe639",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libnfnetlink-1.0.2-3.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libnfnetlink-0__1.0.2-3.el10.s390x",
    sha256 = "433915ab5525daf404ae37d47f1f735d257a33ff8a2565741e3449d5b39b0cab",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libnfnetlink-1.0.2-3.el10.s390x.rpm",
    ],
)

rpm(
    name = "libnfnetlink-0__1.0.2-3.el10.x86_64",
    sha256 = "2988f90762058160e4071b79b96523901a2170bc6242488878ce51fbc0d871ca",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libnfnetlink-1.0.2-3.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libnftnl-0__1.2.6-4.el9.aarch64",
    sha256 = "59f6d922f5540479c088120d411d2ca3cdb4e5ddf6fe8fc05dbd796b9e36ecd3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libnftnl-1.2.6-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/59f6d922f5540479c088120d411d2ca3cdb4e5ddf6fe8fc05dbd796b9e36ecd3",
    ],
)

rpm(
    name = "libnftnl-0__1.2.6-4.el9.s390x",
    sha256 = "1a717d2a04f257e452753ba29cc6c0848cd51a226bf5d000b89863fa7aad5250",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libnftnl-1.2.6-4.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/1a717d2a04f257e452753ba29cc6c0848cd51a226bf5d000b89863fa7aad5250",
    ],
)

rpm(
    name = "libnftnl-0__1.2.6-4.el9.x86_64",
    sha256 = "45d7325859bdfbddd9f24235695fc55138549fdccbe509484e9f905c5f1b466b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnftnl-1.2.6-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/45d7325859bdfbddd9f24235695fc55138549fdccbe509484e9f905c5f1b466b",
    ],
)

rpm(
    name = "libnftnl-0__1.3.2-1.el10.aarch64",
    sha256 = "57872320022dac3679deba277d6cb9428f2b994c98c591f8e4390a4eb4c7f47a",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libnftnl-1.3.2-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libnftnl-0__1.3.2-1.el10.s390x",
    sha256 = "d07a561a388f91664c12c47496dcae1578d991096d96120f7bf4cc1c8f42f48e",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libnftnl-1.3.2-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "libnftnl-0__1.3.2-1.el10.x86_64",
    sha256 = "ccde9e279c8c4edc88e9a89cc05358fe9c16e8525b8c4486c08d70f94ac75863",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libnftnl-1.3.2-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libnghttp2-0__1.43.0-5.el9.aarch64",
    sha256 = "702abf0c5b1574b828132e4dbea17ad7099034db18f47fd1ac84b4d9534dcfea",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libnghttp2-1.43.0-5.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/702abf0c5b1574b828132e4dbea17ad7099034db18f47fd1ac84b4d9534dcfea",
    ],
)

rpm(
    name = "libnghttp2-0__1.43.0-5.el9.x86_64",
    sha256 = "58c5d589ee370951b98e908ac05a5a6154d52dbb8cf2067583ccdd10cdf099bf",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnghttp2-1.43.0-5.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/58c5d589ee370951b98e908ac05a5a6154d52dbb8cf2067583ccdd10cdf099bf",
    ],
)

rpm(
    name = "libnghttp2-0__1.43.0-8.el9.aarch64",
    sha256 = "59ec0019cc96ae6ac5d2ec21e4688a6d2ecd309e64ff0dec30326d161317f058",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libnghttp2-1.43.0-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/59ec0019cc96ae6ac5d2ec21e4688a6d2ecd309e64ff0dec30326d161317f058",
    ],
)

rpm(
    name = "libnghttp2-0__1.43.0-8.el9.s390x",
    sha256 = "0d20f0659ec2bc88bcd269a32beadcebc744cb74a630c43dd2dbf7937e198826",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libnghttp2-1.43.0-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/0d20f0659ec2bc88bcd269a32beadcebc744cb74a630c43dd2dbf7937e198826",
    ],
)

rpm(
    name = "libnghttp2-0__1.43.0-8.el9.x86_64",
    sha256 = "e854ce03f03d2d0501df8c821bed6cbe77d02e93c306f61fbe4f96c57fd37445",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnghttp2-1.43.0-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e854ce03f03d2d0501df8c821bed6cbe77d02e93c306f61fbe4f96c57fd37445",
    ],
)

rpm(
    name = "libnghttp2-0__1.68.0-6.el10.aarch64",
    sha256 = "e78541b9f3562dbed52bed8a05964f7389238044153039af84e9fec540fa3370",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libnghttp2-1.68.0-6.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libnghttp2-0__1.68.0-6.el10.s390x",
    sha256 = "6db0bbe44d70c97a5d6c9e4af299cc8ef0c67308d5159e12deede3bb519f7edb",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libnghttp2-1.68.0-6.el10.s390x.rpm",
    ],
)

rpm(
    name = "libnghttp2-0__1.68.0-6.el10.x86_64",
    sha256 = "5a8d4ee49593a8cd69c30965c0018de0f33371c0e9d43fbbe23a9c1a83b1646d",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libnghttp2-1.68.0-6.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libpwquality-0__1.4.4-8.el9.aarch64",
    sha256 = "3c22a268ce022cb4722aa2d35a95c1174778f424fbf29e98990801651d468aeb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libpwquality-1.4.4-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/3c22a268ce022cb4722aa2d35a95c1174778f424fbf29e98990801651d468aeb",
    ],
)

rpm(
    name = "libpwquality-0__1.4.4-8.el9.s390x",
    sha256 = "b8b5178474a0a53bc6463e817e0bca8a3568e333bcae9eda3dabbe84a1e24941",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libpwquality-1.4.4-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/b8b5178474a0a53bc6463e817e0bca8a3568e333bcae9eda3dabbe84a1e24941",
    ],
)

rpm(
    name = "libpwquality-0__1.4.4-8.el9.x86_64",
    sha256 = "93f00e5efac1e3f1ecbc0d6a4c068772cb12912cd20c9ea58716d6c0cd004886",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libpwquality-1.4.4-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/93f00e5efac1e3f1ecbc0d6a4c068772cb12912cd20c9ea58716d6c0cd004886",
    ],
)

rpm(
    name = "libseccomp-0__2.5.6-1.el10.aarch64",
    sha256 = "322aa4ea140a63645c7f086b58a08346617eea2efee8044287b76373d633b65f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libseccomp-2.5.6-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libseccomp-0__2.5.6-1.el10.s390x",
    sha256 = "e652de14f8c0d52480c2bba779daf5da7b7fd66e65090e1781f41d3bee250840",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libseccomp-2.5.6-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "libseccomp-0__2.5.6-1.el10.x86_64",
    sha256 = "654051862cc301ed43501ab36b687ed5adeb3ca57689f54a80bf760ad9686e54",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libseccomp-2.5.6-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libseccomp-0__2.5.6-1.el9.aarch64",
    sha256 = "74a99b069ffe2fdd6f2ee19c73197c0ad1b71353df39c5af8c404932a5817974",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libseccomp-2.5.6-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/74a99b069ffe2fdd6f2ee19c73197c0ad1b71353df39c5af8c404932a5817974",
    ],
)

rpm(
    name = "libseccomp-0__2.5.6-1.el9.s390x",
    sha256 = "155ef4319fc1fffa926ba688e12cd3d49e616f55474278b5df2e3a75d971d1a8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libseccomp-2.5.6-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/155ef4319fc1fffa926ba688e12cd3d49e616f55474278b5df2e3a75d971d1a8",
    ],
)

rpm(
    name = "libseccomp-0__2.5.6-1.el9.x86_64",
    sha256 = "73779d9eb83b4334fb312a7a6bcf7764780777f168724d7e57f6477fd912ac0a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libseccomp-2.5.6-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/73779d9eb83b4334fb312a7a6bcf7764780777f168724d7e57f6477fd912ac0a",
    ],
)

rpm(
    name = "libselinux-0__3.11-3.el10.aarch64",
    sha256 = "6b94481d8001e5b1cc198e830456e1616c46da2b1205086e30ae9f0a126ba1b7",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libselinux-3.11-3.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libselinux-0__3.11-3.el10.s390x",
    sha256 = "0071a2681cb80f314f22adb19ffe560ea7b1d8cb0b60aa485dc85d2784cda9af",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libselinux-3.11-3.el10.s390x.rpm",
    ],
)

rpm(
    name = "libselinux-0__3.11-3.el10.x86_64",
    sha256 = "5dfb040a205b1c1df612022675366f07db06d5c928079d096d36f24aac6f6ecd",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libselinux-3.11-3.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libselinux-0__3.3-2.el9.aarch64",
    sha256 = "f14cadbedd18e37a5ecb11d112095aa3e539de58bea77fb6f2aca5f165bf788b",
    urls = ["https://storage.googleapis.com/builddeps/f14cadbedd18e37a5ecb11d112095aa3e539de58bea77fb6f2aca5f165bf788b"],
)

rpm(
    name = "libselinux-0__3.3-2.el9.x86_64",
    sha256 = "8e589b8408b04cbc19564620b229b6768edbaeb9090885d2273d84b8fc2f172b",
    urls = ["https://storage.googleapis.com/builddeps/8e589b8408b04cbc19564620b229b6768edbaeb9090885d2273d84b8fc2f172b"],
)

rpm(
    name = "libselinux-0__3.6-4.el9.aarch64",
    sha256 = "b33fc63c93f3f1194c542c443f6c9b511fa149002fddd527d73e2ee0ddc1f774",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libselinux-3.6-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/b33fc63c93f3f1194c542c443f6c9b511fa149002fddd527d73e2ee0ddc1f774",
    ],
)

rpm(
    name = "libselinux-0__3.6-4.el9.s390x",
    sha256 = "98e1519df815f0f878f4c49810432c0ee305b1a52bb87c8f979e10570b3e1362",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libselinux-3.6-4.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/98e1519df815f0f878f4c49810432c0ee305b1a52bb87c8f979e10570b3e1362",
    ],
)

rpm(
    name = "libselinux-0__3.6-4.el9.x86_64",
    sha256 = "856d614fa2ba1a9d87ebc1ab78554a62c7fa6b7f37594dd9faaff1aac601ae94",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libselinux-3.6-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/856d614fa2ba1a9d87ebc1ab78554a62c7fa6b7f37594dd9faaff1aac601ae94",
    ],
)

rpm(
    name = "libsemanage-0__3.11-3.el10.aarch64",
    sha256 = "52c11f5d9e017a86bd12d9aa89a975a38b07a3ffa140352cff8299cd4e002f98",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libsemanage-3.11-3.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libsemanage-0__3.11-3.el10.s390x",
    sha256 = "a0eb482aaafe57cf1ad967ee5821469a80b99f501c824670300bc43d4e083b34",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libsemanage-3.11-3.el10.s390x.rpm",
    ],
)

rpm(
    name = "libsemanage-0__3.11-3.el10.x86_64",
    sha256 = "22beef891f54f2eaa5fbf75dc262bb29320249ec204d0626eaf19339c99eda6c",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libsemanage-3.11-3.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libsemanage-0__3.6-5.el9.aarch64",
    sha256 = "f5402c7056dc92ea2e52ad436c6eece8c18040ac77141e5f0ffe01eea209dfe7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libsemanage-3.6-5.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f5402c7056dc92ea2e52ad436c6eece8c18040ac77141e5f0ffe01eea209dfe7",
    ],
)

rpm(
    name = "libsemanage-0__3.6-5.el9.s390x",
    sha256 = "888a4ef687c43c03324bfe3c5815810d48322478cd966b4bcb1d237a16b3a0b0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libsemanage-3.6-5.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/888a4ef687c43c03324bfe3c5815810d48322478cd966b4bcb1d237a16b3a0b0",
    ],
)

rpm(
    name = "libsemanage-0__3.6-5.el9.x86_64",
    sha256 = "3dcf6e7f2779434d9dc7aef0065c3a2977792170264a60d4324f6625bb9cd69a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsemanage-3.6-5.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/3dcf6e7f2779434d9dc7aef0065c3a2977792170264a60d4324f6625bb9cd69a",
    ],
)

rpm(
    name = "libsepol-0__3.11-2.el10.aarch64",
    sha256 = "c58953aef27e129bd40fe0cef848511322c16f4ada0be524e434a123cef05ca7",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libsepol-3.11-2.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libsepol-0__3.11-2.el10.s390x",
    sha256 = "eba39391d0cf2386f1deef92b296d04f5bc49779f07e05ce348211f3e18d5b11",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libsepol-3.11-2.el10.s390x.rpm",
    ],
)

rpm(
    name = "libsepol-0__3.11-2.el10.x86_64",
    sha256 = "3111ab062d08411b274da979bd7fef1c6ebd95d90d589d5cbabd29189ec5ba7a",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libsepol-3.11-2.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libsepol-0__3.3-2.el9.aarch64",
    sha256 = "74dcae8d3dceb2aac2cbb3440015419aa4fec51e485eb92ef82df057f574e0ca",
    urls = ["https://storage.googleapis.com/builddeps/74dcae8d3dceb2aac2cbb3440015419aa4fec51e485eb92ef82df057f574e0ca"],
)

rpm(
    name = "libsepol-0__3.3-2.el9.x86_64",
    sha256 = "fc508147fe876706b61941a6ce554d7f7786f1ec3d097c4411fd6c7511acd289",
    urls = ["https://storage.googleapis.com/builddeps/fc508147fe876706b61941a6ce554d7f7786f1ec3d097c4411fd6c7511acd289"],
)

rpm(
    name = "libsepol-0__3.6-3.el9.aarch64",
    sha256 = "2cd63ed497af8a202c79790b04362ba224b50ec7c377abb21901160e4000e07d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libsepol-3.6-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/2cd63ed497af8a202c79790b04362ba224b50ec7c377abb21901160e4000e07d",
    ],
)

rpm(
    name = "libsepol-0__3.6-3.el9.s390x",
    sha256 = "c1246f8553c2aec3ca86721f8bd77fab4f4fcd22527bb6a6e494b4046ee17461",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libsepol-3.6-3.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/c1246f8553c2aec3ca86721f8bd77fab4f4fcd22527bb6a6e494b4046ee17461",
    ],
)

rpm(
    name = "libsepol-0__3.6-3.el9.x86_64",
    sha256 = "6d3d16c3121ccf989f8a123812e524cb1fc098fb01ec9f1c6327544e85aaf84d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsepol-3.6-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/6d3d16c3121ccf989f8a123812e524cb1fc098fb01ec9f1c6327544e85aaf84d",
    ],
)

rpm(
    name = "libsigsegv-0__2.13-4.el9.aarch64",
    sha256 = "097399718ae50fb03fde85fa151c060c50445a1a5af185052cac6b92d6fdcdae",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libsigsegv-2.13-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/097399718ae50fb03fde85fa151c060c50445a1a5af185052cac6b92d6fdcdae",
    ],
)

rpm(
    name = "libsigsegv-0__2.13-4.el9.s390x",
    sha256 = "730c827d66bd292fccdb6f8ac4c29176e7f06283489be41b67f4bf55deeb3ffb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libsigsegv-2.13-4.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/730c827d66bd292fccdb6f8ac4c29176e7f06283489be41b67f4bf55deeb3ffb",
    ],
)

rpm(
    name = "libsigsegv-0__2.13-4.el9.x86_64",
    sha256 = "931bd0ec7050e8c3b37a9bfb489e30af32486a3c77203f1e9113eeceaa3b0a3a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsigsegv-2.13-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/931bd0ec7050e8c3b37a9bfb489e30af32486a3c77203f1e9113eeceaa3b0a3a",
    ],
)

rpm(
    name = "libsmartcols-0__2.37.2-1.el9.aarch64",
    sha256 = "5102aa25f42a101bbc41b9f9286300cdcc863811785e5a4da6ad90d6a1105067",
    urls = ["https://storage.googleapis.com/builddeps/5102aa25f42a101bbc41b9f9286300cdcc863811785e5a4da6ad90d6a1105067"],
)

rpm(
    name = "libsmartcols-0__2.37.2-1.el9.x86_64",
    sha256 = "c62433784604a2e6571e0fcbdd4a2d60f059c5c15624207998c5f03b18d9d382",
    urls = ["https://storage.googleapis.com/builddeps/c62433784604a2e6571e0fcbdd4a2d60f059c5c15624207998c5f03b18d9d382"],
)

rpm(
    name = "libsmartcols-0__2.37.4-27.el9.aarch64",
    sha256 = "f6648e8dd5905217a243accada1bb419898f32b90e4fc26ecbafa0d1a5d8c960",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libsmartcols-2.37.4-27.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f6648e8dd5905217a243accada1bb419898f32b90e4fc26ecbafa0d1a5d8c960",
    ],
)

rpm(
    name = "libsmartcols-0__2.37.4-27.el9.s390x",
    sha256 = "90103d1637ee65ae39b1f1a5172f49b641597d7d41e30d71f593a1bf9e8d5dff",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libsmartcols-2.37.4-27.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/90103d1637ee65ae39b1f1a5172f49b641597d7d41e30d71f593a1bf9e8d5dff",
    ],
)

rpm(
    name = "libsmartcols-0__2.37.4-27.el9.x86_64",
    sha256 = "207adc3397575338e49eb87bc94e9bf67a5600ec1318ee3f9da134cacba74ba7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsmartcols-2.37.4-27.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/207adc3397575338e49eb87bc94e9bf67a5600ec1318ee3f9da134cacba74ba7",
    ],
)

rpm(
    name = "libsmartcols-0__2.40.2-23.el10.aarch64",
    sha256 = "818887fc7ccb430b5c0a6cfcbf41b885fffb8ffdd30855f4110c3a1f12e64c95",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libsmartcols-2.40.2-23.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libsmartcols-0__2.40.2-23.el10.s390x",
    sha256 = "3879e2117cff1ad2e51d6a5fcc8db7f2956731970d5f00114e4dbf8daa5d5512",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libsmartcols-2.40.2-23.el10.s390x.rpm",
    ],
)

rpm(
    name = "libsmartcols-0__2.40.2-23.el10.x86_64",
    sha256 = "3f18c113527dd41cb675b0878ea90e6b4d95f9a0e9b7cb01432a6c620e1b8783",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libsmartcols-2.40.2-23.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libstdc__plus____plus__-0__11.5.0-15.el9.aarch64",
    sha256 = "ff179801d1aebc179103fe94c5b4d2455f1e3efb7b4e0423dd125143fd720d55",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libstdc++-11.5.0-15.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ff179801d1aebc179103fe94c5b4d2455f1e3efb7b4e0423dd125143fd720d55",
    ],
)

rpm(
    name = "libstdc__plus____plus__-0__11.5.0-15.el9.s390x",
    sha256 = "9c0197756b9b25906f65ab12230bf3577e55a934d2a81a257638e956b2bae684",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libstdc++-11.5.0-15.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/9c0197756b9b25906f65ab12230bf3577e55a934d2a81a257638e956b2bae684",
    ],
)

rpm(
    name = "libstdc__plus____plus__-0__11.5.0-15.el9.x86_64",
    sha256 = "aef7b17304d056eb7cbd07ecf8cb75e10de85b010b15905d3127d06bdf045ffe",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libstdc++-11.5.0-15.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/aef7b17304d056eb7cbd07ecf8cb75e10de85b010b15905d3127d06bdf045ffe",
    ],
)

rpm(
    name = "libstdc__plus____plus__-0__14.4.1-4.el10.aarch64",
    sha256 = "23e55fc59fbabb4a9bd6171189e7a4a5d374f272c3c2d47f57deb0cca2ce575f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libstdc++-14.4.1-4.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libstdc__plus____plus__-0__14.4.1-4.el10.s390x",
    sha256 = "ff8cb38bc21a31dcb02388f0ccb2c78c8b779926357ffdf56e3508516097067d",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libstdc++-14.4.1-4.el10.s390x.rpm",
    ],
)

rpm(
    name = "libstdc__plus____plus__-0__14.4.1-4.el10.x86_64",
    sha256 = "c0a386ead9580edd7e19e3dea558eff341f51da15b6e893c62305d9ad860a926",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libstdc++-14.4.1-4.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libtasn1-0__4.16.0-10.el9.aarch64",
    sha256 = "18fee5d9b7dc486f774d1fac61238a6d6ac1a2dbdf61fdc38496838015e61712",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libtasn1-4.16.0-10.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/18fee5d9b7dc486f774d1fac61238a6d6ac1a2dbdf61fdc38496838015e61712",
    ],
)

rpm(
    name = "libtasn1-0__4.16.0-10.el9.s390x",
    sha256 = "ce71d8eb0cfb625616683e3db2db40bcb8bb7506c46dbea6097c2cc2b2d360fe",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libtasn1-4.16.0-10.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/ce71d8eb0cfb625616683e3db2db40bcb8bb7506c46dbea6097c2cc2b2d360fe",
    ],
)

rpm(
    name = "libtasn1-0__4.16.0-10.el9.x86_64",
    sha256 = "05f75ceb9f083ec511756eb9ed4078368c56ad55a6fe0abb819b8948e50b0d90",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libtasn1-4.16.0-10.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/05f75ceb9f083ec511756eb9ed4078368c56ad55a6fe0abb819b8948e50b0d90",
    ],
)

rpm(
    name = "libtasn1-0__4.16.0-7.el9.aarch64",
    sha256 = "4eaa01b044d688793eb928170f3937bc8618b76d702d49a8843aa89461e43fa8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libtasn1-4.16.0-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/4eaa01b044d688793eb928170f3937bc8618b76d702d49a8843aa89461e43fa8",
    ],
)

rpm(
    name = "libtasn1-0__4.16.0-7.el9.x86_64",
    sha256 = "656031558c53da4a5b3ccfd883bd6d55996037891323152b1f07e8d1d5377406",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libtasn1-4.16.0-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/656031558c53da4a5b3ccfd883bd6d55996037891323152b1f07e8d1d5377406",
    ],
)

rpm(
    name = "libtasn1-0__4.20.0-5.el10.aarch64",
    sha256 = "7620003f6a891292015f8db155c6d9c0717deb2f5ebfc30821da519637825ecc",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libtasn1-4.20.0-5.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libtasn1-0__4.20.0-5.el10.s390x",
    sha256 = "04bac1902d0e9dbec5e67637b0952ff8f274b86e0d226fae09351675c7058b6f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libtasn1-4.20.0-5.el10.s390x.rpm",
    ],
)

rpm(
    name = "libtasn1-0__4.20.0-5.el10.x86_64",
    sha256 = "6caa5fabfeda372b6cadce6329b3a9132ccde0cc1d1d2999a1fb968b052cf5e7",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libtasn1-4.20.0-5.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libtool-ltdl-0__2.4.6-46.el9.aarch64",
    sha256 = "4efdb557a6a26e888d976cb15f3eadd8302dc25903df85b8cbfc92e61d7d6d2f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libtool-ltdl-2.4.6-46.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/4efdb557a6a26e888d976cb15f3eadd8302dc25903df85b8cbfc92e61d7d6d2f",
    ],
)

rpm(
    name = "libtool-ltdl-0__2.4.6-46.el9.s390x",
    sha256 = "548a2de100fb988854c4e3e814314eb03c8645f7a6e9f658b61adbed81c8251e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libtool-ltdl-2.4.6-46.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/548a2de100fb988854c4e3e814314eb03c8645f7a6e9f658b61adbed81c8251e",
    ],
)

rpm(
    name = "libtool-ltdl-0__2.4.6-46.el9.x86_64",
    sha256 = "a04d5a4ccd83b8903e2d7fe76208f57636a6ed07f20e0d350a2b1075c15a2147",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libtool-ltdl-2.4.6-46.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/a04d5a4ccd83b8903e2d7fe76208f57636a6ed07f20e0d350a2b1075c15a2147",
    ],
)

rpm(
    name = "libunistring-0__0.9.10-15.el9.aarch64",
    sha256 = "09381b23c9d2343592b8b565dcbb23d055999ab1e521aa802b6d40a682b80e42",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libunistring-0.9.10-15.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/09381b23c9d2343592b8b565dcbb23d055999ab1e521aa802b6d40a682b80e42",
    ],
)

rpm(
    name = "libunistring-0__0.9.10-15.el9.s390x",
    sha256 = "029cedc9f79dcc145f59e2bbf2121d406b3853765d56345a75bc987760d5d2d2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libunistring-0.9.10-15.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/029cedc9f79dcc145f59e2bbf2121d406b3853765d56345a75bc987760d5d2d2",
    ],
)

rpm(
    name = "libunistring-0__0.9.10-15.el9.x86_64",
    sha256 = "11e736e44265d2d0ca0afa4c11cfe0856553c4124e534fb616e6ab61c9b59e46",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libunistring-0.9.10-15.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/11e736e44265d2d0ca0afa4c11cfe0856553c4124e534fb616e6ab61c9b59e46",
    ],
)

rpm(
    name = "libunistring-0__1.1-10.el10.aarch64",
    sha256 = "aa793b61f51cb8727c37520bc4b261845831b9a5789649a798c4e8a2cc207f4f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libunistring-1.1-10.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libunistring-0__1.1-10.el10.s390x",
    sha256 = "9c45ddec6ffa51201a570b9881e9277b8f22c8eb40ee62b45d3c2b86bdc8eeac",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libunistring-1.1-10.el10.s390x.rpm",
    ],
)

rpm(
    name = "libunistring-0__1.1-10.el10.x86_64",
    sha256 = "603c06593a43f5766a53588d9ba18855ddb7b238963b8e09d8a328a17959b774",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libunistring-1.1-10.el10.x86_64.rpm",
    ],
)

rpm(
    name = "liburing-0__2.12-1.el10.aarch64",
    sha256 = "29f16a2950ef7ddaae31d7806d98961bf1f7d1772623782ae45cc687a3980c62",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/liburing-2.12-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "liburing-0__2.12-1.el10.s390x",
    sha256 = "53f3015878c4044a13caf8060a4afa6c10aff93452aa4b8de84cd2374d456a51",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/liburing-2.12-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "liburing-0__2.12-1.el10.x86_64",
    sha256 = "133309fc854ab7859713d7944e5a14e8cbc3f3916bbcd9f9e6af4d4850424c15",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/liburing-2.12-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "liburing-0__2.12-1.el9.aarch64",
    sha256 = "7b99b8c28e8cf9a7d355231207e6151cc3b98cd722682359fff41737744d35d0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/liburing-2.12-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/7b99b8c28e8cf9a7d355231207e6151cc3b98cd722682359fff41737744d35d0",
    ],
)

rpm(
    name = "liburing-0__2.12-1.el9.s390x",
    sha256 = "b259bcadc7623840495a33d9dabec62511a0f2133b731d070b59c5df60e8f7c6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/liburing-2.12-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/b259bcadc7623840495a33d9dabec62511a0f2133b731d070b59c5df60e8f7c6",
    ],
)

rpm(
    name = "liburing-0__2.12-1.el9.x86_64",
    sha256 = "49b44a2192b8a3f3184d0ca80c318aa9852dddda391b66e7c38c53f900a08ce4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/liburing-2.12-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/49b44a2192b8a3f3184d0ca80c318aa9852dddda391b66e7c38c53f900a08ce4",
    ],
)

rpm(
    name = "libutempter-0__1.2.1-6.el9.aarch64",
    sha256 = "65cd8c3813afc69dd2ea9eeb6e2fc7db4a7d626b51efe376b8000dfdaa10402a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libutempter-1.2.1-6.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/65cd8c3813afc69dd2ea9eeb6e2fc7db4a7d626b51efe376b8000dfdaa10402a",
    ],
)

rpm(
    name = "libutempter-0__1.2.1-6.el9.s390x",
    sha256 = "6c000dac4305215beb37c8931a85ee137806f06547ecfb9a23e1915f01a3baa2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libutempter-1.2.1-6.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/6c000dac4305215beb37c8931a85ee137806f06547ecfb9a23e1915f01a3baa2",
    ],
)

rpm(
    name = "libutempter-0__1.2.1-6.el9.x86_64",
    sha256 = "fab361a9cba04490fd8b5664049983d1e57ebf7c1080804726ba600708524125",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libutempter-1.2.1-6.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/fab361a9cba04490fd8b5664049983d1e57ebf7c1080804726ba600708524125",
    ],
)

rpm(
    name = "libuuid-0__2.37.2-1.el9.aarch64",
    sha256 = "49e914c5f068ded96c050fd66c1110ec77f703369b9f0b08d85f80b822b1431d",
    urls = ["https://storage.googleapis.com/builddeps/49e914c5f068ded96c050fd66c1110ec77f703369b9f0b08d85f80b822b1431d"],
)

rpm(
    name = "libuuid-0__2.37.2-1.el9.x86_64",
    sha256 = "ffd8317ccc6f80524b7bf15a8157d82f36a2b9c7478bb04eb4a34c18d019e6fa",
    urls = ["https://storage.googleapis.com/builddeps/ffd8317ccc6f80524b7bf15a8157d82f36a2b9c7478bb04eb4a34c18d019e6fa"],
)

rpm(
    name = "libuuid-0__2.37.4-27.el9.aarch64",
    sha256 = "4fe31a3a82650ab3192e1c81351a5591d2d7e057798b2f759714e85741fbb3a5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libuuid-2.37.4-27.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/4fe31a3a82650ab3192e1c81351a5591d2d7e057798b2f759714e85741fbb3a5",
    ],
)

rpm(
    name = "libuuid-0__2.37.4-27.el9.s390x",
    sha256 = "effba51dcf357dc7352ade83de53ef4835895c55bb9ade09bd1dcc978c6538e0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libuuid-2.37.4-27.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/effba51dcf357dc7352ade83de53ef4835895c55bb9ade09bd1dcc978c6538e0",
    ],
)

rpm(
    name = "libuuid-0__2.37.4-27.el9.x86_64",
    sha256 = "2e340e7920662e24867f38f624b199102384f9142010a0d10223ee5acc31a0d5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libuuid-2.37.4-27.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/2e340e7920662e24867f38f624b199102384f9142010a0d10223ee5acc31a0d5",
    ],
)

rpm(
    name = "libuuid-0__2.40.2-23.el10.aarch64",
    sha256 = "85741ec1b4098862450b2cd0decd9f17c392dda4a93455516fa02b2abc326d38",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libuuid-2.40.2-23.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libuuid-0__2.40.2-23.el10.s390x",
    sha256 = "dd2b3a79bfb0a37e467f1f4d86d66be746da52b9fd7ac0ee8d2bc5739374ec75",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libuuid-2.40.2-23.el10.s390x.rpm",
    ],
)

rpm(
    name = "libuuid-0__2.40.2-23.el10.x86_64",
    sha256 = "36a04c0f176e602dc44770fe7fa8c8ca09055bb9b55815cf2809376d344204e8",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libuuid-2.40.2-23.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libverto-0__0.3.2-10.el10.aarch64",
    sha256 = "0583db7823a8f33a1e09db1e4aa389c10bc98b58de3bd985b6f67be5351d814a",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libverto-0.3.2-10.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libverto-0__0.3.2-10.el10.s390x",
    sha256 = "80757eae2999d4dbc8975747eb4d8fdfb64b144826ba58215672a0f34d313228",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libverto-0.3.2-10.el10.s390x.rpm",
    ],
)

rpm(
    name = "libverto-0__0.3.2-10.el10.x86_64",
    sha256 = "52777e532dc2351c83b72b5033c40df20494afb6504100f7413a65f74368c284",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libverto-0.3.2-10.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libverto-0__0.3.2-3.el9.aarch64",
    sha256 = "1190ea8310b0dab3ebbade3180b4c2cf7064e90c894e5415711d7751e709be8a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libverto-0.3.2-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/1190ea8310b0dab3ebbade3180b4c2cf7064e90c894e5415711d7751e709be8a",
    ],
)

rpm(
    name = "libverto-0__0.3.2-3.el9.s390x",
    sha256 = "3d794c924cc3611f1b37033d6835c4af71a555fcba053618bd6d48ad79547ab0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libverto-0.3.2-3.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/3d794c924cc3611f1b37033d6835c4af71a555fcba053618bd6d48ad79547ab0",
    ],
)

rpm(
    name = "libverto-0__0.3.2-3.el9.x86_64",
    sha256 = "c55578b84f169c4ed79b2d50ea03fd1817007e35062c9fe7a58e6cad025f3b24",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libverto-0.3.2-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c55578b84f169c4ed79b2d50ea03fd1817007e35062c9fe7a58e6cad025f3b24",
    ],
)

rpm(
    name = "libxcrypt-0__4.4.18-3.el9.aarch64",
    sha256 = "f697d91abb19e9be9b69b8836a802711d2cf7989af27a4e1ba261f35ce53b8b5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libxcrypt-4.4.18-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f697d91abb19e9be9b69b8836a802711d2cf7989af27a4e1ba261f35ce53b8b5",
    ],
)

rpm(
    name = "libxcrypt-0__4.4.18-3.el9.s390x",
    sha256 = "dd9d51f68ae799b41cbe4cc00945280c65ed0c098b72f79d8d39a5c462b37074",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libxcrypt-4.4.18-3.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/dd9d51f68ae799b41cbe4cc00945280c65ed0c098b72f79d8d39a5c462b37074",
    ],
)

rpm(
    name = "libxcrypt-0__4.4.18-3.el9.x86_64",
    sha256 = "97e88678b420f619a44608fff30062086aa1dd6931ecbd54f21bba005ff1de1a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libxcrypt-4.4.18-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/97e88678b420f619a44608fff30062086aa1dd6931ecbd54f21bba005ff1de1a",
    ],
)

rpm(
    name = "libxcrypt-0__4.4.36-10.el10.aarch64",
    sha256 = "465ade16c8f369b5abc1a39671f882bc645ac90b1aeaa29cdfc3958e57640144",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libxcrypt-4.4.36-10.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libxcrypt-0__4.4.36-10.el10.s390x",
    sha256 = "d14c5523dd6c7f233277acbbb11fb2644f26e91da18e6184ae6ad445e3835a36",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libxcrypt-4.4.36-10.el10.s390x.rpm",
    ],
)

rpm(
    name = "libxcrypt-0__4.4.36-10.el10.x86_64",
    sha256 = "503a29c4c767637d810c7e89ed4355fe0b588381cb360517585fb56a2cf5ee46",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libxcrypt-4.4.36-10.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libxcrypt-compat-0__4.4.18-3.el9.x86_64",
    sha256 = "3ea916c72412d3a7efd8c70cfa1ed18863c018091001b631390b19c454136b87",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libxcrypt-compat-4.4.18-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/3ea916c72412d3a7efd8c70cfa1ed18863c018091001b631390b19c454136b87",
    ],
)

rpm(
    name = "libxcrypt-compat-0__4.4.36-10.el10.x86_64",
    sha256 = "dfe46d3b27dd4e67251607f0fa8e24a7ff567ab480e27db4906760ef4d41fae6",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/x86_64/os/Packages/libxcrypt-compat-4.4.36-10.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libxml2-0__2.12.5-16.el10.aarch64",
    sha256 = "d563c0f91dbf542679bb9ddf27b604f38dc0623bc6c04c454ce3abdbba524748",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libxml2-2.12.5-16.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libxml2-0__2.12.5-16.el10.s390x",
    sha256 = "25e589425b680d598b8e5008683aab25dd28ea1dd24cbd56e437d8bd616346fe",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libxml2-2.12.5-16.el10.s390x.rpm",
    ],
)

rpm(
    name = "libxml2-0__2.12.5-16.el10.x86_64",
    sha256 = "33597e81f428d42f4b6c6eeac7c58087c298c23618fab92007d95cd8102667f7",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libxml2-2.12.5-16.el10.x86_64.rpm",
    ],
)

rpm(
    name = "libxml2-0__2.9.13-16.el9.aarch64",
    sha256 = "aadb7ca54b54a4d976a6ebb9c6a780e74d33f294a71f9bfb808a3f6a89d5f2f6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libxml2-2.9.13-16.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/aadb7ca54b54a4d976a6ebb9c6a780e74d33f294a71f9bfb808a3f6a89d5f2f6",
    ],
)

rpm(
    name = "libxml2-0__2.9.13-16.el9.s390x",
    sha256 = "f33487680484e3a9bc67b939862b9f0ad9872db5ab29d33f53d0ee582c21f4ce",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libxml2-2.9.13-16.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/f33487680484e3a9bc67b939862b9f0ad9872db5ab29d33f53d0ee582c21f4ce",
    ],
)

rpm(
    name = "libxml2-0__2.9.13-16.el9.x86_64",
    sha256 = "66a9bffc0993810538f3532a1964d56e7a073c128a9dbe8e394bbe56cd29f5d6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libxml2-2.9.13-16.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/66a9bffc0993810538f3532a1964d56e7a073c128a9dbe8e394bbe56cd29f5d6",
    ],
)

rpm(
    name = "libzstd-0__1.5.1-2.el9.aarch64",
    sha256 = "68101e014106305c840611b64d71311600edb30a34e09514c169c9eef6090d42",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libzstd-1.5.1-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/68101e014106305c840611b64d71311600edb30a34e09514c169c9eef6090d42",
    ],
)

rpm(
    name = "libzstd-0__1.5.1-2.el9.x86_64",
    sha256 = "0840678cb3c1b418286f55da6973df9468c4cf500192de82d05ef28e6b4215a0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libzstd-1.5.1-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/0840678cb3c1b418286f55da6973df9468c4cf500192de82d05ef28e6b4215a0",
    ],
)

rpm(
    name = "libzstd-0__1.5.5-1.el9.aarch64",
    sha256 = "49fb3a1052d9f50abb9ad3f0ab4ed186b2c0bb51fcb04883702fbc362d116108",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libzstd-1.5.5-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/49fb3a1052d9f50abb9ad3f0ab4ed186b2c0bb51fcb04883702fbc362d116108",
    ],
)

rpm(
    name = "libzstd-0__1.5.5-1.el9.s390x",
    sha256 = "720ce927a447b6c9fd2479ecb924112d450ec9b4f927090b36ef34b10ad4b163",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libzstd-1.5.5-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/720ce927a447b6c9fd2479ecb924112d450ec9b4f927090b36ef34b10ad4b163",
    ],
)

rpm(
    name = "libzstd-0__1.5.5-1.el9.x86_64",
    sha256 = "3439a7437a4b47ef4b6efbcd8c5862180fb281dd956d70a4ffe3764fd8d997dd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libzstd-1.5.5-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/3439a7437a4b47ef4b6efbcd8c5862180fb281dd956d70a4ffe3764fd8d997dd",
    ],
)

rpm(
    name = "libzstd-0__1.5.5-9.el10.aarch64",
    sha256 = "474a4497b7901176be4a59895cd02bba744300fd673668ef068bd1dfc5e129c7",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/libzstd-1.5.5-9.el10.aarch64.rpm",
    ],
)

rpm(
    name = "libzstd-0__1.5.5-9.el10.s390x",
    sha256 = "59d29a77a5792bbc4ce42b3ac700a1df776ace058e040f391374f011d39f0eef",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/libzstd-1.5.5-9.el10.s390x.rpm",
    ],
)

rpm(
    name = "libzstd-0__1.5.5-9.el10.x86_64",
    sha256 = "86f3cb406d56283119c45ec8c1f4689aa37ff6c04cf44f6608c10cfdcccdb2c1",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/libzstd-1.5.5-9.el10.x86_64.rpm",
    ],
)

rpm(
    name = "lz4-libs-0__1.9.3-5.el9.aarch64",
    sha256 = "9aa14d26393dd46c0a390cf04f939f7f759a33165bdb506f8bee0653f3b70f45",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/lz4-libs-1.9.3-5.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/9aa14d26393dd46c0a390cf04f939f7f759a33165bdb506f8bee0653f3b70f45",
    ],
)

rpm(
    name = "lz4-libs-0__1.9.3-5.el9.s390x",
    sha256 = "358c7c19e9ec8778874066342c591b71877c3324f0727357342dffb4e1ec3498",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/lz4-libs-1.9.3-5.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/358c7c19e9ec8778874066342c591b71877c3324f0727357342dffb4e1ec3498",
    ],
)

rpm(
    name = "lz4-libs-0__1.9.3-5.el9.x86_64",
    sha256 = "cba6a63054d070956a182e33269ee245bcfbe87e3e605c27816519db762a66ad",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/lz4-libs-1.9.3-5.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/cba6a63054d070956a182e33269ee245bcfbe87e3e605c27816519db762a66ad",
    ],
)

rpm(
    name = "mpdecimal-0__2.5.1-12.el10.aarch64",
    sha256 = "f7755f98208b3f400c950ba46acf568f113029893fede5770d19eedadfa0b3ea",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/mpdecimal-2.5.1-12.el10.aarch64.rpm",
    ],
)

rpm(
    name = "mpdecimal-0__2.5.1-12.el10.s390x",
    sha256 = "2dd0dbab48a3481fab6bcb4554b0854cb66c8d142ef28b8e97b7dfc96d4c2c93",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/mpdecimal-2.5.1-12.el10.s390x.rpm",
    ],
)

rpm(
    name = "mpdecimal-0__2.5.1-12.el10.x86_64",
    sha256 = "7d1762e4770170efa93ff4f7e07cf523f62d3e3378f50d87d7b307cd8a73ee77",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/mpdecimal-2.5.1-12.el10.x86_64.rpm",
    ],
)

rpm(
    name = "mpfr-0__4.1.0-10.el9.aarch64",
    sha256 = "bea56ccc46a2a14f3f2c8d9624675abc135e4f002e87c76541784b047d51764d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/mpfr-4.1.0-10.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/bea56ccc46a2a14f3f2c8d9624675abc135e4f002e87c76541784b047d51764d",
    ],
)

rpm(
    name = "mpfr-0__4.1.0-10.el9.s390x",
    sha256 = "b166f1d2ae951d053a5761c826cd5bd8735412e465ce7cbfe78b1292c27aa10e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/mpfr-4.1.0-10.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/b166f1d2ae951d053a5761c826cd5bd8735412e465ce7cbfe78b1292c27aa10e",
    ],
)

rpm(
    name = "mpfr-0__4.1.0-10.el9.x86_64",
    sha256 = "11c1d6b33b7e64ddc40faf45b949618c829bd2e3d3661132417e4c8aee6ab0fd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/mpfr-4.1.0-10.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/11c1d6b33b7e64ddc40faf45b949618c829bd2e3d3661132417e4c8aee6ab0fd",
    ],
)

rpm(
    name = "mpfr-0__4.1.0-7.el9.aarch64",
    sha256 = "f3bd8510505a53450abe05dc34edbc5313fe89a6f88d0252624205dc7bb884c7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/mpfr-4.1.0-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f3bd8510505a53450abe05dc34edbc5313fe89a6f88d0252624205dc7bb884c7",
    ],
)

rpm(
    name = "mpfr-0__4.1.0-7.el9.x86_64",
    sha256 = "179760104aa5a31ca463c586d0f21f380ba4d0eed212eee91bd1ca513e5d7a8d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/mpfr-4.1.0-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/179760104aa5a31ca463c586d0f21f380ba4d0eed212eee91bd1ca513e5d7a8d",
    ],
)

rpm(
    name = "mpfr-0__4.2.1-8.el10.aarch64",
    sha256 = "df1662c3221a03d86963ca2f2b4db1b745493ce805e95fa07ebde2a8df7b3628",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/mpfr-4.2.1-8.el10.aarch64.rpm",
    ],
)

rpm(
    name = "mpfr-0__4.2.1-8.el10.s390x",
    sha256 = "3b4858c266478b5173b90c650c9d0bbe7a9f84c31bde1fb85ac709051531f98f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/mpfr-4.2.1-8.el10.s390x.rpm",
    ],
)

rpm(
    name = "mpfr-0__4.2.1-8.el10.x86_64",
    sha256 = "351ac05205dd7c29daecbeeb7e53bab4a130060d1d1d1f610097d8b5fc30289f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/mpfr-4.2.1-8.el10.x86_64.rpm",
    ],
)

rpm(
    name = "nbdkit-basic-filters-0__1.38.5-12.el9.aarch64",
    sha256 = "8ef88cfc7c4f2b9687508f1e45d6e7819797e9b717eed0ae582e73e9f8b7cd5b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-basic-filters-1.38.5-12.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/8ef88cfc7c4f2b9687508f1e45d6e7819797e9b717eed0ae582e73e9f8b7cd5b",
    ],
)

rpm(
    name = "nbdkit-basic-filters-0__1.38.5-12.el9.s390x",
    sha256 = "9a0a15a8a94aff575a900243dd0c63630a9d8a1079222a512faddcbdb0ddb560",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nbdkit-basic-filters-1.38.5-12.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/9a0a15a8a94aff575a900243dd0c63630a9d8a1079222a512faddcbdb0ddb560",
    ],
)

rpm(
    name = "nbdkit-basic-filters-0__1.38.5-12.el9.x86_64",
    sha256 = "05781b562f330e9299a455788c07b86a5f18204857b65a23f583caccc3318f6c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-basic-filters-1.38.5-12.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/05781b562f330e9299a455788c07b86a5f18204857b65a23f583caccc3318f6c",
    ],
)

rpm(
    name = "nbdkit-basic-filters-0__1.47.6-2.el10.aarch64",
    sha256 = "d54014c4dd9255a75b0151819de097cbebc29eecce23da722f2137129e253035",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/aarch64/os/Packages/nbdkit-basic-filters-1.47.6-2.el10.aarch64.rpm",
    ],
)

rpm(
    name = "nbdkit-basic-filters-0__1.47.6-2.el10.s390x",
    sha256 = "821423e5555822597c07a1f15168b76d2d19ee7d8753ba6d110362fe4ca0b4fa",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/s390x/os/Packages/nbdkit-basic-filters-1.47.6-2.el10.s390x.rpm",
    ],
)

rpm(
    name = "nbdkit-basic-filters-0__1.47.6-2.el10.x86_64",
    sha256 = "776f9024f43106498d037de24371de86429137525e1992aacea82054820daa45",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/x86_64/os/Packages/nbdkit-basic-filters-1.47.6-2.el10.x86_64.rpm",
    ],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.38.5-12.el9.aarch64",
    sha256 = "29244acc7e52b570fa0799b3f25984fd271ca25f059768af9ae01de1e7b800ee",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-curl-plugin-1.38.5-12.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/29244acc7e52b570fa0799b3f25984fd271ca25f059768af9ae01de1e7b800ee",
    ],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.38.5-12.el9.s390x",
    sha256 = "7ed8101daada49760a999d201ebeac6df545a3070e82d280fafe5bd69b2159ab",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nbdkit-curl-plugin-1.38.5-12.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/7ed8101daada49760a999d201ebeac6df545a3070e82d280fafe5bd69b2159ab",
    ],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.38.5-12.el9.x86_64",
    sha256 = "0f21b320f4165f5189b9cab2deaa812ca751d4302e91a629da485a21aa9f48d3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-curl-plugin-1.38.5-12.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/0f21b320f4165f5189b9cab2deaa812ca751d4302e91a629da485a21aa9f48d3",
    ],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.47.6-2.el10.aarch64",
    sha256 = "602a29a67f7f06cd776f9dc7ae1418d8c3ccad801dfac500f74f69b1fcc1fa29",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/aarch64/os/Packages/nbdkit-curl-plugin-1.47.6-2.el10.aarch64.rpm",
    ],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.47.6-2.el10.s390x",
    sha256 = "a4acebc04ad5b9ba80e17e8cd235e6e8531f10eeeaa2ba55894fedf0100f0d79",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/s390x/os/Packages/nbdkit-curl-plugin-1.47.6-2.el10.s390x.rpm",
    ],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.47.6-2.el10.x86_64",
    sha256 = "90125e41cac7fe2359e5e247a4ead2dbe984fc8d345d81445ec46edc56ff5603",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/x86_64/os/Packages/nbdkit-curl-plugin-1.47.6-2.el10.x86_64.rpm",
    ],
)

rpm(
    name = "nbdkit-gzip-filter-0__1.38.5-12.el9.aarch64",
    sha256 = "25c2b893939e6fad021e78ce64fd9a1d29e96dd3dd713b01b765d55d485ce3ae",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-gzip-filter-1.38.5-12.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/25c2b893939e6fad021e78ce64fd9a1d29e96dd3dd713b01b765d55d485ce3ae",
    ],
)

rpm(
    name = "nbdkit-gzip-filter-0__1.38.5-12.el9.s390x",
    sha256 = "a5db148cc43f5dce23bd4c7ea6760fe24db61b09f67916e4aa8da78bc2b9a8ce",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nbdkit-gzip-filter-1.38.5-12.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/a5db148cc43f5dce23bd4c7ea6760fe24db61b09f67916e4aa8da78bc2b9a8ce",
    ],
)

rpm(
    name = "nbdkit-gzip-filter-0__1.38.5-12.el9.x86_64",
    sha256 = "b2d66b22038e505860864367510cc534d7d0ed2ebecc63ed1c264487f39e67b7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-gzip-filter-1.38.5-12.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/b2d66b22038e505860864367510cc534d7d0ed2ebecc63ed1c264487f39e67b7",
    ],
)

rpm(
    name = "nbdkit-server-0__1.38.5-12.el9.aarch64",
    sha256 = "ece68648c7fc85efe3ac4f7d3195f476adf0ef55a08a7e5884091f20f73e42b1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-server-1.38.5-12.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ece68648c7fc85efe3ac4f7d3195f476adf0ef55a08a7e5884091f20f73e42b1",
    ],
)

rpm(
    name = "nbdkit-server-0__1.38.5-12.el9.s390x",
    sha256 = "f2aa2c5accdb4549a2fdcdd1fdf74b7c7ae5a41f0d5929999ca0d55ecf87032f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nbdkit-server-1.38.5-12.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/f2aa2c5accdb4549a2fdcdd1fdf74b7c7ae5a41f0d5929999ca0d55ecf87032f",
    ],
)

rpm(
    name = "nbdkit-server-0__1.38.5-12.el9.x86_64",
    sha256 = "1c9088b3a948c530d3ad5ef42a0136f4a8c9ea4d4ec843a0b19c2feb60ad9f1b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-server-1.38.5-12.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/1c9088b3a948c530d3ad5ef42a0136f4a8c9ea4d4ec843a0b19c2feb60ad9f1b",
    ],
)

rpm(
    name = "nbdkit-server-0__1.47.6-2.el10.aarch64",
    sha256 = "4baae3de9822b37c487658724324cff7f79b6f96dbe42a4a45e407dc3e3c2310",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/aarch64/os/Packages/nbdkit-server-1.47.6-2.el10.aarch64.rpm",
    ],
)

rpm(
    name = "nbdkit-server-0__1.47.6-2.el10.s390x",
    sha256 = "4790efb0345962c8662236de94e72b901b579d835b0e75f92ffbe365fdfaa0a1",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/s390x/os/Packages/nbdkit-server-1.47.6-2.el10.s390x.rpm",
    ],
)

rpm(
    name = "nbdkit-server-0__1.47.6-2.el10.x86_64",
    sha256 = "33d748b94166e9a4851a42210baf21ac4f4c145b08170ede6ca6bb97b5383fa8",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/x86_64/os/Packages/nbdkit-server-1.47.6-2.el10.x86_64.rpm",
    ],
)

rpm(
    name = "nbdkit-vddk-plugin-0__1.38.5-12.el9.x86_64",
    sha256 = "370ba01c836494abd280e4490a8d7a5888e97f741e2f71fe5112a12ace71e6e0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-vddk-plugin-1.38.5-12.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/370ba01c836494abd280e4490a8d7a5888e97f741e2f71fe5112a12ace71e6e0",
    ],
)

rpm(
    name = "nbdkit-vddk-plugin-0__1.47.6-2.el10.x86_64",
    sha256 = "b5432b04cd8dd5d8e201b2d60fb10692f59f264c1ee14b075c99a642dd85ddcf",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/x86_64/os/Packages/nbdkit-vddk-plugin-1.47.6-2.el10.x86_64.rpm",
    ],
)

rpm(
    name = "nbdkit-xz-filter-0__1.38.5-12.el9.aarch64",
    sha256 = "14971bfa1c8dbb27c3643e38958e66089dd9326c404fb26c99160807f376b279",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-xz-filter-1.38.5-12.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/14971bfa1c8dbb27c3643e38958e66089dd9326c404fb26c99160807f376b279",
    ],
)

rpm(
    name = "nbdkit-xz-filter-0__1.38.5-12.el9.s390x",
    sha256 = "47c1b21205dde8305e07762658cf3b2f2345b9e1684b984d3e3df888bfb9b069",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nbdkit-xz-filter-1.38.5-12.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/47c1b21205dde8305e07762658cf3b2f2345b9e1684b984d3e3df888bfb9b069",
    ],
)

rpm(
    name = "nbdkit-xz-filter-0__1.38.5-12.el9.x86_64",
    sha256 = "2a61bf916bc0b31b7d7037bc555c9a0604e430c2276e09fd98003d56000e9862",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-xz-filter-1.38.5-12.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/2a61bf916bc0b31b7d7037bc555c9a0604e430c2276e09fd98003d56000e9862",
    ],
)

rpm(
    name = "nbdkit-xz-filter-0__1.47.6-2.el10.aarch64",
    sha256 = "2db8baa122c9b012de6df3b9fdfadf8ad8933eb34a1d6ccf8a8a7af5b7f4444c",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/aarch64/os/Packages/nbdkit-xz-filter-1.47.6-2.el10.aarch64.rpm",
    ],
)

rpm(
    name = "nbdkit-xz-filter-0__1.47.6-2.el10.s390x",
    sha256 = "5b20d296d677d2a1289d9644a46048cc49d8c906c8e27b5d082578e440f38597",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/s390x/os/Packages/nbdkit-xz-filter-1.47.6-2.el10.s390x.rpm",
    ],
)

rpm(
    name = "nbdkit-xz-filter-0__1.47.6-2.el10.x86_64",
    sha256 = "652833e12bc131e5bfbc27bd6810159c62b8c035a7bb88dca6a59cd9dc68f790",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/x86_64/os/Packages/nbdkit-xz-filter-1.47.6-2.el10.x86_64.rpm",
    ],
)

rpm(
    name = "ncurses-base-0__6.2-12.20210508.el9.aarch64",
    sha256 = "49f6470fa7dd1b3ba81ccdd0547b29953af2835e067de915eeca3c45d5faa339",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/ncurses-base-6.2-12.20210508.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/49f6470fa7dd1b3ba81ccdd0547b29953af2835e067de915eeca3c45d5faa339",
    ],
)

rpm(
    name = "ncurses-base-0__6.2-12.20210508.el9.s390x",
    sha256 = "49f6470fa7dd1b3ba81ccdd0547b29953af2835e067de915eeca3c45d5faa339",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/ncurses-base-6.2-12.20210508.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/49f6470fa7dd1b3ba81ccdd0547b29953af2835e067de915eeca3c45d5faa339",
    ],
)

rpm(
    name = "ncurses-base-0__6.2-12.20210508.el9.x86_64",
    sha256 = "49f6470fa7dd1b3ba81ccdd0547b29953af2835e067de915eeca3c45d5faa339",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ncurses-base-6.2-12.20210508.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/49f6470fa7dd1b3ba81ccdd0547b29953af2835e067de915eeca3c45d5faa339",
    ],
)

rpm(
    name = "ncurses-base-0__6.2-8.20210508.el9.aarch64",
    sha256 = "e4cc4a4a479b8c27776debba5c20e8ef21dc4b513da62a25ed09f88386ac08a8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/ncurses-base-6.2-8.20210508.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/e4cc4a4a479b8c27776debba5c20e8ef21dc4b513da62a25ed09f88386ac08a8",
    ],
)

rpm(
    name = "ncurses-base-0__6.2-8.20210508.el9.x86_64",
    sha256 = "e4cc4a4a479b8c27776debba5c20e8ef21dc4b513da62a25ed09f88386ac08a8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ncurses-base-6.2-8.20210508.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/e4cc4a4a479b8c27776debba5c20e8ef21dc4b513da62a25ed09f88386ac08a8",
    ],
)

rpm(
    name = "ncurses-base-0__6.4-15.20240127.el10.aarch64",
    sha256 = "9feb09b8a1f86ca76ed7dd6f66476b05fde5084757501cf24be4b8e836a99693",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/ncurses-base-6.4-15.20240127.el10.noarch.rpm",
    ],
)

rpm(
    name = "ncurses-base-0__6.4-15.20240127.el10.s390x",
    sha256 = "9feb09b8a1f86ca76ed7dd6f66476b05fde5084757501cf24be4b8e836a99693",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/ncurses-base-6.4-15.20240127.el10.noarch.rpm",
    ],
)

rpm(
    name = "ncurses-base-0__6.4-15.20240127.el10.x86_64",
    sha256 = "9feb09b8a1f86ca76ed7dd6f66476b05fde5084757501cf24be4b8e836a99693",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/ncurses-base-6.4-15.20240127.el10.noarch.rpm",
    ],
)

rpm(
    name = "ncurses-libs-0__6.2-12.20210508.el9.aarch64",
    sha256 = "7b61d1dab8d4113a6ad015c083ac3053ec9db1f2503527d547ba7c741d54e57a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/ncurses-libs-6.2-12.20210508.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/7b61d1dab8d4113a6ad015c083ac3053ec9db1f2503527d547ba7c741d54e57a",
    ],
)

rpm(
    name = "ncurses-libs-0__6.2-12.20210508.el9.s390x",
    sha256 = "d2a6307a398b9cde8f0a83fff92c3b31f5f6c4c15f911f64ff84168a7cd060a4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/ncurses-libs-6.2-12.20210508.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/d2a6307a398b9cde8f0a83fff92c3b31f5f6c4c15f911f64ff84168a7cd060a4",
    ],
)

rpm(
    name = "ncurses-libs-0__6.2-12.20210508.el9.x86_64",
    sha256 = "7b396883232158d4f9a6977bcd72b5e6f7fa6bc34a51030379833d4c0d24ab6f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ncurses-libs-6.2-12.20210508.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/7b396883232158d4f9a6977bcd72b5e6f7fa6bc34a51030379833d4c0d24ab6f",
    ],
)

rpm(
    name = "ncurses-libs-0__6.2-8.20210508.el9.aarch64",
    sha256 = "26a21395b0bb4f7b60ab89bacaa8fc210c9921f1aba90ec950b91b3ee9e25dcc",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/ncurses-libs-6.2-8.20210508.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/26a21395b0bb4f7b60ab89bacaa8fc210c9921f1aba90ec950b91b3ee9e25dcc",
    ],
)

rpm(
    name = "ncurses-libs-0__6.2-8.20210508.el9.x86_64",
    sha256 = "328f4d50e66b00f24344ebe239817204fda8e68b1d988c6943abb3c36231beaa",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ncurses-libs-6.2-8.20210508.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/328f4d50e66b00f24344ebe239817204fda8e68b1d988c6943abb3c36231beaa",
    ],
)

rpm(
    name = "ncurses-libs-0__6.4-15.20240127.el10.aarch64",
    sha256 = "3436974a653ed58ff9bb3fe4cab25f3b2b81e672b5e034cc8c951ccd947f3257",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/ncurses-libs-6.4-15.20240127.el10.aarch64.rpm",
    ],
)

rpm(
    name = "ncurses-libs-0__6.4-15.20240127.el10.s390x",
    sha256 = "1fa5412403feb16c29c7558a9a7ee39e0332ea31b28134dfe6c7110294b78a68",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/ncurses-libs-6.4-15.20240127.el10.s390x.rpm",
    ],
)

rpm(
    name = "ncurses-libs-0__6.4-15.20240127.el10.x86_64",
    sha256 = "2bd5fc0a9343904137f1d2fdf6188d0f40d72971810dabf8fb1052f7b3915095",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/ncurses-libs-6.4-15.20240127.el10.x86_64.rpm",
    ],
)

rpm(
    name = "netavark-3__1.17.2-3.el9.aarch64",
    sha256 = "c214873c1b2dd15261b43895700a4cff26e1af4ab432f8d2e6a9c13ddfeb92a9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/netavark-1.17.2-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c214873c1b2dd15261b43895700a4cff26e1af4ab432f8d2e6a9c13ddfeb92a9",
    ],
)

rpm(
    name = "netavark-3__1.17.2-3.el9.s390x",
    sha256 = "ded39d4791a8245986e82a9a27ba448dccfa922cbc30db5b4c0ec5846dff1f65",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/netavark-1.17.2-3.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/ded39d4791a8245986e82a9a27ba448dccfa922cbc30db5b4c0ec5846dff1f65",
    ],
)

rpm(
    name = "netavark-3__1.17.2-3.el9.x86_64",
    sha256 = "f3e2a002e4c360c7cf8de7792b2d3be8df99929c3f21276ff547267ce03f0289",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/netavark-1.17.2-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f3e2a002e4c360c7cf8de7792b2d3be8df99929c3f21276ff547267ce03f0289",
    ],
)

rpm(
    name = "netavark-4__2.1.0-1.el10.aarch64",
    sha256 = "fe583bf348de058b53aed1ab1b0eb9d58612b3f2c9efc8c9914781c207e31a13",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/netavark-2.1.0-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "netavark-4__2.1.0-1.el10.s390x",
    sha256 = "4463329c21ba4e335f8d8ca6aa6a3774a1e785f02aba4f0747b8e8442398b047",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/netavark-2.1.0-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "netavark-4__2.1.0-1.el10.x86_64",
    sha256 = "f910bd097e2aa9a79f9817bc52f012440bf3d283c68724d4e6d36abb09aa86df",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/netavark-2.1.0-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "nettle-0__3.10.1-1.el9.aarch64",
    sha256 = "caf6dda4eaf3c7e3061ec335d45176ebfcaa72ed583df59c32c9dffc00a24ad9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/nettle-3.10.1-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/caf6dda4eaf3c7e3061ec335d45176ebfcaa72ed583df59c32c9dffc00a24ad9",
    ],
)

rpm(
    name = "nettle-0__3.10.1-1.el9.s390x",
    sha256 = "d05a33e0b673bc34580c443f7d7c28b50f8b4fd77ad87ed3cef30f991d7cbf09",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/nettle-3.10.1-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/d05a33e0b673bc34580c443f7d7c28b50f8b4fd77ad87ed3cef30f991d7cbf09",
    ],
)

rpm(
    name = "nettle-0__3.10.1-1.el9.x86_64",
    sha256 = "aa28996450c98399099cfcc0fb722723b5821edff27cff53288e1c0298a98190",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/nettle-3.10.1-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/aa28996450c98399099cfcc0fb722723b5821edff27cff53288e1c0298a98190",
    ],
)

rpm(
    name = "nettle-0__3.7.3-2.el9.aarch64",
    sha256 = "6e1d488f0495d26bf9f81bfc18f496f964cce9c080b428528b32211eb4f3d437",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/nettle-3.7.3-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/6e1d488f0495d26bf9f81bfc18f496f964cce9c080b428528b32211eb4f3d437",
    ],
)

rpm(
    name = "nettle-0__3.7.3-2.el9.x86_64",
    sha256 = "7f60a98cb26b946d9a3feb77d3a0d34dfadd7ff45771b662f05f59a019962764",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/nettle-3.7.3-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/7f60a98cb26b946d9a3feb77d3a0d34dfadd7ff45771b662f05f59a019962764",
    ],
)

rpm(
    name = "nftables-1__1.0.9-9.el9.aarch64",
    sha256 = "5330e373b967b94b6eb20c5a8ca36881e1bffd7d80a26f9ca09db0a2e6e13cb7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/nftables-1.0.9-9.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/5330e373b967b94b6eb20c5a8ca36881e1bffd7d80a26f9ca09db0a2e6e13cb7",
    ],
)

rpm(
    name = "nftables-1__1.0.9-9.el9.s390x",
    sha256 = "103c8ce379ffc363dcd563797d9b10df26dee2c44f3baded3f156001154e3b47",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/nftables-1.0.9-9.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/103c8ce379ffc363dcd563797d9b10df26dee2c44f3baded3f156001154e3b47",
    ],
)

rpm(
    name = "nftables-1__1.0.9-9.el9.x86_64",
    sha256 = "073e9b6bc30fe9e1cf9a0228c24a9555eda3d0eb9f6f59c86e9dbb68a2ad610b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/nftables-1.0.9-9.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/073e9b6bc30fe9e1cf9a0228c24a9555eda3d0eb9f6f59c86e9dbb68a2ad610b",
    ],
)

rpm(
    name = "nftables-1__1.1.7-1.el10.aarch64",
    sha256 = "2ab5dacfb073169b9878e677be1427f7042e3efaea7c7c8629be762589a917ee",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/nftables-1.1.7-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "nftables-1__1.1.7-1.el10.s390x",
    sha256 = "acb9edb8f6d4b585a6c4894023a6a66c0fc6f0fa298d56796243a29ca863d6bf",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/nftables-1.1.7-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "nftables-1__1.1.7-1.el10.x86_64",
    sha256 = "37bcbeed18b198e46b5e555ec9bfc1381ea50a77cc0201eaefbd64132e72839b",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/nftables-1.1.7-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "nginx-1__1.22.1-4.module_el9__plus__666__plus__132dc76f.aarch64",
    sha256 = "73f304462d847fd7324ed9d578c8fcf31bf7f9a15dd04097b7bd757670531c33",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nginx-1.22.1-4.module_el9+666+132dc76f.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/73f304462d847fd7324ed9d578c8fcf31bf7f9a15dd04097b7bd757670531c33",
    ],
)

rpm(
    name = "nginx-1__1.22.1-4.module_el9__plus__666__plus__132dc76f.s390x",
    sha256 = "68a6c2280597e75c452a55b075a38e3ccfac069233e8f58202bfa7b5ce6d4489",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nginx-1.22.1-4.module_el9+666+132dc76f.s390x.rpm",
        "https://storage.googleapis.com/builddeps/68a6c2280597e75c452a55b075a38e3ccfac069233e8f58202bfa7b5ce6d4489",
    ],
)

rpm(
    name = "nginx-1__1.22.1-4.module_el9__plus__666__plus__132dc76f.x86_64",
    sha256 = "6a6e8247840fdd4f8328f94701a7a3649903c8d8fd72aaec9b214a9dad690e5c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nginx-1.22.1-4.module_el9+666+132dc76f.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/6a6e8247840fdd4f8328f94701a7a3649903c8d8fd72aaec9b214a9dad690e5c",
    ],
)

rpm(
    name = "nginx-2__1.26.3-16.el10.aarch64",
    sha256 = "2ad187053d8dd30c03012a55764c554cc5e3ff5bc69ddb825d94c222fba13662",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/aarch64/os/Packages/nginx-1.26.3-16.el10.aarch64.rpm",
    ],
)

rpm(
    name = "nginx-2__1.26.3-16.el10.s390x",
    sha256 = "290e6cff7a56e1204ed701ab58a1cd035beb0bc940b861a877dcaf967195ae2d",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/s390x/os/Packages/nginx-1.26.3-16.el10.s390x.rpm",
    ],
)

rpm(
    name = "nginx-2__1.26.3-16.el10.x86_64",
    sha256 = "1abac7b6aed5bb176eb6291dc95061e16a1eb45909013c4674900cb28fc9b1d4",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/x86_64/os/Packages/nginx-1.26.3-16.el10.x86_64.rpm",
    ],
)

rpm(
    name = "nginx-core-1__1.22.1-4.module_el9__plus__666__plus__132dc76f.aarch64",
    sha256 = "70009983e5f5cf026805ce8d584aa251f48f8f386e1d489def7902eb6714990c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nginx-core-1.22.1-4.module_el9+666+132dc76f.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/70009983e5f5cf026805ce8d584aa251f48f8f386e1d489def7902eb6714990c",
    ],
)

rpm(
    name = "nginx-core-1__1.22.1-4.module_el9__plus__666__plus__132dc76f.s390x",
    sha256 = "85ec7142eb8257fd55e809856dcaf6d8035a22c6abe18d7c360ef54715753c57",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nginx-core-1.22.1-4.module_el9+666+132dc76f.s390x.rpm",
        "https://storage.googleapis.com/builddeps/85ec7142eb8257fd55e809856dcaf6d8035a22c6abe18d7c360ef54715753c57",
    ],
)

rpm(
    name = "nginx-core-1__1.22.1-4.module_el9__plus__666__plus__132dc76f.x86_64",
    sha256 = "67e61b35c5b95457264201f23ef8aef797b2564e7fb0ea9a68b2245d75464d5c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nginx-core-1.22.1-4.module_el9+666+132dc76f.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/67e61b35c5b95457264201f23ef8aef797b2564e7fb0ea9a68b2245d75464d5c",
    ],
)

rpm(
    name = "nginx-core-2__1.26.3-16.el10.aarch64",
    sha256 = "d90b7ad3c033b5907953416d8f1506428dd07231c1b387396d597f5a5823c21f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/aarch64/os/Packages/nginx-core-1.26.3-16.el10.aarch64.rpm",
    ],
)

rpm(
    name = "nginx-core-2__1.26.3-16.el10.s390x",
    sha256 = "a80514be1b0dc1e2003f2611f06f2149aaa4b6c7a291addd83b97127d50cafa3",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/s390x/os/Packages/nginx-core-1.26.3-16.el10.s390x.rpm",
    ],
)

rpm(
    name = "nginx-core-2__1.26.3-16.el10.x86_64",
    sha256 = "fface4247e20aee7eff15d6c17ec1e7cba42204c0062b2042f542b0048497598",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/x86_64/os/Packages/nginx-core-1.26.3-16.el10.x86_64.rpm",
    ],
)

rpm(
    name = "nginx-filesystem-1__1.22.1-4.module_el9__plus__666__plus__132dc76f.aarch64",
    sha256 = "8e564ab711b4a60b769a970dc03fb7a32e6441bf9e761321e3f0a24080491eb2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nginx-filesystem-1.22.1-4.module_el9+666+132dc76f.noarch.rpm",
        "https://storage.googleapis.com/builddeps/8e564ab711b4a60b769a970dc03fb7a32e6441bf9e761321e3f0a24080491eb2",
    ],
)

rpm(
    name = "nginx-filesystem-1__1.22.1-4.module_el9__plus__666__plus__132dc76f.s390x",
    sha256 = "8e564ab711b4a60b769a970dc03fb7a32e6441bf9e761321e3f0a24080491eb2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nginx-filesystem-1.22.1-4.module_el9+666+132dc76f.noarch.rpm",
        "https://storage.googleapis.com/builddeps/8e564ab711b4a60b769a970dc03fb7a32e6441bf9e761321e3f0a24080491eb2",
    ],
)

rpm(
    name = "nginx-filesystem-1__1.22.1-4.module_el9__plus__666__plus__132dc76f.x86_64",
    sha256 = "8e564ab711b4a60b769a970dc03fb7a32e6441bf9e761321e3f0a24080491eb2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nginx-filesystem-1.22.1-4.module_el9+666+132dc76f.noarch.rpm",
        "https://storage.googleapis.com/builddeps/8e564ab711b4a60b769a970dc03fb7a32e6441bf9e761321e3f0a24080491eb2",
    ],
)

rpm(
    name = "nginx-filesystem-2__1.26.3-16.el10.aarch64",
    sha256 = "43e0eb9ba3d2b3bd25edb347a2f471ef798c64ca370f83f1379af70cda04cf5e",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/aarch64/os/Packages/nginx-filesystem-1.26.3-16.el10.noarch.rpm",
    ],
)

rpm(
    name = "nginx-filesystem-2__1.26.3-16.el10.s390x",
    sha256 = "43e0eb9ba3d2b3bd25edb347a2f471ef798c64ca370f83f1379af70cda04cf5e",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/s390x/os/Packages/nginx-filesystem-1.26.3-16.el10.noarch.rpm",
    ],
)

rpm(
    name = "nginx-filesystem-2__1.26.3-16.el10.x86_64",
    sha256 = "43e0eb9ba3d2b3bd25edb347a2f471ef798c64ca370f83f1379af70cda04cf5e",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/x86_64/os/Packages/nginx-filesystem-1.26.3-16.el10.noarch.rpm",
    ],
)

rpm(
    name = "npth-0__1.6-21.el10.aarch64",
    sha256 = "5b7b2090a9fb0dc9749482cac1c90c0ef46619866e825f9183d51d10f54a5f6b",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/npth-1.6-21.el10.aarch64.rpm",
    ],
)

rpm(
    name = "npth-0__1.6-21.el10.s390x",
    sha256 = "47f1f79ad844c4d845591871bc752bf8677fb257fa2cc4d58778fab215965bf1",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/npth-1.6-21.el10.s390x.rpm",
    ],
)

rpm(
    name = "npth-0__1.6-21.el10.x86_64",
    sha256 = "9d5de697dd346d3eeac85008ab93fbfce90ea49342418402959eda90829578d0",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/npth-1.6-21.el10.x86_64.rpm",
    ],
)

rpm(
    name = "npth-0__1.6-8.el9.aarch64",
    sha256 = "95bd797672d70a8752fb881c4ff04ccc14234842dfd9de6bc48373dd96c1ec81",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/npth-1.6-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/95bd797672d70a8752fb881c4ff04ccc14234842dfd9de6bc48373dd96c1ec81",
    ],
)

rpm(
    name = "npth-0__1.6-8.el9.s390x",
    sha256 = "f66f12068208409067e6c342e6c0f4f0646fe527dbb7d5bc3d41adb4d9802b52",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/npth-1.6-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/f66f12068208409067e6c342e6c0f4f0646fe527dbb7d5bc3d41adb4d9802b52",
    ],
)

rpm(
    name = "npth-0__1.6-8.el9.x86_64",
    sha256 = "a7da4ef003bc60045bc60dae299b703e7f1db326f25208fb922ce1b79e2882da",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/npth-1.6-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/a7da4ef003bc60045bc60dae299b703e7f1db326f25208fb922ce1b79e2882da",
    ],
)

rpm(
    name = "numactl-libs-0__2.0.19-3.el10.aarch64",
    sha256 = "81016ab56c83cb8c221216794571ae58bb914e21dd3794c242f7ce8a8d8fbf8f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/numactl-libs-2.0.19-3.el10.aarch64.rpm",
    ],
)

rpm(
    name = "numactl-libs-0__2.0.19-3.el10.x86_64",
    sha256 = "263ee2cba1d57996778f70045fbc4657067f73edafd6c6b04f4599c3eb12fbfd",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/numactl-libs-2.0.19-3.el10.x86_64.rpm",
    ],
)

rpm(
    name = "numactl-libs-0__2.0.19-3.el9.aarch64",
    sha256 = "ff63cef9b42cbc82149a6bc6970c20c9e781016dbb3eadd03effa330cb3b2bdd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/numactl-libs-2.0.19-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ff63cef9b42cbc82149a6bc6970c20c9e781016dbb3eadd03effa330cb3b2bdd",
    ],
)

rpm(
    name = "numactl-libs-0__2.0.19-3.el9.x86_64",
    sha256 = "ad52833edf28b5bf2053bd96d96b211de4c6b11376978379dae211460c4596d8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/numactl-libs-2.0.19-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/ad52833edf28b5bf2053bd96d96b211de4c6b11376978379dae211460c4596d8",
    ],
)

rpm(
    name = "openldap-0__2.6.13-1.el10.aarch64",
    sha256 = "6711ed63ab2257917dbc7e161b9fb41be1243378f3e0d8ef9c942e1d95e7b809",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/openldap-2.6.13-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "openldap-0__2.6.13-1.el10.s390x",
    sha256 = "51c1bcd25a49f81085101b74e089e9ac065f763a147d07c62b4f0978413c9dcc",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/openldap-2.6.13-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "openldap-0__2.6.13-1.el10.x86_64",
    sha256 = "668bf98af411e595330006c5eb8834cfe0fd8b11e90525d0f72709b0c200e224",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/openldap-2.6.13-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "openldap-0__2.6.13-1.el9.aarch64",
    sha256 = "4ec33dce6b05c7f39f6d1d817fc5ee0460ddb2d30eb4ea55a3af4f13643d1ad4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openldap-2.6.13-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/4ec33dce6b05c7f39f6d1d817fc5ee0460ddb2d30eb4ea55a3af4f13643d1ad4",
    ],
)

rpm(
    name = "openldap-0__2.6.13-1.el9.s390x",
    sha256 = "b8a4974ea6b1e8b307bf73054a22b6f4d3c34724ca6fa960d0b97978ab52290f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openldap-2.6.13-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/b8a4974ea6b1e8b307bf73054a22b6f4d3c34724ca6fa960d0b97978ab52290f",
    ],
)

rpm(
    name = "openldap-0__2.6.13-1.el9.x86_64",
    sha256 = "6bed5684275d340e78f9300c4da665a6a0ea6779f7cee7217ddee868af81d8eb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openldap-2.6.13-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/6bed5684275d340e78f9300c4da665a6a0ea6779f7cee7217ddee868af81d8eb",
    ],
)

rpm(
    name = "openssl-1__3.5.8-1.el9.aarch64",
    sha256 = "4046cc98903b0bdf84ef0b7f24cba48ab414445a50ed58d6efbe41ecceedcb03",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openssl-3.5.8-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/4046cc98903b0bdf84ef0b7f24cba48ab414445a50ed58d6efbe41ecceedcb03",
    ],
)

rpm(
    name = "openssl-1__3.5.8-1.el9.s390x",
    sha256 = "b40b53d87e62422d4e01ef71e777db3457fc9313a278f5da35d8cf6c6c4f5628",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openssl-3.5.8-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/b40b53d87e62422d4e01ef71e777db3457fc9313a278f5da35d8cf6c6c4f5628",
    ],
)

rpm(
    name = "openssl-1__3.5.8-1.el9.x86_64",
    sha256 = "b97860c63a00ab5a00a3bb1ffa0fc2657e7da128e1adbb7a0a6b90c30ae8204e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-3.5.8-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/b97860c63a00ab5a00a3bb1ffa0fc2657e7da128e1adbb7a0a6b90c30ae8204e",
    ],
)

rpm(
    name = "openssl-1__3.5.9-1.el10.aarch64",
    sha256 = "7baab07bbbe255aef683922f9a3801921feae05623008f939500820a2239c304",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/openssl-3.5.9-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "openssl-1__3.5.9-1.el10.s390x",
    sha256 = "b5106760bb68c07523681ab485cee279fab7236d0438b0e8704927531578031e",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/openssl-3.5.9-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "openssl-1__3.5.9-1.el10.x86_64",
    sha256 = "2c9b63c93997a2e259c8d31ef84fb2514dba7dab8adcb39e9dc79cf37e9be9d6",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/openssl-3.5.9-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "openssl-fips-provider-1__3.5.8-1.el9.aarch64",
    sha256 = "62090dac8c54674d25a8346e2107052e2fec38aa262a2d4d88f4355791c1a152",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openssl-fips-provider-3.5.8-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/62090dac8c54674d25a8346e2107052e2fec38aa262a2d4d88f4355791c1a152",
    ],
)

rpm(
    name = "openssl-fips-provider-1__3.5.8-1.el9.s390x",
    sha256 = "548bc61aeb21789ee3327eabf6d225bdae4ddeb823647475ce595abfd7beb676",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openssl-fips-provider-3.5.8-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/548bc61aeb21789ee3327eabf6d225bdae4ddeb823647475ce595abfd7beb676",
    ],
)

rpm(
    name = "openssl-fips-provider-1__3.5.8-1.el9.x86_64",
    sha256 = "a85d39899c41ea7cc151a8c3600f2f452a7b4d4942b3e2911bdbb843ca2c37d8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-fips-provider-3.5.8-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/a85d39899c41ea7cc151a8c3600f2f452a7b4d4942b3e2911bdbb843ca2c37d8",
    ],
)

rpm(
    name = "openssl-fips-provider-1__3.5.9-1.el10.aarch64",
    sha256 = "cb132104592293b6470754831d26f47d4491a203ee959f5366d7e68ebf4f4539",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/openssl-fips-provider-3.5.9-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "openssl-fips-provider-1__3.5.9-1.el10.s390x",
    sha256 = "9870fce9cd9edffdf83841ed24ab7f86b4b78d3c20cfb99c51fb3a00598f55db",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/openssl-fips-provider-3.5.9-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "openssl-fips-provider-1__3.5.9-1.el10.x86_64",
    sha256 = "6dabd095a1b9977cbbb9ef70d9e0d2507f099de4cea6e87a31784658460a1bb1",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/openssl-fips-provider-3.5.9-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "openssl-libs-1__3.0.1-18.el9.aarch64",
    sha256 = "a69db31e7748b0e23c98520d632d6a76f2f3ea1bff4f7b71cde60adaed470c96",
    urls = ["https://storage.googleapis.com/builddeps/a69db31e7748b0e23c98520d632d6a76f2f3ea1bff4f7b71cde60adaed470c96"],
)

rpm(
    name = "openssl-libs-1__3.0.1-18.el9.x86_64",
    sha256 = "cbe97622a4d4dbd00e2264a5f96087805af03717dfb842dbb6b6412be8f24e99",
    urls = ["https://storage.googleapis.com/builddeps/cbe97622a4d4dbd00e2264a5f96087805af03717dfb842dbb6b6412be8f24e99"],
)

rpm(
    name = "openssl-libs-1__3.5.8-1.el9.aarch64",
    sha256 = "a9d04beccd00356feb421f324d6943a2002720773dde03f7c3024760a0bbf84a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openssl-libs-3.5.8-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/a9d04beccd00356feb421f324d6943a2002720773dde03f7c3024760a0bbf84a",
    ],
)

rpm(
    name = "openssl-libs-1__3.5.8-1.el9.s390x",
    sha256 = "02a65e679bf478528508dfb0cc71732dbf426cc1c8d5d9c161b43e7f028de431",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openssl-libs-3.5.8-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/02a65e679bf478528508dfb0cc71732dbf426cc1c8d5d9c161b43e7f028de431",
    ],
)

rpm(
    name = "openssl-libs-1__3.5.8-1.el9.x86_64",
    sha256 = "35e05f6572e8fa953564ccf58e2c1198ef20778bd828b724bdb3a6226fb9d031",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-libs-3.5.8-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/35e05f6572e8fa953564ccf58e2c1198ef20778bd828b724bdb3a6226fb9d031",
    ],
)

rpm(
    name = "openssl-libs-1__3.5.9-1.el10.aarch64",
    sha256 = "2f6ca3bc09a6da1717b0503e50f162cb99c57f545537ff5e9d919882bc644134",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/openssl-libs-3.5.9-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "openssl-libs-1__3.5.9-1.el10.s390x",
    sha256 = "08c7c55942d16ba4304475fe1232db2a3fab97129564ca9cc6e81abca32395e3",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/openssl-libs-3.5.9-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "openssl-libs-1__3.5.9-1.el10.x86_64",
    sha256 = "33c5f1171f58c13c11bc4152a50da289c98b61c1d0935d2190c402dc09b1c421",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/openssl-libs-3.5.9-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "ovirt-imageio-client-0__2.5.2-1.el10s.aarch64",
    sha256 = "66ef48a96c6a92d35cdba983d25dcf90397cf218dcf847a0d58c14944e771d2d",
    urls = [
        "https://mirror.stream.centos.org/SIGs/10-stream/virt/aarch64/ovirt-45/Packages/o/ovirt-imageio-client-2.5.2-1.el10s.aarch64.rpm",
    ],
)

rpm(
    name = "ovirt-imageio-client-0__2.5.2-1.el9.aarch64",
    sha256 = "56a58715ac2a79bb678d3fbdc781900218402f9fe7187339b75422b77d39f265",
    urls = [
        "https://mirror.stream.centos.org/SIGs/9-stream/virt/aarch64/ovirt-45/Packages/o/ovirt-imageio-client-2.5.2-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/56a58715ac2a79bb678d3fbdc781900218402f9fe7187339b75422b77d39f265",
    ],
)

rpm(
    name = "ovirt-imageio-client-0__2.5.3-0.202603060908.git0eb617c.el10.x86_64",
    sha256 = "5681a2a996fbc02a9187ed0e1924232a92a6d0f68f1bc74dc2ca00113ff62a6f",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-10-x86_64/Packages/o/ovirt-imageio-client-2.5.3-0.202603060908.git0eb617c.el10.x86_64.rpm",
    ],
)

rpm(
    name = "ovirt-imageio-client-0__2.5.3-0.202603060908.git0eb617c.el9.x86_64",
    sha256 = "c21e977f1387aef1d8fe04db1ce35eee310b2160bcc1ca3cda409f1990553497",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-9-x86_64/10195366-ovirt-imageio/ovirt-imageio-client-2.5.3-0.202603060908.git0eb617c.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c21e977f1387aef1d8fe04db1ce35eee310b2160bcc1ca3cda409f1990553497",
    ],
)

rpm(
    name = "ovirt-imageio-common-0__2.5.2-1.el10s.aarch64",
    sha256 = "88d1278d69d2eeeb14e33f3c54d8968b12199018bbcfdf5b927bc5a5fd4802c7",
    urls = [
        "https://mirror.stream.centos.org/SIGs/10-stream/virt/aarch64/ovirt-45/Packages/o/ovirt-imageio-common-2.5.2-1.el10s.aarch64.rpm",
    ],
)

rpm(
    name = "ovirt-imageio-common-0__2.5.2-1.el9.aarch64",
    sha256 = "1d4244f0979112c8f6768e689cf923f1313b3d6d885d512784276522be2485dc",
    urls = [
        "https://mirror.stream.centos.org/SIGs/9-stream/virt/aarch64/ovirt-45/Packages/o/ovirt-imageio-common-2.5.2-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/1d4244f0979112c8f6768e689cf923f1313b3d6d885d512784276522be2485dc",
    ],
)

rpm(
    name = "ovirt-imageio-common-0__2.5.3-0.202603060908.git0eb617c.el10.x86_64",
    sha256 = "1c936fce6c32238a9d06f1735de7114dab2cbe93696afc40d5235bf04d8a5a6e",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-10-x86_64/Packages/o/ovirt-imageio-common-2.5.3-0.202603060908.git0eb617c.el10.x86_64.rpm",
    ],
)

rpm(
    name = "ovirt-imageio-common-0__2.5.3-0.202603060908.git0eb617c.el9.x86_64",
    sha256 = "85c1a1a6b4805a88a243388899cf363d8cc1f3b11a1dc7043b5b6e0cd080ad81",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-9-x86_64/10195366-ovirt-imageio/ovirt-imageio-common-2.5.3-0.202603060908.git0eb617c.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/85c1a1a6b4805a88a243388899cf363d8cc1f3b11a1dc7043b5b6e0cd080ad81",
    ],
)

rpm(
    name = "ovirt-imageio-daemon-0__2.5.2-1.el10s.aarch64",
    sha256 = "ca2e341446143d12ac1ea48a4c059ee3295baa673ae97b2fc37df33063ec6e82",
    urls = [
        "https://mirror.stream.centos.org/SIGs/10-stream/virt/aarch64/ovirt-45/Packages/o/ovirt-imageio-daemon-2.5.2-1.el10s.aarch64.rpm",
    ],
)

rpm(
    name = "ovirt-imageio-daemon-0__2.5.2-1.el9.aarch64",
    sha256 = "ddb50e2fad931c1dd8c9b88720ab94bb4e439fedfc4ec31918ceaca8c1310d50",
    urls = [
        "https://mirror.stream.centos.org/SIGs/9-stream/virt/aarch64/ovirt-45/Packages/o/ovirt-imageio-daemon-2.5.2-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ddb50e2fad931c1dd8c9b88720ab94bb4e439fedfc4ec31918ceaca8c1310d50",
    ],
)

rpm(
    name = "ovirt-imageio-daemon-0__2.5.3-0.202603060908.git0eb617c.el10.x86_64",
    sha256 = "90b3d304545b857ef512d6d9e262db7d3b2fef69ecf32493afdfdffe81aa3e11",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-10-x86_64/Packages/o/ovirt-imageio-daemon-2.5.3-0.202603060908.git0eb617c.el10.x86_64.rpm",
    ],
)

rpm(
    name = "ovirt-imageio-daemon-0__2.5.3-0.202603060908.git0eb617c.el9.x86_64",
    sha256 = "190c4763d39b5b5f9f4051ecc6901ca8e04fc9362e1d9980d061a95ce0c9937a",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-9-x86_64/10195366-ovirt-imageio/ovirt-imageio-daemon-2.5.3-0.202603060908.git0eb617c.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/190c4763d39b5b5f9f4051ecc6901ca8e04fc9362e1d9980d061a95ce0c9937a",
    ],
)

rpm(
    name = "p11-kit-0__0.24.1-2.el9.aarch64",
    sha256 = "98e7f00d012549fa8fbaba21626388a0b07731f3f25a5801418247d66a5a985f",
    urls = ["https://storage.googleapis.com/builddeps/98e7f00d012549fa8fbaba21626388a0b07731f3f25a5801418247d66a5a985f"],
)

rpm(
    name = "p11-kit-0__0.24.1-2.el9.x86_64",
    sha256 = "da167e41efd19cf25fd1c708b6f123d0203824324b14dd32401d49f2aa0ef0a6",
    urls = ["https://storage.googleapis.com/builddeps/da167e41efd19cf25fd1c708b6f123d0203824324b14dd32401d49f2aa0ef0a6"],
)

rpm(
    name = "p11-kit-0__0.26.4-1.el10.aarch64",
    sha256 = "a3cfdc5bde366b5f88222ad2154f578de277bfe0a35e5631c8fe1503d39ad810",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/p11-kit-0.26.4-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "p11-kit-0__0.26.4-1.el10.s390x",
    sha256 = "d8fcc01535dfbcfdc173293236d6b28f59a669f0fea76860de258491ec91e901",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/p11-kit-0.26.4-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "p11-kit-0__0.26.4-1.el10.x86_64",
    sha256 = "8a4a97c411dbda23a0b91fadfd269f14aa669af424002eca91ac9fc202400204",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/p11-kit-0.26.4-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "p11-kit-0__0.26.4-1.el9.aarch64",
    sha256 = "9769d2d2bfa7db493218bdba14075df2428543817e125c45f8775482236ce2aa",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/p11-kit-0.26.4-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/9769d2d2bfa7db493218bdba14075df2428543817e125c45f8775482236ce2aa",
    ],
)

rpm(
    name = "p11-kit-0__0.26.4-1.el9.s390x",
    sha256 = "b9a14789372202a1b4d0d30c57756d8bce2a22342c90f63479343415b866495e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/p11-kit-0.26.4-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/b9a14789372202a1b4d0d30c57756d8bce2a22342c90f63479343415b866495e",
    ],
)

rpm(
    name = "p11-kit-0__0.26.4-1.el9.x86_64",
    sha256 = "185c0ee8f5470fe94b492f11c1e0c1674be3051062e906cdd32473a5ff8db045",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/p11-kit-0.26.4-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/185c0ee8f5470fe94b492f11c1e0c1674be3051062e906cdd32473a5ff8db045",
    ],
)

rpm(
    name = "p11-kit-trust-0__0.24.1-2.el9.aarch64",
    sha256 = "80e288a5b62f20f7794674c6fdf2f0765a322cd0e81df9359e37582fe950289c",
    urls = ["https://storage.googleapis.com/builddeps/80e288a5b62f20f7794674c6fdf2f0765a322cd0e81df9359e37582fe950289c"],
)

rpm(
    name = "p11-kit-trust-0__0.24.1-2.el9.x86_64",
    sha256 = "ae9a633c58980328bef6358c6aa3c9ce0a65130c66fbfa4249922ddf5a3e2bb1",
    urls = ["https://storage.googleapis.com/builddeps/ae9a633c58980328bef6358c6aa3c9ce0a65130c66fbfa4249922ddf5a3e2bb1"],
)

rpm(
    name = "p11-kit-trust-0__0.26.4-1.el10.aarch64",
    sha256 = "465c251c3f8b10ae8afa72da7a4d99f615048eb96b7602954f05fc1bf86e14fb",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/p11-kit-trust-0.26.4-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "p11-kit-trust-0__0.26.4-1.el10.s390x",
    sha256 = "e1317a46aaaa816a201c3ec6eab8482c00e2e14db2332fa3686b724c3b8b42ee",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/p11-kit-trust-0.26.4-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "p11-kit-trust-0__0.26.4-1.el10.x86_64",
    sha256 = "4ac0cba633e175b2893ec96e1ac7ab5d70b8d77d19c688cda7458b1ff573e53e",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/p11-kit-trust-0.26.4-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "p11-kit-trust-0__0.26.4-1.el9.aarch64",
    sha256 = "89535f163fd9f6d78be992d9145e7e628fd1d7308bc8f9d6fb4744b5e14a7628",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/p11-kit-trust-0.26.4-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/89535f163fd9f6d78be992d9145e7e628fd1d7308bc8f9d6fb4744b5e14a7628",
    ],
)

rpm(
    name = "p11-kit-trust-0__0.26.4-1.el9.s390x",
    sha256 = "66c57115a29f40b23a6cf8b522033e5731850ea4e7df2769871db794816588f5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/p11-kit-trust-0.26.4-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/66c57115a29f40b23a6cf8b522033e5731850ea4e7df2769871db794816588f5",
    ],
)

rpm(
    name = "p11-kit-trust-0__0.26.4-1.el9.x86_64",
    sha256 = "8dc277e3855df15bf2db2e8acd03e08311cd565152fa958ac75cbb862f98ebe1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/p11-kit-trust-0.26.4-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/8dc277e3855df15bf2db2e8acd03e08311cd565152fa958ac75cbb862f98ebe1",
    ],
)

rpm(
    name = "pam-0__1.5.1-30.el9.aarch64",
    sha256 = "4cfbf288aae8a577ecc1af7b0a0aeb560fae1e25e8d7a26d45ced083bebea7e4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/pam-1.5.1-30.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/4cfbf288aae8a577ecc1af7b0a0aeb560fae1e25e8d7a26d45ced083bebea7e4",
    ],
)

rpm(
    name = "pam-0__1.5.1-30.el9.s390x",
    sha256 = "99e9c68c8da890da50dedcc0f25637b232c9925e7e8199205d4b5a021c982dce",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/pam-1.5.1-30.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/99e9c68c8da890da50dedcc0f25637b232c9925e7e8199205d4b5a021c982dce",
    ],
)

rpm(
    name = "pam-0__1.5.1-30.el9.x86_64",
    sha256 = "bd114843bfc3e1e51c3180a77bf453381854420e19bc1c0f0d65ea99dedc7782",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pam-1.5.1-30.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/bd114843bfc3e1e51c3180a77bf453381854420e19bc1c0f0d65ea99dedc7782",
    ],
)

rpm(
    name = "pam-libs-0__1.6.1-11.el10.aarch64",
    sha256 = "9b730de5039b508cea3b478b58f529d138694dbf89dc260ee61a8cc605a5902f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/pam-libs-1.6.1-11.el10.aarch64.rpm",
    ],
)

rpm(
    name = "pam-libs-0__1.6.1-11.el10.s390x",
    sha256 = "82b0636b6527f0b2d2254965dd052f46426bc08502c60dc286d2dd6d1da74f0e",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/pam-libs-1.6.1-11.el10.s390x.rpm",
    ],
)

rpm(
    name = "pam-libs-0__1.6.1-11.el10.x86_64",
    sha256 = "4879d3c55fcc2d3ff3a9f3d73cd840adba191f22e3c33745cc81f5910cc3c51d",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/pam-libs-1.6.1-11.el10.x86_64.rpm",
    ],
)

rpm(
    name = "passt-0__0__caret__20260728.gf8df3f1-1.el10.aarch64",
    sha256 = "1129e86fdc28f428eed119d74fd9b072989e46689155fec4e4a88a5e2b2937a1",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/passt-0%5E20260728.gf8df3f1-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "passt-0__0__caret__20260728.gf8df3f1-1.el10.s390x",
    sha256 = "7968714b4f561e0ce6b93966d60a166499f1985e76027602525b88a44fe020d0",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/passt-0%5E20260728.gf8df3f1-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "passt-0__0__caret__20260728.gf8df3f1-1.el10.x86_64",
    sha256 = "a43832527266d5af8b612ddb7d6ee8e63d9e28e58ced37dd88dfb5733feb491a",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/passt-0%5E20260728.gf8df3f1-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "passt-0__0__caret__20260728.gf8df3f1-1.el9.aarch64",
    sha256 = "a45ef14c0eaa87dfc99c874211358e96be52f62a69fc170d6a20c4cb0cf0c848",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/passt-0%5E20260728.gf8df3f1-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/a45ef14c0eaa87dfc99c874211358e96be52f62a69fc170d6a20c4cb0cf0c848",
    ],
)

rpm(
    name = "passt-0__0__caret__20260728.gf8df3f1-1.el9.s390x",
    sha256 = "0e2f3c6f80b45f7f47883ff22656f6eff4d329c34bed0ed91923e736c2f8b2fa",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/passt-0%5E20260728.gf8df3f1-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/0e2f3c6f80b45f7f47883ff22656f6eff4d329c34bed0ed91923e736c2f8b2fa",
    ],
)

rpm(
    name = "passt-0__0__caret__20260728.gf8df3f1-1.el9.x86_64",
    sha256 = "5789e9a1d80b2cd31cdad8b914df49d49d78cb9cd8a1c8dfbbcd88b94d501740",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/passt-0%5E20260728.gf8df3f1-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/5789e9a1d80b2cd31cdad8b914df49d49d78cb9cd8a1c8dfbbcd88b94d501740",
    ],
)

rpm(
    name = "pcre-0__8.44-3.el9.3.aarch64",
    sha256 = "0331efd537704e75e26324ba6bb1568762d01bafe7fbce5b981ff0ee0d3ea80c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/pcre-8.44-3.el9.3.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/0331efd537704e75e26324ba6bb1568762d01bafe7fbce5b981ff0ee0d3ea80c",
    ],
)

rpm(
    name = "pcre-0__8.44-3.el9.3.x86_64",
    sha256 = "4a3cb61eb08c4f24e44756b6cb329812fe48d5c65c1fba546fadfa975045a8c5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pcre-8.44-3.el9.3.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/4a3cb61eb08c4f24e44756b6cb329812fe48d5c65c1fba546fadfa975045a8c5",
    ],
)

rpm(
    name = "pcre-0__8.44-4.el9.aarch64",
    sha256 = "dc5d71786a68cfa15f49aecd12e90de7af7489a2d0a4d102be38a9faf0c99ae8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/pcre-8.44-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/dc5d71786a68cfa15f49aecd12e90de7af7489a2d0a4d102be38a9faf0c99ae8",
    ],
)

rpm(
    name = "pcre-0__8.44-4.el9.s390x",
    sha256 = "e42ebd2b71ed4d5ee34a5fbba116396c22ed4deb7d7ab6189f048a3f603e5dbb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/pcre-8.44-4.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/e42ebd2b71ed4d5ee34a5fbba116396c22ed4deb7d7ab6189f048a3f603e5dbb",
    ],
)

rpm(
    name = "pcre-0__8.44-4.el9.x86_64",
    sha256 = "7d6be1d41cb4d0b159a764bfc7c8efecc0353224b46e5286cbbea7092b700690",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pcre-8.44-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/7d6be1d41cb4d0b159a764bfc7c8efecc0353224b46e5286cbbea7092b700690",
    ],
)

rpm(
    name = "pcre2-0__10.37-3.el9.1.aarch64",
    sha256 = "82de22426c96c26e987befb1056e2a6ecd71ba6966736cd3810522e7da77a0f2",
    urls = ["https://storage.googleapis.com/builddeps/82de22426c96c26e987befb1056e2a6ecd71ba6966736cd3810522e7da77a0f2"],
)

rpm(
    name = "pcre2-0__10.37-3.el9.1.x86_64",
    sha256 = "441e71f24e95b7c319f02264db53f88aa49778b2214f7dd5c75f1a3838e72dea",
    urls = ["https://storage.googleapis.com/builddeps/441e71f24e95b7c319f02264db53f88aa49778b2214f7dd5c75f1a3838e72dea"],
)

rpm(
    name = "pcre2-0__10.40-6.el9.aarch64",
    sha256 = "c13e323c383bd5bbe3415701aa21a56b3fefc32d96e081e91c012ef692c78599",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/pcre2-10.40-6.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c13e323c383bd5bbe3415701aa21a56b3fefc32d96e081e91c012ef692c78599",
    ],
)

rpm(
    name = "pcre2-0__10.40-6.el9.s390x",
    sha256 = "f7c2df461b8fe6a9617a1c1089fc88576e4df16f6ff9aea83b05413d2e15b4d5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/pcre2-10.40-6.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/f7c2df461b8fe6a9617a1c1089fc88576e4df16f6ff9aea83b05413d2e15b4d5",
    ],
)

rpm(
    name = "pcre2-0__10.40-6.el9.x86_64",
    sha256 = "bc1012f5417aab8393836d78ac8c5472b1a2d84a2f9fa2b00fff5f8ad3a5ec26",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pcre2-10.40-6.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/bc1012f5417aab8393836d78ac8c5472b1a2d84a2f9fa2b00fff5f8ad3a5ec26",
    ],
)

rpm(
    name = "pcre2-0__10.44-1.4.el10.aarch64",
    sha256 = "0c6c8922af73980644044772ea5c70b276066282138eba1c6e3a128cfae071d2",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/pcre2-10.44-1.4.el10.aarch64.rpm",
    ],
)

rpm(
    name = "pcre2-0__10.44-1.4.el10.s390x",
    sha256 = "69e655487e194b94ad718f2f9165e6c843e55e16bf9d3e146a24725ca9d81dbb",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/pcre2-10.44-1.4.el10.s390x.rpm",
    ],
)

rpm(
    name = "pcre2-0__10.44-1.4.el10.x86_64",
    sha256 = "f66d4f9ebb21b42dcb2d52b72ca0d4a5ee8a24f5d3cfb8f077f5a33316ae56e1",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/pcre2-10.44-1.4.el10.x86_64.rpm",
    ],
)

rpm(
    name = "pcre2-syntax-0__10.37-3.el9.1.aarch64",
    sha256 = "55d7d2bc962334c236418b78199a496b05dea4efdc89e52453154bd1a5ad0e2e",
    urls = ["https://storage.googleapis.com/builddeps/55d7d2bc962334c236418b78199a496b05dea4efdc89e52453154bd1a5ad0e2e"],
)

rpm(
    name = "pcre2-syntax-0__10.37-3.el9.1.x86_64",
    sha256 = "55d7d2bc962334c236418b78199a496b05dea4efdc89e52453154bd1a5ad0e2e",
    urls = ["https://storage.googleapis.com/builddeps/55d7d2bc962334c236418b78199a496b05dea4efdc89e52453154bd1a5ad0e2e"],
)

rpm(
    name = "pcre2-syntax-0__10.40-6.el9.aarch64",
    sha256 = "be36a84f6e311a59190664d61a466471391ab01fb77bd1d2348e9a76414aded4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/pcre2-syntax-10.40-6.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/be36a84f6e311a59190664d61a466471391ab01fb77bd1d2348e9a76414aded4",
    ],
)

rpm(
    name = "pcre2-syntax-0__10.40-6.el9.s390x",
    sha256 = "be36a84f6e311a59190664d61a466471391ab01fb77bd1d2348e9a76414aded4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/pcre2-syntax-10.40-6.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/be36a84f6e311a59190664d61a466471391ab01fb77bd1d2348e9a76414aded4",
    ],
)

rpm(
    name = "pcre2-syntax-0__10.40-6.el9.x86_64",
    sha256 = "be36a84f6e311a59190664d61a466471391ab01fb77bd1d2348e9a76414aded4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pcre2-syntax-10.40-6.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/be36a84f6e311a59190664d61a466471391ab01fb77bd1d2348e9a76414aded4",
    ],
)

rpm(
    name = "pcre2-syntax-0__10.44-1.4.el10.aarch64",
    sha256 = "ea0b3e9690d614d677a310647126277c0aa190bae8af1e0058ca6086f49e60c0",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/pcre2-syntax-10.44-1.4.el10.noarch.rpm",
    ],
)

rpm(
    name = "pcre2-syntax-0__10.44-1.4.el10.s390x",
    sha256 = "ea0b3e9690d614d677a310647126277c0aa190bae8af1e0058ca6086f49e60c0",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/pcre2-syntax-10.44-1.4.el10.noarch.rpm",
    ],
)

rpm(
    name = "pcre2-syntax-0__10.44-1.4.el10.x86_64",
    sha256 = "ea0b3e9690d614d677a310647126277c0aa190bae8af1e0058ca6086f49e60c0",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/pcre2-syntax-10.44-1.4.el10.noarch.rpm",
    ],
)

rpm(
    name = "podman-sequoia-0__0.4.0__tilde__pqc.3-1.el10.aarch64",
    sha256 = "e472d595148d75524d3fd8fc1700f52699526eb34481836b9ea45919ba2e4a25",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/podman-sequoia-0.4.0~pqc.3-1.el10.aarch64.rpm",
    ],
)

rpm(
    name = "podman-sequoia-0__0.4.0__tilde__pqc.3-1.el10.s390x",
    sha256 = "e4530ba2bc3cb6f9f71047acf4cd589e35b4178221f61e10834ec416416d6cc1",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/podman-sequoia-0.4.0~pqc.3-1.el10.s390x.rpm",
    ],
)

rpm(
    name = "podman-sequoia-0__0.4.0__tilde__pqc.3-1.el10.x86_64",
    sha256 = "224ec24586303b8ecef5fb14396355ce1e56605aafd9f5f0eb3acc56aa701f29",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/podman-sequoia-0.4.0~pqc.3-1.el10.x86_64.rpm",
    ],
)

rpm(
    name = "python3-0__3.12.16-2.el10.aarch64",
    sha256 = "2b8fb214987d3ede70f93dd19d77cbe8c72fdc3f42ec869c4830972dfb14f111",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/python3-3.12.16-2.el10.aarch64.rpm",
    ],
)

rpm(
    name = "python3-0__3.12.16-2.el10.s390x",
    sha256 = "a2c7442bab606ceb850d5c8316909a4b3b65eb397e442fb10aee4923f3235e18",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/python3-3.12.16-2.el10.s390x.rpm",
    ],
)

rpm(
    name = "python3-0__3.12.16-2.el10.x86_64",
    sha256 = "76d93b628c36edc9c6d764ef2b97487c789bee72b06a2984738bd88b810d0adb",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/python3-3.12.16-2.el10.x86_64.rpm",
    ],
)

rpm(
    name = "python3-0__3.9.25-10.el9.aarch64",
    sha256 = "06e67547874c4e2bef792df38ca721b752e7916560a29eab42132487f97b2f20",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-3.9.25-10.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/06e67547874c4e2bef792df38ca721b752e7916560a29eab42132487f97b2f20",
    ],
)

rpm(
    name = "python3-0__3.9.25-10.el9.s390x",
    sha256 = "792d088f20f7ba6616d45ef130e9469d5405298ee7afed49ec2e8221053b321a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-3.9.25-10.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/792d088f20f7ba6616d45ef130e9469d5405298ee7afed49ec2e8221053b321a",
    ],
)

rpm(
    name = "python3-0__3.9.25-10.el9.x86_64",
    sha256 = "6b3c05b37d2f681991467525e085c3fa8fc7a7c80ff1c8734443c3f25f21cf2f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-3.9.25-10.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/6b3c05b37d2f681991467525e085c3fa8fc7a7c80ff1c8734443c3f25f21cf2f",
    ],
)

rpm(
    name = "python3-libs-0__3.12.16-2.el10.aarch64",
    sha256 = "4ff127832e127d16618ce758ec0825e9609362d9d06c78962484ffb543bae205",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/python3-libs-3.12.16-2.el10.aarch64.rpm",
    ],
)

rpm(
    name = "python3-libs-0__3.12.16-2.el10.s390x",
    sha256 = "ee7b265e470ce8185ff69ebbd386e67d1e116040d70df06ab94b8c1ded9a8a5c",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/python3-libs-3.12.16-2.el10.s390x.rpm",
    ],
)

rpm(
    name = "python3-libs-0__3.12.16-2.el10.x86_64",
    sha256 = "06b80aa03745b62c6d912677cb12947d9ecaca992e552160cd462f875cdfd399",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/python3-libs-3.12.16-2.el10.x86_64.rpm",
    ],
)

rpm(
    name = "python3-libs-0__3.9.25-10.el9.aarch64",
    sha256 = "319b68bd3b1e00fea12a2ed53931368ea44fc84db9057496d9cfa0671bafc563",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-libs-3.9.25-10.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/319b68bd3b1e00fea12a2ed53931368ea44fc84db9057496d9cfa0671bafc563",
    ],
)

rpm(
    name = "python3-libs-0__3.9.25-10.el9.s390x",
    sha256 = "e93bb284606370109cb6bc44c93ab9431953f505229e70349aac826a0070bf7a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-libs-3.9.25-10.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/e93bb284606370109cb6bc44c93ab9431953f505229e70349aac826a0070bf7a",
    ],
)

rpm(
    name = "python3-libs-0__3.9.25-10.el9.x86_64",
    sha256 = "15a8e78b5ed922254502d0605727dc9cb9edb63b8cdf0fb8a0448fccdc312cea",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-libs-3.9.25-10.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/15a8e78b5ed922254502d0605727dc9cb9edb63b8cdf0fb8a0448fccdc312cea",
    ],
)

rpm(
    name = "python3-ovirt-engine-sdk4-0__4.6.3-1.el10s.aarch64",
    sha256 = "530ca616e04771c9320516fd2f1821b5126f4fbd23b684a33e309fddc0ff4ee2",
    urls = [
        "https://mirror.stream.centos.org/SIGs/10-stream/virt/aarch64/ovirt-45/Packages/p/python3-ovirt-engine-sdk4-4.6.3-1.el10s.aarch64.rpm",
    ],
)

rpm(
    name = "python3-ovirt-engine-sdk4-0__4.6.3-1.el9.aarch64",
    sha256 = "fda97a26ffdcb3e6a136e7341d5c47a14dc1511a54a604709caa27c1fb096922",
    urls = [
        "https://mirror.stream.centos.org/SIGs/9-stream/virt/aarch64/ovirt-45/Packages/p/python3-ovirt-engine-sdk4-4.6.3-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/fda97a26ffdcb3e6a136e7341d5c47a14dc1511a54a604709caa27c1fb096922",
    ],
)

rpm(
    name = "python3-ovirt-engine-sdk4-0__4.6.4-0.1.master.20251015140243.el10.x86_64",
    sha256 = "d7f5addc274cda22c4c7f5ce8378bed7d671a3872a5726c2a53980d1cad69d4d",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-10-x86_64/Packages/p/python3-ovirt-engine-sdk4-4.6.4-0.1.master.20251015140243.el10.x86_64.rpm",
    ],
)

rpm(
    name = "python3-ovirt-engine-sdk4-0__4.6.4-0.1.master.20251015140243.el9.x86_64",
    sha256 = "163df111509d8620abb55bef796fb0754607c4735ad458e26691fe115d039a2e",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-9-x86_64/09691226-python-ovirt-engine-sdk4/python3-ovirt-engine-sdk4-4.6.4-0.1.master.20251015140243.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/163df111509d8620abb55bef796fb0754607c4735ad458e26691fe115d039a2e",
    ],
)

rpm(
    name = "python3-pip-wheel-0__21.3.1-2.el9.aarch64",
    sha256 = "c8a53917081942a659da7f98c64137c5a7aab2b25fc6cb948a3ce4bef0b59309",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-pip-wheel-21.3.1-2.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/c8a53917081942a659da7f98c64137c5a7aab2b25fc6cb948a3ce4bef0b59309",
    ],
)

rpm(
    name = "python3-pip-wheel-0__21.3.1-2.el9.s390x",
    sha256 = "c8a53917081942a659da7f98c64137c5a7aab2b25fc6cb948a3ce4bef0b59309",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-pip-wheel-21.3.1-2.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/c8a53917081942a659da7f98c64137c5a7aab2b25fc6cb948a3ce4bef0b59309",
    ],
)

rpm(
    name = "python3-pip-wheel-0__21.3.1-2.el9.x86_64",
    sha256 = "c8a53917081942a659da7f98c64137c5a7aab2b25fc6cb948a3ce4bef0b59309",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-pip-wheel-21.3.1-2.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/c8a53917081942a659da7f98c64137c5a7aab2b25fc6cb948a3ce4bef0b59309",
    ],
)

rpm(
    name = "python3-pip-wheel-0__23.3.2-11.el10.aarch64",
    sha256 = "6bcd3976b56848076c8be3cacd495d39049ebeacc3dfd8dc70975df2f21d078a",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/python3-pip-wheel-23.3.2-11.el10.noarch.rpm",
    ],
)

rpm(
    name = "python3-pip-wheel-0__23.3.2-11.el10.s390x",
    sha256 = "6bcd3976b56848076c8be3cacd495d39049ebeacc3dfd8dc70975df2f21d078a",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/python3-pip-wheel-23.3.2-11.el10.noarch.rpm",
    ],
)

rpm(
    name = "python3-pip-wheel-0__23.3.2-11.el10.x86_64",
    sha256 = "6bcd3976b56848076c8be3cacd495d39049ebeacc3dfd8dc70975df2f21d078a",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/python3-pip-wheel-23.3.2-11.el10.noarch.rpm",
    ],
)

rpm(
    name = "python3-pycurl-0__7.43.0.6-8.el9.s390x",
    sha256 = "8123a9e9ae677fea67e44edd76e00219be76b371ae62e4bd5f2a324f738aaebd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/python3-pycurl-7.43.0.6-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/8123a9e9ae677fea67e44edd76e00219be76b371ae62e4bd5f2a324f738aaebd",
    ],
)

rpm(
    name = "python3-pycurl-0__7.43.0.6-8.el9.x86_64",
    sha256 = "250c5fc154b79c97e5f66514b5b2335d52e879f932c863df157094ac87fc4fd1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/python3-pycurl-7.43.0.6-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/250c5fc154b79c97e5f66514b5b2335d52e879f932c863df157094ac87fc4fd1",
    ],
)

rpm(
    name = "python3-pycurl-0__7.45.2-2.1.el9.aarch64",
    sha256 = "73d128066d06d512547a42ed4d5bde098eb04b109de5112f1329444312a8c6ba",
    urls = [
        "https://mirror.stream.centos.org/SIGs/9-stream/virt/aarch64/ovirt-45/Packages/p/python3-pycurl-7.45.2-2.1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/73d128066d06d512547a42ed4d5bde098eb04b109de5112f1329444312a8c6ba",
    ],
)

rpm(
    name = "python3-pycurl-0__7.45.3-4.el10_0.aarch64",
    sha256 = "9fda00a07550aec7295bad052725fb7f578863a213c14b4788ce56be3a276ef4",
    urls = [
        "https://dl.fedoraproject.org/pub/epel/10/Everything/aarch64/Packages/p/python3-pycurl-7.45.3-4.el10_0.aarch64.rpm",
    ],
)

rpm(
    name = "python3-pycurl-0__7.45.3-4.el10_0.x86_64",
    sha256 = "5dcf95512d5e9a8c446d63dc126b67a8f3318e6cc585d8a2eb8b69cc75e2bbf4",
    urls = [
        "https://dl.fedoraproject.org/pub/epel/10/Everything/x86_64/Packages/p/python3-pycurl-7.45.3-4.el10_0.x86_64.rpm",
    ],
)

rpm(
    name = "python3-setuptools-wheel-0__53.0.0-15.el9.aarch64",
    sha256 = "4d61c666c3862bd18caebac2295c088627b47612f3367cd636fcaec9a021bbac",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-setuptools-wheel-53.0.0-15.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/4d61c666c3862bd18caebac2295c088627b47612f3367cd636fcaec9a021bbac",
    ],
)

rpm(
    name = "python3-setuptools-wheel-0__53.0.0-15.el9.s390x",
    sha256 = "4d61c666c3862bd18caebac2295c088627b47612f3367cd636fcaec9a021bbac",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-setuptools-wheel-53.0.0-15.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/4d61c666c3862bd18caebac2295c088627b47612f3367cd636fcaec9a021bbac",
    ],
)

rpm(
    name = "python3-setuptools-wheel-0__53.0.0-15.el9.x86_64",
    sha256 = "4d61c666c3862bd18caebac2295c088627b47612f3367cd636fcaec9a021bbac",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-setuptools-wheel-53.0.0-15.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/4d61c666c3862bd18caebac2295c088627b47612f3367cd636fcaec9a021bbac",
    ],
)

rpm(
    name = "python3-six-0__1.15.0-9.el9.aarch64",
    sha256 = "efecffed29602079a1ea1d41c819271ec705a97a68891b43e1d626b2fa0ea8a1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-six-1.15.0-9.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/efecffed29602079a1ea1d41c819271ec705a97a68891b43e1d626b2fa0ea8a1",
    ],
)

rpm(
    name = "python3-six-0__1.15.0-9.el9.s390x",
    sha256 = "efecffed29602079a1ea1d41c819271ec705a97a68891b43e1d626b2fa0ea8a1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-six-1.15.0-9.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/efecffed29602079a1ea1d41c819271ec705a97a68891b43e1d626b2fa0ea8a1",
    ],
)

rpm(
    name = "python3-six-0__1.15.0-9.el9.x86_64",
    sha256 = "efecffed29602079a1ea1d41c819271ec705a97a68891b43e1d626b2fa0ea8a1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-six-1.15.0-9.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/efecffed29602079a1ea1d41c819271ec705a97a68891b43e1d626b2fa0ea8a1",
    ],
)

rpm(
    name = "python3-six-0__1.16.0-16.el10.aarch64",
    sha256 = "587391f25be67ed7389c4623f1260a16b33dfab99b5b7376e9eb72dafbc78403",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/python3-six-1.16.0-16.el10.noarch.rpm",
    ],
)

rpm(
    name = "python3-six-0__1.16.0-16.el10.s390x",
    sha256 = "587391f25be67ed7389c4623f1260a16b33dfab99b5b7376e9eb72dafbc78403",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/python3-six-1.16.0-16.el10.noarch.rpm",
    ],
)

rpm(
    name = "python3-six-0__1.16.0-16.el10.x86_64",
    sha256 = "587391f25be67ed7389c4623f1260a16b33dfab99b5b7376e9eb72dafbc78403",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/python3-six-1.16.0-16.el10.noarch.rpm",
    ],
)

rpm(
    name = "python3-systemd-0__234-19.el9.aarch64",
    sha256 = "c5bc7ec403ee44fe1e479d392f87309c0ab0c86632c3515cc867882e82bbd679",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-systemd-234-19.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c5bc7ec403ee44fe1e479d392f87309c0ab0c86632c3515cc867882e82bbd679",
    ],
)

rpm(
    name = "python3-systemd-0__234-19.el9.s390x",
    sha256 = "fb74b14bf11fa996a2ef540126119febcb8aa2eeb3a855dffb1eba481b07c878",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-systemd-234-19.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/fb74b14bf11fa996a2ef540126119febcb8aa2eeb3a855dffb1eba481b07c878",
    ],
)

rpm(
    name = "python3-systemd-0__234-19.el9.x86_64",
    sha256 = "10ce18f02053671942ae5dc165c95cb195a50c309b90159e006214da2c953ea0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-systemd-234-19.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/10ce18f02053671942ae5dc165c95cb195a50c309b90159e006214da2c953ea0",
    ],
)

rpm(
    name = "python3-systemd-0__235-11.el10.aarch64",
    sha256 = "093fbc9e1b510a2e57e46d5ae99a8da0dbc98d7c5a4e1c28ba9b34f1933e1c38",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/python3-systemd-235-11.el10.aarch64.rpm",
    ],
)

rpm(
    name = "python3-systemd-0__235-11.el10.s390x",
    sha256 = "66d8f9459969127212c14cea5e322030ff3539ffa84ca87b56119d6fd3581ad9",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/python3-systemd-235-11.el10.s390x.rpm",
    ],
)

rpm(
    name = "python3-systemd-0__235-11.el10.x86_64",
    sha256 = "737a49555845e7c8ecefc1625fba516845dd8b56e4c0c2fa8584e26b1b0539ef",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/python3-systemd-235-11.el10.x86_64.rpm",
    ],
)

rpm(
    name = "qemu-img-17__10.1.0-24.el9.aarch64",
    sha256 = "cbd2fff19674514234e1ab9b4e8651c5f5a07d454d5b2879da7ceb9aa9e72af2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/qemu-img-10.1.0-24.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/cbd2fff19674514234e1ab9b4e8651c5f5a07d454d5b2879da7ceb9aa9e72af2",
    ],
)

rpm(
    name = "qemu-img-17__10.1.0-24.el9.s390x",
    sha256 = "64dd994fd3fb412505929e544b243324441ad3329496f4671844a5f49ede7310",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/qemu-img-10.1.0-24.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/64dd994fd3fb412505929e544b243324441ad3329496f4671844a5f49ede7310",
    ],
)

rpm(
    name = "qemu-img-17__10.1.0-24.el9.x86_64",
    sha256 = "fd88b8ca07fedd72132dd20f0655b50b25141e902db3617ffed025fd106240dd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-img-10.1.0-24.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/fd88b8ca07fedd72132dd20f0655b50b25141e902db3617ffed025fd106240dd",
    ],
)

rpm(
    name = "qemu-img-17__6.2.0-12.el9.aarch64",
    sha256 = "af1a47580fc30e1b139f69e37ba37a03843b86cbe79d68403bdb3ace0978e18b",
    urls = ["https://storage.googleapis.com/builddeps/af1a47580fc30e1b139f69e37ba37a03843b86cbe79d68403bdb3ace0978e18b"],
)

rpm(
    name = "qemu-img-17__6.2.0-12.el9.x86_64",
    sha256 = "895ec7a5139022b1601f1b7ce7235bac7131b9c9a77ab6c2638700e6ea268437",
    urls = ["https://storage.googleapis.com/builddeps/895ec7a5139022b1601f1b7ce7235bac7131b9c9a77ab6c2638700e6ea268437"],
)

rpm(
    name = "qemu-img-18__10.1.0-28.el10.aarch64",
    sha256 = "72019fde88fbcefe89fc460550453e207aed050552b9109829a24cd4ddb74205",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/aarch64/os/Packages/qemu-img-10.1.0-28.el10.aarch64.rpm",
    ],
)

rpm(
    name = "qemu-img-18__10.1.0-28.el10.s390x",
    sha256 = "d97c3aa3b316fd5ffec2d3fc15b4ffe26f06b1388a4522b836d2a02e9d9080fc",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/s390x/os/Packages/qemu-img-10.1.0-28.el10.s390x.rpm",
    ],
)

rpm(
    name = "qemu-img-18__10.1.0-28.el10.x86_64",
    sha256 = "547063053056e139c11c822e899f20f7d2deb6d05938040bb308c7a4277d9b75",
    urls = [
        "http://mirror.stream.centos.org/10-stream/AppStream/x86_64/os/Packages/qemu-img-10.1.0-28.el10.x86_64.rpm",
    ],
)

rpm(
    name = "readline-0__8.1-4.el9.aarch64",
    sha256 = "2ecec47a882ff434cc869b691a7e1e8d7639bc1af44bcb214ff4921f675776aa",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/readline-8.1-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/2ecec47a882ff434cc869b691a7e1e8d7639bc1af44bcb214ff4921f675776aa",
    ],
)

rpm(
    name = "readline-0__8.1-4.el9.s390x",
    sha256 = "7b4b6f641f65d99d33ccbefaf4fbfe25a146d80213d359940779be4ad29569a8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/readline-8.1-4.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/7b4b6f641f65d99d33ccbefaf4fbfe25a146d80213d359940779be4ad29569a8",
    ],
)

rpm(
    name = "readline-0__8.1-4.el9.x86_64",
    sha256 = "49945472925286ad89b0575657b43f9224777e36b442f0c88df67f0b61e26aee",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/readline-8.1-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/49945472925286ad89b0575657b43f9224777e36b442f0c88df67f0b61e26aee",
    ],
)

rpm(
    name = "readline-0__8.2-11.el10.aarch64",
    sha256 = "a1f1fe411d40cb802c7a3e3b105faffe05c2376563bec2d59c71ed28778684cd",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/readline-8.2-11.el10.aarch64.rpm",
    ],
)

rpm(
    name = "readline-0__8.2-11.el10.s390x",
    sha256 = "fd8cc3c7dd19bf773afb0488b2521d1710dfc1d8337fbff55c9f8d84572a4f9f",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/readline-8.2-11.el10.s390x.rpm",
    ],
)

rpm(
    name = "readline-0__8.2-11.el10.x86_64",
    sha256 = "d8e2d7c011d0e5c56b6875919ce036605862db02d59d6983d470bc5757021783",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/readline-8.2-11.el10.x86_64.rpm",
    ],
)

rpm(
    name = "sed-0__4.8-10.el9.aarch64",
    sha256 = "5a2930318f5ca770e800b2a42c05c945ccb02cd8ea3ed2b177d759d0e9090d5d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/sed-4.8-10.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/5a2930318f5ca770e800b2a42c05c945ccb02cd8ea3ed2b177d759d0e9090d5d",
    ],
)

rpm(
    name = "sed-0__4.8-10.el9.s390x",
    sha256 = "a515c69e92880844e6fbcf690421bd0d44304b642e5e56392a00ede362da5056",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/sed-4.8-10.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/a515c69e92880844e6fbcf690421bd0d44304b642e5e56392a00ede362da5056",
    ],
)

rpm(
    name = "sed-0__4.8-10.el9.x86_64",
    sha256 = "8db670e1de34148e71c07f4ed8dbd5f41e1d6717325d5912a8651aa4e063b9e7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/sed-4.8-10.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/8db670e1de34148e71c07f4ed8dbd5f41e1d6717325d5912a8651aa4e063b9e7",
    ],
)

rpm(
    name = "sed-0__4.8-9.el9.aarch64",
    sha256 = "cfdec0f026af984c11277ae613f16af7a86ea6170aac3da495a027599fdc8e3d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/sed-4.8-9.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/cfdec0f026af984c11277ae613f16af7a86ea6170aac3da495a027599fdc8e3d",
    ],
)

rpm(
    name = "sed-0__4.8-9.el9.x86_64",
    sha256 = "a2c5d9a7f569abb5a592df1c3aaff0441bf827c9d0e2df0ab42b6c443dbc475f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/sed-4.8-9.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/a2c5d9a7f569abb5a592df1c3aaff0441bf827c9d0e2df0ab42b6c443dbc475f",
    ],
)

rpm(
    name = "sed-0__4.9-5.el10.aarch64",
    sha256 = "9c03d6148a319111bce62ba46e859c17c6615f51648788b499e28cbb429d1390",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/sed-4.9-5.el10.aarch64.rpm",
    ],
)

rpm(
    name = "sed-0__4.9-5.el10.s390x",
    sha256 = "428403e0e45d47510ffc7bebbe47a7e981bbe5347432f4b055e23c199482cbd3",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/sed-4.9-5.el10.s390x.rpm",
    ],
)

rpm(
    name = "sed-0__4.9-5.el10.x86_64",
    sha256 = "b5c61c0b1b90892375c36acaf52f8a19e68397a18703687df8b1883792485b7c",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/sed-4.9-5.el10.x86_64.rpm",
    ],
)

rpm(
    name = "setup-0__2.13.7-10.el9.aarch64",
    sha256 = "42a1c5a415c44e3b55551f49595c087e2ba55f0fd9ece8056b791983601b76d2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/setup-2.13.7-10.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/42a1c5a415c44e3b55551f49595c087e2ba55f0fd9ece8056b791983601b76d2",
    ],
)

rpm(
    name = "setup-0__2.13.7-10.el9.s390x",
    sha256 = "42a1c5a415c44e3b55551f49595c087e2ba55f0fd9ece8056b791983601b76d2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/setup-2.13.7-10.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/42a1c5a415c44e3b55551f49595c087e2ba55f0fd9ece8056b791983601b76d2",
    ],
)

rpm(
    name = "setup-0__2.13.7-10.el9.x86_64",
    sha256 = "42a1c5a415c44e3b55551f49595c087e2ba55f0fd9ece8056b791983601b76d2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/setup-2.13.7-10.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/42a1c5a415c44e3b55551f49595c087e2ba55f0fd9ece8056b791983601b76d2",
    ],
)

rpm(
    name = "setup-0__2.13.7-6.el9.aarch64",
    sha256 = "c0202712e8ec928cf61f3d777f23859ba6de2e85786e928ee5472fdde570aeee",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/setup-2.13.7-6.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/c0202712e8ec928cf61f3d777f23859ba6de2e85786e928ee5472fdde570aeee",
    ],
)

rpm(
    name = "setup-0__2.13.7-6.el9.x86_64",
    sha256 = "c0202712e8ec928cf61f3d777f23859ba6de2e85786e928ee5472fdde570aeee",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/setup-2.13.7-6.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/c0202712e8ec928cf61f3d777f23859ba6de2e85786e928ee5472fdde570aeee",
    ],
)

rpm(
    name = "setup-0__2.14.5-7.el10.aarch64",
    sha256 = "bd7fb604e635ec8e49abc330cb15e9f30dcc1c6f248495308acd83e41896b29e",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/setup-2.14.5-7.el10.noarch.rpm",
    ],
)

rpm(
    name = "setup-0__2.14.5-7.el10.s390x",
    sha256 = "bd7fb604e635ec8e49abc330cb15e9f30dcc1c6f248495308acd83e41896b29e",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/setup-2.14.5-7.el10.noarch.rpm",
    ],
)

rpm(
    name = "setup-0__2.14.5-7.el10.x86_64",
    sha256 = "bd7fb604e635ec8e49abc330cb15e9f30dcc1c6f248495308acd83e41896b29e",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/setup-2.14.5-7.el10.noarch.rpm",
    ],
)

rpm(
    name = "shadow-utils-2__4.15.0-13.el10.aarch64",
    sha256 = "a55f9afb87de4255e90deba2e45dfdd0d384ea23095ec667b7f6bc25faf4b6a4",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/shadow-utils-4.15.0-13.el10.aarch64.rpm",
    ],
)

rpm(
    name = "shadow-utils-2__4.15.0-13.el10.s390x",
    sha256 = "405cb51e9e6312ecb997e0f63a24ac5109e4ddcd559c78a8e906fdd37c9fb51b",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/shadow-utils-4.15.0-13.el10.s390x.rpm",
    ],
)

rpm(
    name = "shadow-utils-2__4.15.0-13.el10.x86_64",
    sha256 = "f2827e8ef5b980b81720b85300a6a1162e09fe29f26f4d596e5bc060a22149b1",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/shadow-utils-4.15.0-13.el10.x86_64.rpm",
    ],
)

rpm(
    name = "shadow-utils-2__4.9-17.el9.aarch64",
    sha256 = "3edd4c583815a1e74b05972137144264bd2fe062106f63697fddffc4a5fc957d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/shadow-utils-4.9-17.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/3edd4c583815a1e74b05972137144264bd2fe062106f63697fddffc4a5fc957d",
    ],
)

rpm(
    name = "shadow-utils-2__4.9-17.el9.s390x",
    sha256 = "df349811eb3501d6653321753b4bd37f15c69a024f9601208c974b53058b66ae",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/shadow-utils-4.9-17.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/df349811eb3501d6653321753b4bd37f15c69a024f9601208c974b53058b66ae",
    ],
)

rpm(
    name = "shadow-utils-2__4.9-17.el9.x86_64",
    sha256 = "1b9b0829668ce68f0ff0904fa651005e9c0c5e53b7481adb41f8f6b758d9e36a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/shadow-utils-4.9-17.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/1b9b0829668ce68f0ff0904fa651005e9c0c5e53b7481adb41f8f6b758d9e36a",
    ],
)

rpm(
    name = "shadow-utils-subid-2__4.15.0-13.el10.aarch64",
    sha256 = "2cd68936aadd6edba7f191f88023c49b1ca103a56aed358c6f4d6de9234b82e5",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/shadow-utils-subid-4.15.0-13.el10.aarch64.rpm",
    ],
)

rpm(
    name = "shadow-utils-subid-2__4.15.0-13.el10.s390x",
    sha256 = "b3a8abe280660eb6b6403ffe6371121438270acedf1d1e0ad3f956ab319764c0",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/shadow-utils-subid-4.15.0-13.el10.s390x.rpm",
    ],
)

rpm(
    name = "shadow-utils-subid-2__4.15.0-13.el10.x86_64",
    sha256 = "de879db8d99a2f0e73da16bc43f9e239fa206f7d5115e80a9053a595b7cb5f8e",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/shadow-utils-subid-4.15.0-13.el10.x86_64.rpm",
    ],
)

rpm(
    name = "shadow-utils-subid-2__4.9-17.el9.aarch64",
    sha256 = "28b8b6e8ec917190a43d14893afa977b4cf01c26422f3686e474c4bfb668c28d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/shadow-utils-subid-4.9-17.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/28b8b6e8ec917190a43d14893afa977b4cf01c26422f3686e474c4bfb668c28d",
    ],
)

rpm(
    name = "shadow-utils-subid-2__4.9-17.el9.s390x",
    sha256 = "d0b301c682c784d471138040de9c74e372ae0ffcf2dba8d9925d41ccd417391f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/shadow-utils-subid-4.9-17.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/d0b301c682c784d471138040de9c74e372ae0ffcf2dba8d9925d41ccd417391f",
    ],
)

rpm(
    name = "shadow-utils-subid-2__4.9-17.el9.x86_64",
    sha256 = "8792e56a4a0502fce0a15fdbe2eadcdef1a467874666a2ca15ad95a3112878c1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/shadow-utils-subid-4.9-17.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/8792e56a4a0502fce0a15fdbe2eadcdef1a467874666a2ca15ad95a3112878c1",
    ],
)

rpm(
    name = "sqlite-libs-0__3.34.1-11.el9.aarch64",
    sha256 = "2f65e7f2b71d9f290b3847fdc7aec651e61f867493c8fafa093f94591723f7c5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/sqlite-libs-3.34.1-11.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/2f65e7f2b71d9f290b3847fdc7aec651e61f867493c8fafa093f94591723f7c5",
    ],
)

rpm(
    name = "sqlite-libs-0__3.34.1-11.el9.s390x",
    sha256 = "ecf4aaf07ad4cf53b96bf0f9a597936bb333091a3faa4255996227ff0c2073c8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/sqlite-libs-3.34.1-11.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/ecf4aaf07ad4cf53b96bf0f9a597936bb333091a3faa4255996227ff0c2073c8",
    ],
)

rpm(
    name = "sqlite-libs-0__3.34.1-11.el9.x86_64",
    sha256 = "dfdc4f315ec0723e6c2687f810bcc5a9d3cb83f4d73496734645123bd5a6f3d4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/sqlite-libs-3.34.1-11.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/dfdc4f315ec0723e6c2687f810bcc5a9d3cb83f4d73496734645123bd5a6f3d4",
    ],
)

rpm(
    name = "sqlite-libs-0__3.46.1-6.el10.aarch64",
    sha256 = "4ad7d280241abdbc1dcb180a26d686ce991278fc75374844275eb9d19e7d0163",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/sqlite-libs-3.46.1-6.el10.aarch64.rpm",
    ],
)

rpm(
    name = "sqlite-libs-0__3.46.1-6.el10.s390x",
    sha256 = "d042a60cce2dd376a154963dca3e04464468ac52b00350769461b85754918345",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/sqlite-libs-3.46.1-6.el10.s390x.rpm",
    ],
)

rpm(
    name = "sqlite-libs-0__3.46.1-6.el10.x86_64",
    sha256 = "3d9907ff33bfd5fa0d1ab63b637f5b6ea6e8a30a19ab91dc0c1f135558e003dd",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/sqlite-libs-3.46.1-6.el10.x86_64.rpm",
    ],
)

rpm(
    name = "systemd-0__252-78.el9.aarch64",
    sha256 = "c97950426da2186056fdf1d1900ac2c4ade8cd8db44072a8865bc52cf1b46e50",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-252-78.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c97950426da2186056fdf1d1900ac2c4ade8cd8db44072a8865bc52cf1b46e50",
    ],
)

rpm(
    name = "systemd-0__252-78.el9.s390x",
    sha256 = "7342457c7ed8515b806f156170ce6493382f73f59d058d970e5b21c233207558",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-252-78.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/7342457c7ed8515b806f156170ce6493382f73f59d058d970e5b21c233207558",
    ],
)

rpm(
    name = "systemd-0__252-78.el9.x86_64",
    sha256 = "8928a714a1804b3f1e5cef750d2f3c10f3b2054c009aa11a5cb5747eb3b174a6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-252-78.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/8928a714a1804b3f1e5cef750d2f3c10f3b2054c009aa11a5cb5747eb3b174a6",
    ],
)

rpm(
    name = "systemd-0__257-35.el10.aarch64",
    sha256 = "6f3c5e943ce4cba7b5c0bb137c833ffe0f244d45a63bc5b767f1b8f7fbaf39ee",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/systemd-257-35.el10.aarch64.rpm",
    ],
)

rpm(
    name = "systemd-0__257-35.el10.s390x",
    sha256 = "8efc191df6c9fcff8b24a0557f88ea50b9286b34a3b4290fed0ca64363517b4a",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/systemd-257-35.el10.s390x.rpm",
    ],
)

rpm(
    name = "systemd-0__257-35.el10.x86_64",
    sha256 = "1ccc2b40ec4a86f03098558438e67827e1f1630914094cf8a7e6e0098e9308e9",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/systemd-257-35.el10.x86_64.rpm",
    ],
)

rpm(
    name = "systemd-libs-0__250-4.el9.aarch64",
    sha256 = "0afc6fc8e96fb76f2183774bf309efb5bef2c0f85b68f351bece3e0385f08106",
    urls = ["https://storage.googleapis.com/builddeps/0afc6fc8e96fb76f2183774bf309efb5bef2c0f85b68f351bece3e0385f08106"],
)

rpm(
    name = "systemd-libs-0__250-4.el9.x86_64",
    sha256 = "f0a57df3dcea7a138470ffb9a4e5201edf807ce4082730dd9f0e886435df7ced",
    urls = ["https://storage.googleapis.com/builddeps/f0a57df3dcea7a138470ffb9a4e5201edf807ce4082730dd9f0e886435df7ced"],
)

rpm(
    name = "systemd-libs-0__252-78.el9.aarch64",
    sha256 = "176eab1534229e86c93e94630da085287e0a93d260bae66503b4167632eda244",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-libs-252-78.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/176eab1534229e86c93e94630da085287e0a93d260bae66503b4167632eda244",
    ],
)

rpm(
    name = "systemd-libs-0__252-78.el9.s390x",
    sha256 = "ec9a39e00b29d8b65fd7ca5efbeea13c719b7611451f5c7bdae0aa39622d0240",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-libs-252-78.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/ec9a39e00b29d8b65fd7ca5efbeea13c719b7611451f5c7bdae0aa39622d0240",
    ],
)

rpm(
    name = "systemd-libs-0__252-78.el9.x86_64",
    sha256 = "10cf9e4fe79afee84cf06ea989fcb2544ada5f39fda301cda81efd970381808d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-libs-252-78.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/10cf9e4fe79afee84cf06ea989fcb2544ada5f39fda301cda81efd970381808d",
    ],
)

rpm(
    name = "systemd-libs-0__257-35.el10.aarch64",
    sha256 = "e34c1a15c862d024380283f9884f950db9d5f5494774499d69531f256676a136",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/systemd-libs-257-35.el10.aarch64.rpm",
    ],
)

rpm(
    name = "systemd-libs-0__257-35.el10.s390x",
    sha256 = "85f5a3dc4848edfdc8ff9c02cfbcd37eb2dfb7e22847824e35d9d5c65c04e0be",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/systemd-libs-257-35.el10.s390x.rpm",
    ],
)

rpm(
    name = "systemd-libs-0__257-35.el10.x86_64",
    sha256 = "3c4759223466e65e2cc3c22c46f96247ce2ab94acd6d2087d43fe9845174057d",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/systemd-libs-257-35.el10.x86_64.rpm",
    ],
)

rpm(
    name = "systemd-pam-0__252-78.el9.aarch64",
    sha256 = "6845e3290cf7ce8085ea6f160681070df506b031deb1a2d1acd79b36ce5a00d6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-pam-252-78.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/6845e3290cf7ce8085ea6f160681070df506b031deb1a2d1acd79b36ce5a00d6",
    ],
)

rpm(
    name = "systemd-pam-0__252-78.el9.s390x",
    sha256 = "ea09ff68aab5229212b8167c1aefb2d6b4ff648a524df2adbe5c9dfad8c315c4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-pam-252-78.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/ea09ff68aab5229212b8167c1aefb2d6b4ff648a524df2adbe5c9dfad8c315c4",
    ],
)

rpm(
    name = "systemd-pam-0__252-78.el9.x86_64",
    sha256 = "a1fce5fb7bf0c6ac86e738dbb21c5e02361015f1858d22b0c368de76e8a152c7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-pam-252-78.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/a1fce5fb7bf0c6ac86e738dbb21c5e02361015f1858d22b0c368de76e8a152c7",
    ],
)

rpm(
    name = "systemd-pam-0__257-35.el10.aarch64",
    sha256 = "bc174fe2c5898bf0e6fc13b3197f0a254e51ad2e2f78449d92b3d515b0837b5b",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/systemd-pam-257-35.el10.aarch64.rpm",
    ],
)

rpm(
    name = "systemd-pam-0__257-35.el10.s390x",
    sha256 = "d1eca24a7fb98a6fca62a3a220fd0a86dc98e50508ae10bb61462b7bd17730ed",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/systemd-pam-257-35.el10.s390x.rpm",
    ],
)

rpm(
    name = "systemd-pam-0__257-35.el10.x86_64",
    sha256 = "d0e84e8620beba4146c946187d648ff22a4a393bb614de5fb3d993c520c05e44",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/systemd-pam-257-35.el10.x86_64.rpm",
    ],
)

rpm(
    name = "systemd-rpm-macros-0__252-78.el9.aarch64",
    sha256 = "a0ee77264681d14a9298dc6d9e3ca4c4abb4a546619d2075b6c666e376cf78d0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-rpm-macros-252-78.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/a0ee77264681d14a9298dc6d9e3ca4c4abb4a546619d2075b6c666e376cf78d0",
    ],
)

rpm(
    name = "systemd-rpm-macros-0__252-78.el9.s390x",
    sha256 = "a0ee77264681d14a9298dc6d9e3ca4c4abb4a546619d2075b6c666e376cf78d0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-rpm-macros-252-78.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/a0ee77264681d14a9298dc6d9e3ca4c4abb4a546619d2075b6c666e376cf78d0",
    ],
)

rpm(
    name = "systemd-rpm-macros-0__252-78.el9.x86_64",
    sha256 = "a0ee77264681d14a9298dc6d9e3ca4c4abb4a546619d2075b6c666e376cf78d0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-rpm-macros-252-78.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/a0ee77264681d14a9298dc6d9e3ca4c4abb4a546619d2075b6c666e376cf78d0",
    ],
)

rpm(
    name = "tar-2__1.34-13.el9.aarch64",
    sha256 = "1a76f91090e909c3cac78d572e528a9047ffb9b4df80bb2c847869fe79c4ba55",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/tar-1.34-13.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/1a76f91090e909c3cac78d572e528a9047ffb9b4df80bb2c847869fe79c4ba55",
    ],
)

rpm(
    name = "tar-2__1.34-13.el9.s390x",
    sha256 = "c2c23987237142f81360eb75f4bd8f6b69b24051116e565470be56a5e0f059e2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/tar-1.34-13.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/c2c23987237142f81360eb75f4bd8f6b69b24051116e565470be56a5e0f059e2",
    ],
)

rpm(
    name = "tar-2__1.34-13.el9.x86_64",
    sha256 = "f2b2023c1bf3f527851ff07c860719c70e5fed850377fee92ca599fd64aa5e1a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/tar-1.34-13.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f2b2023c1bf3f527851ff07c860719c70e5fed850377fee92ca599fd64aa5e1a",
    ],
)

rpm(
    name = "tar-2__1.35-13.el10.aarch64",
    sha256 = "b3b1990cd895df38ee302659c377d7ef6af29c863ba10b399f2f254047556aaf",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/tar-1.35-13.el10.aarch64.rpm",
    ],
)

rpm(
    name = "tar-2__1.35-13.el10.s390x",
    sha256 = "fa4a7727cd0d171a30a1e3bca1628d97af49ba50b0c9933d7574094b310aa42e",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/tar-1.35-13.el10.s390x.rpm",
    ],
)

rpm(
    name = "tar-2__1.35-13.el10.x86_64",
    sha256 = "b29aed04bffe8607b22ee6cbb2cedfb3ffaba904b3aa86baf0f59203432a9029",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/tar-1.35-13.el10.x86_64.rpm",
    ],
)

rpm(
    name = "tpm2-tss-0__4.1.3-6.el10.aarch64",
    sha256 = "22bdfcc4af5dd47fa52bad5f8ebca5d80f6caa6e548ad18c7c037899195d4bd3",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/tpm2-tss-4.1.3-6.el10.aarch64.rpm",
    ],
)

rpm(
    name = "tpm2-tss-0__4.1.3-6.el10.s390x",
    sha256 = "6c572b1029f932a26e9b4e6f15f2ab3554c13c0ec041ba9ffd4b525514ddee7d",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/tpm2-tss-4.1.3-6.el10.s390x.rpm",
    ],
)

rpm(
    name = "tpm2-tss-0__4.1.3-6.el10.x86_64",
    sha256 = "95f4b1e56a2c511de79a18c0dd24a03b22175e8cc3d79c9cbd4eb4fd62207fbc",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/tpm2-tss-4.1.3-6.el10.x86_64.rpm",
    ],
)

rpm(
    name = "tzdata-0__2021e-1.el9.aarch64",
    sha256 = "42d89577a0f887c4baa162250862dea2c1830b1ced56c45ced9645ad8e2a3671",
    urls = ["https://storage.googleapis.com/builddeps/42d89577a0f887c4baa162250862dea2c1830b1ced56c45ced9645ad8e2a3671"],
)

rpm(
    name = "tzdata-0__2021e-1.el9.x86_64",
    sha256 = "42d89577a0f887c4baa162250862dea2c1830b1ced56c45ced9645ad8e2a3671",
    urls = ["https://storage.googleapis.com/builddeps/42d89577a0f887c4baa162250862dea2c1830b1ced56c45ced9645ad8e2a3671"],
)

rpm(
    name = "tzdata-0__2026c-1.el9.aarch64",
    sha256 = "405079b128b6a4e6654df756538bba2095a49c3fbaf2f299e92bf60df0a209f6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/tzdata-2026c-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/405079b128b6a4e6654df756538bba2095a49c3fbaf2f299e92bf60df0a209f6",
    ],
)

rpm(
    name = "tzdata-0__2026c-1.el9.s390x",
    sha256 = "405079b128b6a4e6654df756538bba2095a49c3fbaf2f299e92bf60df0a209f6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/tzdata-2026c-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/405079b128b6a4e6654df756538bba2095a49c3fbaf2f299e92bf60df0a209f6",
    ],
)

rpm(
    name = "tzdata-0__2026c-1.el9.x86_64",
    sha256 = "405079b128b6a4e6654df756538bba2095a49c3fbaf2f299e92bf60df0a209f6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/tzdata-2026c-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/405079b128b6a4e6654df756538bba2095a49c3fbaf2f299e92bf60df0a209f6",
    ],
)

rpm(
    name = "tzdata-0__2026f-1.el10.aarch64",
    sha256 = "3afc30b9c58eb8a88995609587a673d14a9ad8fe57c3830897e8e5eb82a08c08",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/tzdata-2026f-1.el10.noarch.rpm",
    ],
)

rpm(
    name = "tzdata-0__2026f-1.el10.s390x",
    sha256 = "3afc30b9c58eb8a88995609587a673d14a9ad8fe57c3830897e8e5eb82a08c08",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/tzdata-2026f-1.el10.noarch.rpm",
    ],
)

rpm(
    name = "tzdata-0__2026f-1.el10.x86_64",
    sha256 = "3afc30b9c58eb8a88995609587a673d14a9ad8fe57c3830897e8e5eb82a08c08",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/tzdata-2026f-1.el10.noarch.rpm",
    ],
)

rpm(
    name = "util-linux-0__2.37.4-27.el9.aarch64",
    sha256 = "5e38d7b9b460d714e833889d1c49272feb40f0eb190ae936b5a0265d95c51df2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/util-linux-2.37.4-27.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/5e38d7b9b460d714e833889d1c49272feb40f0eb190ae936b5a0265d95c51df2",
    ],
)

rpm(
    name = "util-linux-0__2.37.4-27.el9.s390x",
    sha256 = "693d363ed7d1f5f296468281f3c1110261b3a4619b4a839d9e5eb96a9802d57b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/util-linux-2.37.4-27.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/693d363ed7d1f5f296468281f3c1110261b3a4619b4a839d9e5eb96a9802d57b",
    ],
)

rpm(
    name = "util-linux-0__2.37.4-27.el9.x86_64",
    sha256 = "3cfa4add546aa6f87319c685980c1e451e9772d12fcd95cea6237288cc299eb7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/util-linux-2.37.4-27.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/3cfa4add546aa6f87319c685980c1e451e9772d12fcd95cea6237288cc299eb7",
    ],
)

rpm(
    name = "util-linux-core-0__2.37.2-1.el9.aarch64",
    sha256 = "5bd360c94d20a11bac665b634569fc2597eab88280d88cd5b71be853e8331e14",
    urls = ["https://storage.googleapis.com/builddeps/5bd360c94d20a11bac665b634569fc2597eab88280d88cd5b71be853e8331e14"],
)

rpm(
    name = "util-linux-core-0__2.37.2-1.el9.x86_64",
    sha256 = "0313682867c1d07785a6d02ff87e1899f484bd1ce6348fa5c673eca78c0da2bd",
    urls = ["https://storage.googleapis.com/builddeps/0313682867c1d07785a6d02ff87e1899f484bd1ce6348fa5c673eca78c0da2bd"],
)

rpm(
    name = "util-linux-core-0__2.37.4-27.el9.aarch64",
    sha256 = "30796f54ae7f633cc724739c9ae147b9cecfd55bf92b980d8461c394ce0a05b0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/util-linux-core-2.37.4-27.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/30796f54ae7f633cc724739c9ae147b9cecfd55bf92b980d8461c394ce0a05b0",
    ],
)

rpm(
    name = "util-linux-core-0__2.37.4-27.el9.s390x",
    sha256 = "5cba0141e9c9c1e11718ce53bf2e62e2a0140481b9c6f1f90b5406dcf0c4ca70",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/util-linux-core-2.37.4-27.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/5cba0141e9c9c1e11718ce53bf2e62e2a0140481b9c6f1f90b5406dcf0c4ca70",
    ],
)

rpm(
    name = "util-linux-core-0__2.37.4-27.el9.x86_64",
    sha256 = "dafdde295a33cea8ac397372808d2a958020790045be6cf1fefae1ce81919a77",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/util-linux-core-2.37.4-27.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/dafdde295a33cea8ac397372808d2a958020790045be6cf1fefae1ce81919a77",
    ],
)

rpm(
    name = "util-linux-core-0__2.40.2-23.el10.aarch64",
    sha256 = "46e67075de97b6dd93bc99afc318cdac85cfed32d53bbb40d08c296aff55b383",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/util-linux-core-2.40.2-23.el10.aarch64.rpm",
    ],
)

rpm(
    name = "util-linux-core-0__2.40.2-23.el10.s390x",
    sha256 = "5cd04a6ac5da5594cfe68c700f53d7d2a2216c600523411a9ea491e50c2ddce5",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/util-linux-core-2.40.2-23.el10.s390x.rpm",
    ],
)

rpm(
    name = "util-linux-core-0__2.40.2-23.el10.x86_64",
    sha256 = "0991fe80a7ec5ade43533b765b8293c7f9b4edb9d581b5a4789f0639154a5200",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/util-linux-core-2.40.2-23.el10.x86_64.rpm",
    ],
)

rpm(
    name = "vim-data-2__9.1.083-26.el10.aarch64",
    sha256 = "b10001580c44903a9c91495ac417760adb37e28faace0b9cb39c31506f347e45",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/vim-data-9.1.083-26.el10.noarch.rpm",
    ],
)

rpm(
    name = "vim-data-2__9.1.083-26.el10.s390x",
    sha256 = "b10001580c44903a9c91495ac417760adb37e28faace0b9cb39c31506f347e45",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/vim-data-9.1.083-26.el10.noarch.rpm",
    ],
)

rpm(
    name = "vim-data-2__9.1.083-26.el10.x86_64",
    sha256 = "b10001580c44903a9c91495ac417760adb37e28faace0b9cb39c31506f347e45",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/vim-data-9.1.083-26.el10.noarch.rpm",
    ],
)

rpm(
    name = "vim-minimal-2__8.2.2637-15.el9.aarch64",
    sha256 = "14136f426b9425d7c66bc6a5cace746b84b0bcf436e58144d782d993998da7da",
    urls = ["https://storage.googleapis.com/builddeps/14136f426b9425d7c66bc6a5cace746b84b0bcf436e58144d782d993998da7da"],
)

rpm(
    name = "vim-minimal-2__8.2.2637-15.el9.x86_64",
    sha256 = "062a1b85ecad3a9ea41e39f268f5660c1e6262999339fc18e77c797101b96461",
    urls = ["https://storage.googleapis.com/builddeps/062a1b85ecad3a9ea41e39f268f5660c1e6262999339fc18e77c797101b96461"],
)

rpm(
    name = "vim-minimal-2__8.2.2637-40.el9.aarch64",
    sha256 = "67884caeaffe9ed6d3ddc42cdac8736374ce6ea48b2187a3bff0f8d24ef088ba",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/vim-minimal-8.2.2637-40.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/67884caeaffe9ed6d3ddc42cdac8736374ce6ea48b2187a3bff0f8d24ef088ba",
    ],
)

rpm(
    name = "vim-minimal-2__8.2.2637-40.el9.s390x",
    sha256 = "d280cfb0c8b9f8f3aac10f21e2144faa21d66370a6a34c3c1eeeb0379a0b8786",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/vim-minimal-8.2.2637-40.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/d280cfb0c8b9f8f3aac10f21e2144faa21d66370a6a34c3c1eeeb0379a0b8786",
    ],
)

rpm(
    name = "vim-minimal-2__8.2.2637-40.el9.x86_64",
    sha256 = "7c1fb896f1bcb816f492b1dfaa2a6ff10ba54f7491ef8be064d8fa984d7a5f4c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/vim-minimal-8.2.2637-40.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/7c1fb896f1bcb816f492b1dfaa2a6ff10ba54f7491ef8be064d8fa984d7a5f4c",
    ],
)

rpm(
    name = "vim-minimal-2__9.1.083-26.el10.aarch64",
    sha256 = "6478996294992aebde0ef4235543c41d293da7f74781a8efde52bd37a6e5830a",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/vim-minimal-9.1.083-26.el10.aarch64.rpm",
    ],
)

rpm(
    name = "vim-minimal-2__9.1.083-26.el10.s390x",
    sha256 = "02ea70f970698f21e0219e562e767b6571a68c56b1b7382a9e45fe14d366b572",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/vim-minimal-9.1.083-26.el10.s390x.rpm",
    ],
)

rpm(
    name = "vim-minimal-2__9.1.083-26.el10.x86_64",
    sha256 = "a8e78c1bb7c16ec964de95e9d47b18099bdcd89127105e140f275c0323ca2765",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/vim-minimal-9.1.083-26.el10.x86_64.rpm",
    ],
)

rpm(
    name = "xz-libs-0__5.2.5-7.el9.aarch64",
    sha256 = "49c5e788208a6e2e458d6bdaf8bde5b834eb32693810b90b4354c4c47695b453",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/xz-libs-5.2.5-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/49c5e788208a6e2e458d6bdaf8bde5b834eb32693810b90b4354c4c47695b453",
    ],
)

rpm(
    name = "xz-libs-0__5.2.5-7.el9.x86_64",
    sha256 = "770819da28cce56e2e2b141b0eee1694d7f3dcf78a5700e1469436461399f001",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/xz-libs-5.2.5-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/770819da28cce56e2e2b141b0eee1694d7f3dcf78a5700e1469436461399f001",
    ],
)

rpm(
    name = "xz-libs-0__5.2.5-8.el9.aarch64",
    sha256 = "99784163a31515239be42e68608478b8337fd168cdb12bcba31de9dd78e35a25",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/xz-libs-5.2.5-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/99784163a31515239be42e68608478b8337fd168cdb12bcba31de9dd78e35a25",
    ],
)

rpm(
    name = "xz-libs-0__5.2.5-8.el9.s390x",
    sha256 = "f5df58b242361ae5aaf97d1149c4331cc762394cadb5ebd054db089a6e10ae24",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/xz-libs-5.2.5-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/f5df58b242361ae5aaf97d1149c4331cc762394cadb5ebd054db089a6e10ae24",
    ],
)

rpm(
    name = "xz-libs-0__5.2.5-8.el9.x86_64",
    sha256 = "ff3c88297d75c51a5f8e9d2d69f8ad1eaf8347e20920b4335a3e0fc53269ad28",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/xz-libs-5.2.5-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/ff3c88297d75c51a5f8e9d2d69f8ad1eaf8347e20920b4335a3e0fc53269ad28",
    ],
)

rpm(
    name = "xz-libs-1__5.6.2-5.el10.aarch64",
    sha256 = "13ec8e033de69355bfc879e9c9f36fde8a3256f4bc9f6c3dc2fb57742a66dc15",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/xz-libs-5.6.2-5.el10.aarch64.rpm",
    ],
)

rpm(
    name = "xz-libs-1__5.6.2-5.el10.s390x",
    sha256 = "3779d1660324ddc72a319e23061cfc935550bffc110f8219f227d8899eb517a8",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/xz-libs-5.6.2-5.el10.s390x.rpm",
    ],
)

rpm(
    name = "xz-libs-1__5.6.2-5.el10.x86_64",
    sha256 = "4386fe02b06559020fe8419be39b12109e8995c52b83ae68a211cacd5f8c0abb",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/xz-libs-5.6.2-5.el10.x86_64.rpm",
    ],
)

rpm(
    name = "zlib-0__1.2.11-32.el9.aarch64",
    sha256 = "1b99ee6c18e92f2a727c39668941273c67f25eef18f7e9fe4febd484e9a80dbd",
    urls = ["https://storage.googleapis.com/builddeps/1b99ee6c18e92f2a727c39668941273c67f25eef18f7e9fe4febd484e9a80dbd"],
)

rpm(
    name = "zlib-0__1.2.11-32.el9.x86_64",
    sha256 = "59b0101c691ea180b992d338b372852c1d1607931c472c6ee22056e2fb099505",
    urls = ["https://storage.googleapis.com/builddeps/59b0101c691ea180b992d338b372852c1d1607931c472c6ee22056e2fb099505"],
)

rpm(
    name = "zlib-0__1.2.11-41.el9.aarch64",
    sha256 = "c50e107cdd35460294852d99c954296e0e833d37852a1be1e2aaea2f1b48f9d2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/zlib-1.2.11-41.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c50e107cdd35460294852d99c954296e0e833d37852a1be1e2aaea2f1b48f9d2",
    ],
)

rpm(
    name = "zlib-0__1.2.11-41.el9.s390x",
    sha256 = "bbe95dadf7383694d5b13ea8ae89b76697ed7009b4be889220d4a7d23db28759",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/zlib-1.2.11-41.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/bbe95dadf7383694d5b13ea8ae89b76697ed7009b4be889220d4a7d23db28759",
    ],
)

rpm(
    name = "zlib-0__1.2.11-41.el9.x86_64",
    sha256 = "370951ea635bc16313f21ac2823ec815147ed1124b74865a34c54e94e4db9602",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/zlib-1.2.11-41.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/370951ea635bc16313f21ac2823ec815147ed1124b74865a34c54e94e4db9602",
    ],
)

rpm(
    name = "zlib-ng-compat-0__2.2.3-3.el10.aarch64",
    sha256 = "a7870bf73b68086ae1fdd3e2fb6191bf79dff1ab5ae16b907efbb0befe590dca",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/aarch64/os/Packages/zlib-ng-compat-2.2.3-3.el10.aarch64.rpm",
    ],
)

rpm(
    name = "zlib-ng-compat-0__2.2.3-3.el10.s390x",
    sha256 = "89c8decb9febd474ba2f3fbb38c37577dd7098b349a9e766267723fb94f25962",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/s390x/os/Packages/zlib-ng-compat-2.2.3-3.el10.s390x.rpm",
    ],
)

rpm(
    name = "zlib-ng-compat-0__2.2.3-3.el10.x86_64",
    sha256 = "8fe3c2d5203810828fa3e4a5d84ae53172ffd27f4f0eec9d192b42b187795c09",
    urls = [
        "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/Packages/zlib-ng-compat-2.2.3-3.el10.x86_64.rpm",
    ],
)
