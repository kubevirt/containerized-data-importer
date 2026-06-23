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
    sha256 = "130739704540caa14e77c54810b9f01d6d9ae897d53eedceb40fd6b75efc3c23",
    urls = [
        "https://github.com/bazel-contrib/rules_go/releases/download/v0.54.1/rules_go-v0.54.1.zip",
        "https://storage.googleapis.com/builddeps/130739704540caa14e77c54810b9f01d6d9ae897d53eedceb40fd6b75efc3c23",
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
    sha256 = "b760f7fe75173886007f7c2e616a21241208f3d90e8657dc65d36a771e916b6a",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.39.1/bazel-gazelle-v0.39.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.39.1/bazel-gazelle-v0.39.1.tar.gz",
        "https://storage.googleapis.com/builddeps/b760f7fe75173886007f7c2e616a21241208f3d90e8657dc65d36a771e916b6a",
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
    sha256 = "fb24d80ad9edad0f7bd3000e8cffcfbba89cc07e495c47a7d3b1f803bd527a40",
    urls = [
        "https://github.com/rmohr/bazeldnf/releases/download/v0.5.9/bazeldnf-v0.5.9.tar.gz",
        "https://storage.googleapis.com/builddeps/fb24d80ad9edad0f7bd3000e8cffcfbba89cc07e495c47a7d3b1f803bd527a40",
    ],
)

load("@bazeldnf//:deps.bzl", "bazeldnf_dependencies", "rpm")

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
    name = "aardvark-dns-2__1.17.0-1.el9.aarch64",
    sha256 = "75dc503a10c47df27a9bda47c2b32153210b1c36bd549007cbf07471895cc3bc",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/aardvark-dns-1.17.0-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/75dc503a10c47df27a9bda47c2b32153210b1c36bd549007cbf07471895cc3bc",
    ],
)

rpm(
    name = "aardvark-dns-2__1.17.0-1.el9.s390x",
    sha256 = "1906e1fcd1530c689bbed3b5b221de0bdec33ff5d549f6a08861978c90985ff4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/aardvark-dns-1.17.0-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/1906e1fcd1530c689bbed3b5b221de0bdec33ff5d549f6a08861978c90985ff4",
    ],
)

rpm(
    name = "aardvark-dns-2__1.17.0-1.el9.x86_64",
    sha256 = "4a1b408cdc00a1647aaa0a51bfaf0b14f40eec702859c59b43ece5191023fa5f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/aardvark-dns-1.17.0-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/4a1b408cdc00a1647aaa0a51bfaf0b14f40eec702859c59b43ece5191023fa5f",
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
    name = "acl-0__2.3.1-4.el9.aarch64",
    sha256 = "a0a9b302d252d32c0da8100a0ad762852c22eeac4ccad0aaf72ad68a2bbd7a93",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/acl-2.3.1-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/a0a9b302d252d32c0da8100a0ad762852c22eeac4ccad0aaf72ad68a2bbd7a93",
    ],
)

rpm(
    name = "acl-0__2.3.1-4.el9.s390x",
    sha256 = "5d12a3e157b07244a7c0546905af864148730e982ac7ceaa4b0bf287dd7ae669",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/acl-2.3.1-4.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/5d12a3e157b07244a7c0546905af864148730e982ac7ceaa4b0bf287dd7ae669",
    ],
)

rpm(
    name = "acl-0__2.3.1-4.el9.x86_64",
    sha256 = "dd11bab2ea0abdfa310362eace871422a003340bf223135626500f8f5a985f6b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/acl-2.3.1-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/dd11bab2ea0abdfa310362eace871422a003340bf223135626500f8f5a985f6b",
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
    name = "audit-libs-0__3.1.5-8.el9.aarch64",
    sha256 = "83af8b9a4dd0539f10ffda2ee09fe4a93eaf45fb12a3fc4aaea5899025f12cac",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/audit-libs-3.1.5-8.el9.aarch64.rpm",
    ],
)

rpm(
    name = "audit-libs-0__3.1.5-8.el9.s390x",
    sha256 = "267f9e2528d2ca70c83abd80002aab8284ea93da3f2d87be0d13a0ec7efb13c9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/audit-libs-3.1.5-8.el9.s390x.rpm",
    ],
)

rpm(
    name = "audit-libs-0__3.1.5-8.el9.x86_64",
    sha256 = "f970ce7fc0589c0a7b37784c6fc602a35a771db811f8061b8b8af2f4e9b46349",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/audit-libs-3.1.5-8.el9.x86_64.rpm",
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
    name = "buildah-2__1.43.1-1.el9.aarch64",
    sha256 = "17bf8c20e5565139911accd2bc7f116942e45fae990d4d998f582cf962f5200b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/buildah-1.43.1-1.el9.aarch64.rpm",
    ],
)

rpm(
    name = "buildah-2__1.43.1-1.el9.s390x",
    sha256 = "051ce89536d7f02069a5babf8ef14bffa5f107c3a231809454b06c80919c5516",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/buildah-1.43.1-1.el9.s390x.rpm",
    ],
)

rpm(
    name = "buildah-2__1.43.1-1.el9.x86_64",
    sha256 = "b360ab508a1d4885c88b342c49c20330f64a11b2d66de2c066e7456183a0d8ec",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/buildah-1.43.1-1.el9.x86_64.rpm",
    ],
)

rpm(
    name = "bzip2-libs-0__1.0.8-11.el9.aarch64",
    sha256 = "fafc0f2b7632774d4c07264c73eebbe52f815b4c81056bd44b944e5255cb20bb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/bzip2-libs-1.0.8-11.el9.aarch64.rpm",
    ],
)

rpm(
    name = "bzip2-libs-0__1.0.8-11.el9.s390x",
    sha256 = "e9746e7bd442b4104b726e239cf3b7b87400824c7094de6d11f356da4c27593f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/bzip2-libs-1.0.8-11.el9.s390x.rpm",
    ],
)

rpm(
    name = "bzip2-libs-0__1.0.8-11.el9.x86_64",
    sha256 = "e1f4ca1a16276a6ede5f67cab8d8d2920b98531419af7498f5fded85835e0fca",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/bzip2-libs-1.0.8-11.el9.x86_64.rpm",
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
    name = "ca-certificates-0__2025.2.80_v9.0.305-91.el9.aarch64",
    sha256 = "489fdf258344892412ff2f10d0c1c849c45d5a15c4628abda33f325a42dd1bb0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/ca-certificates-2025.2.80_v9.0.305-91.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/489fdf258344892412ff2f10d0c1c849c45d5a15c4628abda33f325a42dd1bb0",
    ],
)

rpm(
    name = "ca-certificates-0__2025.2.80_v9.0.305-91.el9.s390x",
    sha256 = "489fdf258344892412ff2f10d0c1c849c45d5a15c4628abda33f325a42dd1bb0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/ca-certificates-2025.2.80_v9.0.305-91.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/489fdf258344892412ff2f10d0c1c849c45d5a15c4628abda33f325a42dd1bb0",
    ],
)

rpm(
    name = "ca-certificates-0__2025.2.80_v9.0.305-91.el9.x86_64",
    sha256 = "489fdf258344892412ff2f10d0c1c849c45d5a15c4628abda33f325a42dd1bb0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ca-certificates-2025.2.80_v9.0.305-91.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/489fdf258344892412ff2f10d0c1c849c45d5a15c4628abda33f325a42dd1bb0",
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
    ],
)

rpm(
    name = "centos-gpg-keys-0__9.0-38.el9.s390x",
    sha256 = "b6dcd5a16160ab017bf5e871975aef477d34ce61b660eeb3f4aa6973dcc6f916",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/centos-gpg-keys-9.0-38.el9.noarch.rpm",
    ],
)

rpm(
    name = "centos-gpg-keys-0__9.0-38.el9.x86_64",
    sha256 = "b6dcd5a16160ab017bf5e871975aef477d34ce61b660eeb3f4aa6973dcc6f916",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-gpg-keys-9.0-38.el9.noarch.rpm",
    ],
)

rpm(
    name = "centos-logos-httpd-0__90.9-1.el9.aarch64",
    sha256 = "0a6e9d58e4941b43b115c90aa468fe3b335a938a805c18676896dc93587b741d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/centos-logos-httpd-90.9-1.el9.noarch.rpm",
    ],
)

rpm(
    name = "centos-logos-httpd-0__90.9-1.el9.s390x",
    sha256 = "0a6e9d58e4941b43b115c90aa468fe3b335a938a805c18676896dc93587b741d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/centos-logos-httpd-90.9-1.el9.noarch.rpm",
    ],
)

rpm(
    name = "centos-logos-httpd-0__90.9-1.el9.x86_64",
    sha256 = "0a6e9d58e4941b43b115c90aa468fe3b335a938a805c18676896dc93587b741d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/centos-logos-httpd-90.9-1.el9.noarch.rpm",
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
    ],
)

rpm(
    name = "centos-stream-release-0__9.0-38.el9.s390x",
    sha256 = "04a24747b2884f59d8ac5583f162dbc5ed043f5ccf602ef6274349f1d7ca9a8e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/centos-stream-release-9.0-38.el9.noarch.rpm",
    ],
)

rpm(
    name = "centos-stream-release-0__9.0-38.el9.x86_64",
    sha256 = "04a24747b2884f59d8ac5583f162dbc5ed043f5ccf602ef6274349f1d7ca9a8e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-stream-release-9.0-38.el9.noarch.rpm",
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
    ],
)

rpm(
    name = "centos-stream-repos-0__9.0-38.el9.s390x",
    sha256 = "ae98322c35b3eb7b013c8989a8b318993e3bec71018e213f729a156c2bbd508d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/centos-stream-repos-9.0-38.el9.noarch.rpm",
    ],
)

rpm(
    name = "centos-stream-repos-0__9.0-38.el9.x86_64",
    sha256 = "ae98322c35b3eb7b013c8989a8b318993e3bec71018e213f729a156c2bbd508d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-stream-repos-9.0-38.el9.noarch.rpm",
    ],
)

rpm(
    name = "containers-common-5__5.8-1.el9.aarch64",
    sha256 = "4844ee600f88ffd111480480c8fd8493b7cb6cda893255d54eaf7aad77f9b643",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/containers-common-5.8-1.el9.aarch64.rpm",
    ],
)

rpm(
    name = "containers-common-5__5.8-1.el9.s390x",
    sha256 = "e3963fa5ed00dfe8c35a41cd270d952863c2908471877629f694b41099cf5221",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/containers-common-5.8-1.el9.s390x.rpm",
    ],
)

rpm(
    name = "containers-common-5__5.8-1.el9.x86_64",
    sha256 = "1eb9aa848c6a0524adb83cd262310df35aa18a90d1adbfff9e89bd3e3c276447",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/containers-common-5.8-1.el9.x86_64.rpm",
    ],
)

rpm(
    name = "containers-common-extra-5__5.8-1.el9.aarch64",
    sha256 = "f46da5419ca301a2bc57b21ffd6a59aa4029533f7d9386d7209f2caae29b7eb0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/containers-common-extra-5.8-1.el9.aarch64.rpm",
    ],
)

rpm(
    name = "containers-common-extra-5__5.8-1.el9.s390x",
    sha256 = "b83c52d71e64925e336cf1c651ddc2bb4d46fb92f6ccad4933e53285699668c6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/containers-common-extra-5.8-1.el9.s390x.rpm",
    ],
)

rpm(
    name = "containers-common-extra-5__5.8-1.el9.x86_64",
    sha256 = "f5b5431d4e35e63f89f2b7d6e6265e49e94262cee582e89a2a5b4dfab910392b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/containers-common-extra-5.8-1.el9.x86_64.rpm",
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
    name = "coreutils-single-0__8.32-42.el9.aarch64",
    sha256 = "4a5d45a991d06cd653bee77a4b674ac0d147acd074b54ffd17527862e1b9568a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/coreutils-single-8.32-42.el9.aarch64.rpm",
    ],
)

rpm(
    name = "coreutils-single-0__8.32-42.el9.s390x",
    sha256 = "73a9a2c05307751eb50579e74e6fd10b9e9df7658f6bbea37d6acc98cc997af5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/coreutils-single-8.32-42.el9.s390x.rpm",
    ],
)

rpm(
    name = "coreutils-single-0__8.32-42.el9.x86_64",
    sha256 = "e6975fdb693afea11fe6e67aefa20679257bfa7984e2cafebd9a977807dbf613",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/coreutils-single-8.32-42.el9.x86_64.rpm",
    ],
)

rpm(
    name = "cracklib-0__2.9.6-28.el9.aarch64",
    sha256 = "78dbd83e4de7c011dedc8071af056989dece25dae7605eb60703b219ebbeadc1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/cracklib-2.9.6-28.el9.aarch64.rpm",
    ],
)

rpm(
    name = "cracklib-0__2.9.6-28.el9.s390x",
    sha256 = "14006fd9132581ca7ab86b87eb4751efd25279bc60df48aced985002e401112d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/cracklib-2.9.6-28.el9.s390x.rpm",
    ],
)

rpm(
    name = "cracklib-0__2.9.6-28.el9.x86_64",
    sha256 = "aa659fc5fc1f40d9301850411e1e4cfb9351175e1879a1d404292cbd909982f0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/cracklib-2.9.6-28.el9.x86_64.rpm",
    ],
)

rpm(
    name = "cracklib-dicts-0__2.9.6-28.el9.aarch64",
    sha256 = "3b449db83d1a649b93eff386e098ab01f24028b106827d9fef899abc99818b15",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/cracklib-dicts-2.9.6-28.el9.aarch64.rpm",
    ],
)

rpm(
    name = "cracklib-dicts-0__2.9.6-28.el9.s390x",
    sha256 = "a0ac88ff592620ae37ea0826d59874f0f5a08828c02fcd514473302d15cf6c03",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/cracklib-dicts-2.9.6-28.el9.s390x.rpm",
    ],
)

rpm(
    name = "cracklib-dicts-0__2.9.6-28.el9.x86_64",
    sha256 = "b0e372c09e6eb01d2de1316b7e59c79178c0eaee6d713004d7fe5fbc7e718603",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/cracklib-dicts-2.9.6-28.el9.x86_64.rpm",
    ],
)

rpm(
    name = "crun-0__1.27-2.el9.aarch64",
    sha256 = "111427d4c9e0ef56c2a945ff152a98dd3b35b35536edf3366fe99eab094f00b7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/crun-1.27-2.el9.aarch64.rpm",
    ],
)

rpm(
    name = "crun-0__1.27-2.el9.s390x",
    sha256 = "220cf80e3f2ec7dba4bba5f9d417b7ff69a4c483ef23707c32641e9f9495e684",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/crun-1.27-2.el9.s390x.rpm",
    ],
)

rpm(
    name = "crun-0__1.27-2.el9.x86_64",
    sha256 = "44e3cb21e7bd34c2c5ab68b12453eadccb14558309e50d3348d7313129f1a4d4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/crun-1.27-2.el9.x86_64.rpm",
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
    name = "crypto-policies-0__20260224-1.gitea0f072.el9.aarch64",
    sha256 = "c8c1a39f2a60386222a51fb9f6bd2a9fd461e1ac1ecc8067c81e45b001cb800c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/crypto-policies-20260224-1.gitea0f072.el9.noarch.rpm",
    ],
)

rpm(
    name = "crypto-policies-0__20260224-1.gitea0f072.el9.s390x",
    sha256 = "c8c1a39f2a60386222a51fb9f6bd2a9fd461e1ac1ecc8067c81e45b001cb800c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/crypto-policies-20260224-1.gitea0f072.el9.noarch.rpm",
    ],
)

rpm(
    name = "crypto-policies-0__20260224-1.gitea0f072.el9.x86_64",
    sha256 = "c8c1a39f2a60386222a51fb9f6bd2a9fd461e1ac1ecc8067c81e45b001cb800c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/crypto-policies-20260224-1.gitea0f072.el9.noarch.rpm",
    ],
)

rpm(
    name = "crypto-policies-scripts-0__20260224-1.gitea0f072.el9.aarch64",
    sha256 = "4111888a478620c87a1170e8688ecaa5ccbe9dcfabebfc86ab3a64d69eb579dd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/crypto-policies-scripts-20260224-1.gitea0f072.el9.noarch.rpm",
    ],
)

rpm(
    name = "crypto-policies-scripts-0__20260224-1.gitea0f072.el9.s390x",
    sha256 = "4111888a478620c87a1170e8688ecaa5ccbe9dcfabebfc86ab3a64d69eb579dd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/crypto-policies-scripts-20260224-1.gitea0f072.el9.noarch.rpm",
    ],
)

rpm(
    name = "crypto-policies-scripts-0__20260224-1.gitea0f072.el9.x86_64",
    sha256 = "4111888a478620c87a1170e8688ecaa5ccbe9dcfabebfc86ab3a64d69eb579dd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/crypto-policies-scripts-20260224-1.gitea0f072.el9.noarch.rpm",
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
    ],
)

rpm(
    name = "curl-0__7.76.1-43.el9.s390x",
    sha256 = "adbad3914af90435ea1843e68d26f217e32a0ec9cb2d754afd2ad89101aced8a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/curl-7.76.1-43.el9.s390x.rpm",
    ],
)

rpm(
    name = "curl-0__7.76.1-43.el9.x86_64",
    sha256 = "a17d00a4bc40bf90f060c607a6ec4d2459c640a84a706b01da3ce0855be9ef4a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/curl-7.76.1-43.el9.x86_64.rpm",
    ],
)

rpm(
    name = "curl-minimal-0__7.76.1-43.el9.aarch64",
    sha256 = "e52f5971cbd6079130b1c6ef4ed35269b7b280a4e63ed1a3e7188ff8f15d610d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/curl-minimal-7.76.1-43.el9.aarch64.rpm",
    ],
)

rpm(
    name = "curl-minimal-0__7.76.1-43.el9.x86_64",
    sha256 = "0ddf97bb566dd3c6c877b2a2fb895b252d13982a12f1cd407c9ca21a53ad0777",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/curl-minimal-7.76.1-43.el9.x86_64.rpm",
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
    name = "dbus-1__1.12.20-9.el9.aarch64",
    sha256 = "f2f2f80cf9c11b7f4e1c27ba65a416b1dad9a48c2991ed1cb77c038a62319754",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/dbus-1.12.20-9.el9.aarch64.rpm",
    ],
)

rpm(
    name = "dbus-1__1.12.20-9.el9.s390x",
    sha256 = "62f819b14f1fec3a9eeb91b6367ba8b1ff464875414477157d61ca04da3aeede",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/dbus-1.12.20-9.el9.s390x.rpm",
    ],
)

rpm(
    name = "dbus-1__1.12.20-9.el9.x86_64",
    sha256 = "9e0a4fc4da86a68b0366601580a9b2af73901440b85219370f60d773c344cc7c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-1.12.20-9.el9.x86_64.rpm",
    ],
)

rpm(
    name = "dbus-broker-0__28-7.el9.aarch64",
    sha256 = "28a7abe52040dcda6e5d941206ef6e5c47478fcc06a9f05c2ab7dacc2afa9f42",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/dbus-broker-28-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/28a7abe52040dcda6e5d941206ef6e5c47478fcc06a9f05c2ab7dacc2afa9f42",
    ],
)

rpm(
    name = "dbus-broker-0__28-7.el9.s390x",
    sha256 = "d38a5ae851f9006000c3cd7a37310f901a02864e0272d7284c4f2db1efcd61ff",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/dbus-broker-28-7.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/d38a5ae851f9006000c3cd7a37310f901a02864e0272d7284c4f2db1efcd61ff",
    ],
)

rpm(
    name = "dbus-broker-0__28-7.el9.x86_64",
    sha256 = "dd65bddd728ed08dcdba5d06b5a5af9f958e5718e8cab938783241bd8f4d1131",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-broker-28-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/dd65bddd728ed08dcdba5d06b5a5af9f958e5718e8cab938783241bd8f4d1131",
    ],
)

rpm(
    name = "dbus-common-1__1.12.20-9.el9.aarch64",
    sha256 = "c9e2580b234cf5591cdecd5472ae14b7886392dcf4e91d63751f18b320e7694b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/dbus-common-1.12.20-9.el9.noarch.rpm",
    ],
)

rpm(
    name = "dbus-common-1__1.12.20-9.el9.s390x",
    sha256 = "c9e2580b234cf5591cdecd5472ae14b7886392dcf4e91d63751f18b320e7694b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/dbus-common-1.12.20-9.el9.noarch.rpm",
    ],
)

rpm(
    name = "dbus-common-1__1.12.20-9.el9.x86_64",
    sha256 = "c9e2580b234cf5591cdecd5472ae14b7886392dcf4e91d63751f18b320e7694b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-common-1.12.20-9.el9.noarch.rpm",
    ],
)

rpm(
    name = "expat-0__2.5.0-6.el9.aarch64",
    sha256 = "01f1ff2194173775ebbc1d00934152585a259c9a852e987e672d1810384e4786",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/expat-2.5.0-6.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/01f1ff2194173775ebbc1d00934152585a259c9a852e987e672d1810384e4786",
    ],
)

rpm(
    name = "expat-0__2.5.0-6.el9.s390x",
    sha256 = "6e85c05c7eacb3d964af391a67898919239b973d8094c442b917ea450391d25d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/expat-2.5.0-6.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/6e85c05c7eacb3d964af391a67898919239b973d8094c442b917ea450391d25d",
    ],
)

rpm(
    name = "expat-0__2.5.0-6.el9.x86_64",
    sha256 = "39cffc5a3a75ccd06d4214f99e3d3a89dd79bee3532175ae38d37c14aad529fc",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/expat-2.5.0-6.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/39cffc5a3a75ccd06d4214f99e3d3a89dd79bee3532175ae38d37c14aad529fc",
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
    name = "fips-provider-next-0__1.5.0-4.el9.aarch64",
    sha256 = "695312999217d74b5973c4872042e1fa56777941a9b9e983f013f20397dfb5ce",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/fips-provider-next-1.5.0-4.el9.aarch64.rpm",
    ],
)

rpm(
    name = "fips-provider-next-0__1.5.0-4.el9.s390x",
    sha256 = "d247fb263436edd1f2ea8ab9414061d24d7980ade1f499d00d8c697c384b7c5d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/fips-provider-next-1.5.0-4.el9.s390x.rpm",
    ],
)

rpm(
    name = "fips-provider-next-0__1.5.0-4.el9.x86_64",
    sha256 = "9363b6edede8b9a928de5516852c01300b8c1a87c964c8fca7cde896267d15b2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/fips-provider-next-1.5.0-4.el9.x86_64.rpm",
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
    name = "gawk-0__5.1.0-6.el9.s390x",
    sha256 = "acad833571094a674d4073b4e747e15d373e3a8b06a7e7e8aecfec6fd4860c0e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gawk-5.1.0-6.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/acad833571094a674d4073b4e747e15d373e3a8b06a7e7e8aecfec6fd4860c0e",
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
    name = "glib2-0__2.68.4-20.el9.aarch64",
    sha256 = "3911bba0d89cc320479fefd6ede6cec6c3c4537c198419ee4784bf6ae3bf60d6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glib2-2.68.4-20.el9.aarch64.rpm",
    ],
)

rpm(
    name = "glib2-0__2.68.4-20.el9.s390x",
    sha256 = "d5f084b1534e680bf72f1a2b7dafccb0775e77c1c6f76de2d133022f5a6feacb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glib2-2.68.4-20.el9.s390x.rpm",
    ],
)

rpm(
    name = "glib2-0__2.68.4-20.el9.x86_64",
    sha256 = "ce540bb580908bb7f025e06c4dab863658f15b1e9f89c232eea0a2d511c2b0ac",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glib2-2.68.4-20.el9.x86_64.rpm",
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
    name = "glibc-0__2.34-270.el9.aarch64",
    sha256 = "270986bf5b06c76c23c28a3230daf90816f43801fdc487350e964ce7db52da86",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-2.34-270.el9.aarch64.rpm",
    ],
)

rpm(
    name = "glibc-0__2.34-270.el9.s390x",
    sha256 = "88ab2c3c94db119ca2d1882d5f4a34f5ae8e294aa8c273b86ba4f22bb137313e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glibc-2.34-270.el9.s390x.rpm",
    ],
)

rpm(
    name = "glibc-0__2.34-270.el9.x86_64",
    sha256 = "f3d6d19a775cd3b75ade47e3428d0d853ec6ee68350087c5f6c91d94e0cd0208",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-2.34-270.el9.x86_64.rpm",
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
    name = "glibc-common-0__2.34-270.el9.aarch64",
    sha256 = "ee277892d39af3afa723e849df98979f7b8839b0e376afc6c5e654c7868012a8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-common-2.34-270.el9.aarch64.rpm",
    ],
)

rpm(
    name = "glibc-common-0__2.34-270.el9.s390x",
    sha256 = "b1a16e9112b8ed375cd277b4b3799fe2380b4355d23f0b4d5c3f248636c47482",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glibc-common-2.34-270.el9.s390x.rpm",
    ],
)

rpm(
    name = "glibc-common-0__2.34-270.el9.x86_64",
    sha256 = "69b8a512ebf1f9e6931c5e8aa27b0cfc56ce0709088e4438086ae4916fa5259f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-common-2.34-270.el9.x86_64.rpm",
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
    name = "glibc-minimal-langpack-0__2.34-270.el9.aarch64",
    sha256 = "14b98c4c261a202c30025f1644e30bc675bcadbd49576e740ad42b46dbd1c831",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-minimal-langpack-2.34-270.el9.aarch64.rpm",
    ],
)

rpm(
    name = "glibc-minimal-langpack-0__2.34-270.el9.s390x",
    sha256 = "484f1c371c30b5fa5357f133d4b906239877147d1b87e480f58e789f451c3cc8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glibc-minimal-langpack-2.34-270.el9.s390x.rpm",
    ],
)

rpm(
    name = "glibc-minimal-langpack-0__2.34-270.el9.x86_64",
    sha256 = "3f602fb59f692fc6d883f393c893e455127cefc9a44a3862213a966872390e8b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-minimal-langpack-2.34-270.el9.x86_64.rpm",
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
    name = "gnupg2-0__2.3.3-5.el9.aarch64",
    sha256 = "5fd008231e14128555e5eb997ae57e3f82fc8ac28b0bfb2b9e961f8a8bdf9937",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gnupg2-2.3.3-5.el9.aarch64.rpm",
    ],
)

rpm(
    name = "gnupg2-0__2.3.3-5.el9.s390x",
    sha256 = "9cbb342b46df96e85e55919bee459b2fd5023642494eeb2466344b765c1802d3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gnupg2-2.3.3-5.el9.s390x.rpm",
    ],
)

rpm(
    name = "gnupg2-0__2.3.3-5.el9.x86_64",
    sha256 = "5628444d9a62a7b6b46951c5187ccf43cb4d9254a45ae225808c6ef7d28c027f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gnupg2-2.3.3-5.el9.x86_64.rpm",
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
    name = "gnutls-0__3.8.10-5.el9.aarch64",
    sha256 = "611478ba2161392d417219f6e83a559287d01c493376e99b2d7dfb01f3d23a3f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gnutls-3.8.10-5.el9.aarch64.rpm",
    ],
)

rpm(
    name = "gnutls-0__3.8.10-5.el9.s390x",
    sha256 = "1596f32434021755a14df443dd6d3849ed7656c5c9946a172079b934cb3cbc7d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gnutls-3.8.10-5.el9.s390x.rpm",
    ],
)

rpm(
    name = "gnutls-0__3.8.10-5.el9.x86_64",
    sha256 = "ab7a9a7e5ce2a976e07632ebd0a1a5267f55026437e2fa4a37e36da0f2589ee0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gnutls-3.8.10-5.el9.x86_64.rpm",
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
    name = "gzip-0__1.12-1.el9.aarch64",
    sha256 = "5a39a441dad01ccc8af601f1cca5bed46ac231fbdbe39ea3202bd54cf9390d81",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gzip-1.12-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/5a39a441dad01ccc8af601f1cca5bed46ac231fbdbe39ea3202bd54cf9390d81",
    ],
)

rpm(
    name = "gzip-0__1.12-1.el9.s390x",
    sha256 = "72b8b818027d9d716be069743c03431f057ce5af62b38273c249990890cbc504",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gzip-1.12-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/72b8b818027d9d716be069743c03431f057ce5af62b38273c249990890cbc504",
    ],
)

rpm(
    name = "gzip-0__1.12-1.el9.x86_64",
    sha256 = "e8d7783c666a58ab870246b04eb0ea22965123fe284697d2c0e1e6dbf10ea861",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gzip-1.12-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e8d7783c666a58ab870246b04eb0ea22965123fe284697d2c0e1e6dbf10ea861",
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
    ],
)

rpm(
    name = "krb5-libs-0__1.21.1-10.el9.s390x",
    sha256 = "7f79794f0adc0b7f0ede5dd6d8536068c7f8de948d947e42ce1cdafeb96fe8e3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/krb5-libs-1.21.1-10.el9.s390x.rpm",
    ],
)

rpm(
    name = "krb5-libs-0__1.21.1-10.el9.x86_64",
    sha256 = "55f585ca5ceb611bcd44ce845179769fa42a2316fe23b83b1e13947fd54b7e0d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/krb5-libs-1.21.1-10.el9.x86_64.rpm",
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
    name = "libacl-0__2.3.1-4.el9.aarch64",
    sha256 = "90e4392e312cd793eeba4cd68bd12836a882ac37356c784806d67a0cd1d48c25",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libacl-2.3.1-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/90e4392e312cd793eeba4cd68bd12836a882ac37356c784806d67a0cd1d48c25",
    ],
)

rpm(
    name = "libacl-0__2.3.1-4.el9.s390x",
    sha256 = "bfdd2316c1742032df9b15d1a91ff2e3674faeae1e27e4a851165e5c6bb666f5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libacl-2.3.1-4.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/bfdd2316c1742032df9b15d1a91ff2e3674faeae1e27e4a851165e5c6bb666f5",
    ],
)

rpm(
    name = "libacl-0__2.3.1-4.el9.x86_64",
    sha256 = "60a3affaa1c387fd6f72dd65aa7ad619a1830947823abb4b29e7b9fcb4c9d27c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libacl-2.3.1-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/60a3affaa1c387fd6f72dd65aa7ad619a1830947823abb4b29e7b9fcb4c9d27c",
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
    name = "libaio-0__0.3.111-13.el9.s390x",
    sha256 = "b4adecd95273b4ae7590b84ecbed5a7b4a1795066bab430d15f04eb82bb9dc1c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libaio-0.3.111-13.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/b4adecd95273b4ae7590b84ecbed5a7b4a1795066bab430d15f04eb82bb9dc1c",
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
    name = "libatomic-0__11.5.0-14.el9.aarch64",
    sha256 = "9111ad5dcd16ac04ee06dbedbc730bdf438d58f1f16af2de5cd3cdb3e346efbe",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libatomic-11.5.0-14.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/9111ad5dcd16ac04ee06dbedbc730bdf438d58f1f16af2de5cd3cdb3e346efbe",
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
    name = "libattr-0__2.5.1-3.el9.s390x",
    sha256 = "c37335be62aaca9f21f2b0b0312d3800e245f6e70fa8b57d03ab89cce863f2be",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libattr-2.5.1-3.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/c37335be62aaca9f21f2b0b0312d3800e245f6e70fa8b57d03ab89cce863f2be",
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
    name = "libblkid-0__2.37.4-25.el9.aarch64",
    sha256 = "40de20b6cbd0d5bf61e1576d47c154b349779be6790d8ad05d54cad94a8f9a3b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libblkid-2.37.4-25.el9.aarch64.rpm",
    ],
)

rpm(
    name = "libblkid-0__2.37.4-25.el9.s390x",
    sha256 = "62d6027ed230599196800f12bbd058670aa4a8759c829c934e0b829c3996c288",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libblkid-2.37.4-25.el9.s390x.rpm",
    ],
)

rpm(
    name = "libblkid-0__2.37.4-25.el9.x86_64",
    sha256 = "2309af12b80fec77070d354fdae370ffa3e57209137b46098286895be5a484f5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libblkid-2.37.4-25.el9.x86_64.rpm",
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
    ],
)

rpm(
    name = "libcurl-minimal-0__7.76.1-43.el9.s390x",
    sha256 = "c2807b0788883480e4c1ecae130f66e1463672461d1ca33bee6160be5e7fe2b8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libcurl-minimal-7.76.1-43.el9.s390x.rpm",
    ],
)

rpm(
    name = "libcurl-minimal-0__7.76.1-43.el9.x86_64",
    sha256 = "ca12a88c313df73ce0e8f5a652b57daded8733183c0d44f85f3dca780b356c67",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcurl-minimal-7.76.1-43.el9.x86_64.rpm",
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
    ],
)

rpm(
    name = "libeconf-0__0.4.1-7.el9.s390x",
    sha256 = "19b54d80020f15ff5753d0d116faa4dd2b358f1a55c4854ea7843aa89379954a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libeconf-0.4.1-7.el9.s390x.rpm",
    ],
)

rpm(
    name = "libeconf-0__0.4.1-7.el9.x86_64",
    sha256 = "5d852e2a7fbb298efeb05303c783afcebb369021337ca934df518362618de8f3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libeconf-0.4.1-7.el9.x86_64.rpm",
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
    name = "libfdisk-0__2.37.4-25.el9.aarch64",
    sha256 = "d724b6dd4dc886b1d598edc24d30ebb06dfc675252073e04838c56d0ed18e173",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libfdisk-2.37.4-25.el9.aarch64.rpm",
    ],
)

rpm(
    name = "libfdisk-0__2.37.4-25.el9.s390x",
    sha256 = "7584b9f892c5378bfa976d40c1e02e5a9ee058fd09ee14658aa13b1ab3448b6b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libfdisk-2.37.4-25.el9.s390x.rpm",
    ],
)

rpm(
    name = "libfdisk-0__2.37.4-25.el9.x86_64",
    sha256 = "57e990f6940ce2caed0d9578838549576535ad83f93ffc97df3bcbaf1ae72567",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libfdisk-2.37.4-25.el9.x86_64.rpm",
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
    name = "libgcc-0__11.5.0-14.el9.aarch64",
    sha256 = "ed0598c9cb4f10406c662d17ac2367eeba1e207683953410146927bba3d92c46",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libgcc-11.5.0-14.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ed0598c9cb4f10406c662d17ac2367eeba1e207683953410146927bba3d92c46",
    ],
)

rpm(
    name = "libgcc-0__11.5.0-14.el9.s390x",
    sha256 = "6ccddf8ec532ddc49d7b857ad46cb5404efc30a1ba2d4af575db77c402efdb8e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libgcc-11.5.0-14.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/6ccddf8ec532ddc49d7b857ad46cb5404efc30a1ba2d4af575db77c402efdb8e",
    ],
)

rpm(
    name = "libgcc-0__11.5.0-14.el9.x86_64",
    sha256 = "8e9b2f611466e02703348bfd7fbdc40035898c804dcc417b920d6ad77bf077e9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libgcc-11.5.0-14.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/8e9b2f611466e02703348bfd7fbdc40035898c804dcc417b920d6ad77bf077e9",
    ],
)

rpm(
    name = "libgcrypt-0__1.10.0-11.el9.aarch64",
    sha256 = "932bfe51b207e2ad8a0bd2b89e2fb33df73f3993586aaa4cc60576f57795e4db",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libgcrypt-1.10.0-11.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/932bfe51b207e2ad8a0bd2b89e2fb33df73f3993586aaa4cc60576f57795e4db",
    ],
)

rpm(
    name = "libgcrypt-0__1.10.0-11.el9.s390x",
    sha256 = "cf30c86fc1a18f504d639d3cbcf9e431af1ea639e6a5e7db1f6d30b763dd51a8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libgcrypt-1.10.0-11.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/cf30c86fc1a18f504d639d3cbcf9e431af1ea639e6a5e7db1f6d30b763dd51a8",
    ],
)

rpm(
    name = "libgcrypt-0__1.10.0-11.el9.x86_64",
    sha256 = "0323a74a5ad27bc3dc4ac4e9565825f37dc58b2a4800adbf33f767fa7a267c35",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libgcrypt-1.10.0-11.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/0323a74a5ad27bc3dc4ac4e9565825f37dc58b2a4800adbf33f767fa7a267c35",
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
    name = "libmount-0__2.37.4-25.el9.aarch64",
    sha256 = "903e1c5a61a57eafa8b68d5d23b1288cae061b65fdd4a942933cf8862ee4b1e3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libmount-2.37.4-25.el9.aarch64.rpm",
    ],
)

rpm(
    name = "libmount-0__2.37.4-25.el9.s390x",
    sha256 = "e4f81986fd3609aeaf6099697a7aebcd72dc96f160ee79c3dc2e8c8c5f1df10b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libmount-2.37.4-25.el9.s390x.rpm",
    ],
)

rpm(
    name = "libmount-0__2.37.4-25.el9.x86_64",
    sha256 = "ffb1ab2134b59539b097ce4a3c5287c61d2d4a626f512dbb93036d90ce2d755a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libmount-2.37.4-25.el9.x86_64.rpm",
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
    name = "libnghttp2-0__1.43.0-7.el9.aarch64",
    sha256 = "7702676980b7c34cc834be8da466c0381f846ca00d7e4bf41d54be77795c1027",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libnghttp2-1.43.0-7.el9.aarch64.rpm",
    ],
)

rpm(
    name = "libnghttp2-0__1.43.0-7.el9.s390x",
    sha256 = "6ce8782fd5fd6484df8206ad3f90d2f6b278ffcca82d5f2eab98a583f33563ed",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libnghttp2-1.43.0-7.el9.s390x.rpm",
    ],
)

rpm(
    name = "libnghttp2-0__1.43.0-7.el9.x86_64",
    sha256 = "2966ee44488ecd822e67ae030eeea4dc19b0323fa9f3da1fbd35dbbb42bc50aa",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnghttp2-1.43.0-7.el9.x86_64.rpm",
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
    name = "libseccomp-0__2.5.6-1.el9.aarch64",
    sha256 = "74a99b069ffe2fdd6f2ee19c73197c0ad1b71353df39c5af8c404932a5817974",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libseccomp-2.5.6-1.el9.aarch64.rpm",
    ],
)

rpm(
    name = "libseccomp-0__2.5.6-1.el9.s390x",
    sha256 = "155ef4319fc1fffa926ba688e12cd3d49e616f55474278b5df2e3a75d971d1a8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libseccomp-2.5.6-1.el9.s390x.rpm",
    ],
)

rpm(
    name = "libseccomp-0__2.5.6-1.el9.x86_64",
    sha256 = "73779d9eb83b4334fb312a7a6bcf7764780777f168724d7e57f6477fd912ac0a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libseccomp-2.5.6-1.el9.x86_64.rpm",
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
    ],
)

rpm(
    name = "libselinux-0__3.6-4.el9.s390x",
    sha256 = "98e1519df815f0f878f4c49810432c0ee305b1a52bb87c8f979e10570b3e1362",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libselinux-3.6-4.el9.s390x.rpm",
    ],
)

rpm(
    name = "libselinux-0__3.6-4.el9.x86_64",
    sha256 = "856d614fa2ba1a9d87ebc1ab78554a62c7fa6b7f37594dd9faaff1aac601ae94",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libselinux-3.6-4.el9.x86_64.rpm",
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
    name = "libsmartcols-0__2.37.4-25.el9.aarch64",
    sha256 = "a6c8e44ec15936163ca5075ede209fe4f4ec96a2b8656b517962f4db3f082951",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libsmartcols-2.37.4-25.el9.aarch64.rpm",
    ],
)

rpm(
    name = "libsmartcols-0__2.37.4-25.el9.s390x",
    sha256 = "b9f7f3209532892849db09656f9c2ccffbdda7c60fe1a0cc0c32d9efaeaf065e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libsmartcols-2.37.4-25.el9.s390x.rpm",
    ],
)

rpm(
    name = "libsmartcols-0__2.37.4-25.el9.x86_64",
    sha256 = "d3cc89b398cd94f8ead47a313ce1988b1b887b065842368b6a994559bca02b28",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsmartcols-2.37.4-25.el9.x86_64.rpm",
    ],
)

rpm(
    name = "libstdc__plus____plus__-0__11.5.0-14.el9.aarch64",
    sha256 = "ec5482f096781a16d55762e96be3f6b21ee2f714bc8e45327ea978ae87951cc0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libstdc++-11.5.0-14.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ec5482f096781a16d55762e96be3f6b21ee2f714bc8e45327ea978ae87951cc0",
    ],
)

rpm(
    name = "libstdc__plus____plus__-0__11.5.0-14.el9.s390x",
    sha256 = "e31be1174ae46e9e9cc6bce09d4cfd47eb280f96ef68488d4f0acefb2661a7df",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libstdc++-11.5.0-14.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/e31be1174ae46e9e9cc6bce09d4cfd47eb280f96ef68488d4f0acefb2661a7df",
    ],
)

rpm(
    name = "libstdc__plus____plus__-0__11.5.0-14.el9.x86_64",
    sha256 = "5b9119d93375d19b8ab140c359f9623de0fde1487fc1e930bfa29f54962ec448",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libstdc++-11.5.0-14.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/5b9119d93375d19b8ab140c359f9623de0fde1487fc1e930bfa29f54962ec448",
    ],
)

rpm(
    name = "libtasn1-0__4.16.0-10.el9.aarch64",
    sha256 = "18fee5d9b7dc486f774d1fac61238a6d6ac1a2dbdf61fdc38496838015e61712",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libtasn1-4.16.0-10.el9.aarch64.rpm",
    ],
)

rpm(
    name = "libtasn1-0__4.16.0-10.el9.s390x",
    sha256 = "ce71d8eb0cfb625616683e3db2db40bcb8bb7506c46dbea6097c2cc2b2d360fe",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libtasn1-4.16.0-10.el9.s390x.rpm",
    ],
)

rpm(
    name = "libtasn1-0__4.16.0-10.el9.x86_64",
    sha256 = "05f75ceb9f083ec511756eb9ed4078368c56ad55a6fe0abb819b8948e50b0d90",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libtasn1-4.16.0-10.el9.x86_64.rpm",
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
    name = "libuuid-0__2.37.4-25.el9.aarch64",
    sha256 = "5e740b232a2ab7deb56916d28ef026f16e3d5d11bedc7ceaa7381717193b3836",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libuuid-2.37.4-25.el9.aarch64.rpm",
    ],
)

rpm(
    name = "libuuid-0__2.37.4-25.el9.s390x",
    sha256 = "608adf99d9ad76624ef9d526748b8f0e95cc682edbe16e11ac22561b690dc0cd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libuuid-2.37.4-25.el9.s390x.rpm",
    ],
)

rpm(
    name = "libuuid-0__2.37.4-25.el9.x86_64",
    sha256 = "2305b6ddfd73d94cee66c8071d6ec30f7bd7e91792d76628b008c0d919e0c75e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libuuid-2.37.4-25.el9.x86_64.rpm",
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
    name = "libxcrypt-compat-0__4.4.18-3.el9.x86_64",
    sha256 = "3ea916c72412d3a7efd8c70cfa1ed18863c018091001b631390b19c454136b87",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libxcrypt-compat-4.4.18-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/3ea916c72412d3a7efd8c70cfa1ed18863c018091001b631390b19c454136b87",
    ],
)

rpm(
    name = "libxml2-0__2.9.13-15.el9.aarch64",
    sha256 = "a65a485b31ae542ee36712182597a7bedbfd2031641defd475bd20f2a36c23c6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libxml2-2.9.13-15.el9.aarch64.rpm",
    ],
)

rpm(
    name = "libxml2-0__2.9.13-15.el9.s390x",
    sha256 = "caad24c14431ad375c04c079b68fc823b8d12dafebb47d8e0a7bf5bc109d8578",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libxml2-2.9.13-15.el9.s390x.rpm",
    ],
)

rpm(
    name = "libxml2-0__2.9.13-15.el9.x86_64",
    sha256 = "210b7eb1c99d9bbcf0caae5deada0089a64e9ef8ca5c4a4212b868980504a126",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libxml2-2.9.13-15.el9.x86_64.rpm",
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
    name = "mpfr-0__4.1.0-10.el9.aarch64",
    sha256 = "bea56ccc46a2a14f3f2c8d9624675abc135e4f002e87c76541784b047d51764d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/mpfr-4.1.0-10.el9.aarch64.rpm",
    ],
)

rpm(
    name = "mpfr-0__4.1.0-10.el9.s390x",
    sha256 = "b166f1d2ae951d053a5761c826cd5bd8735412e465ce7cbfe78b1292c27aa10e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/mpfr-4.1.0-10.el9.s390x.rpm",
    ],
)

rpm(
    name = "mpfr-0__4.1.0-10.el9.x86_64",
    sha256 = "11c1d6b33b7e64ddc40faf45b949618c829bd2e3d3661132417e4c8aee6ab0fd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/mpfr-4.1.0-10.el9.x86_64.rpm",
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
    name = "nbdkit-vddk-plugin-0__1.38.5-12.el9.x86_64",
    sha256 = "370ba01c836494abd280e4490a8d7a5888e97f741e2f71fe5112a12ace71e6e0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-vddk-plugin-1.38.5-12.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/370ba01c836494abd280e4490a8d7a5888e97f741e2f71fe5112a12ace71e6e0",
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
    name = "netavark-2__1.17.2-1.el9.aarch64",
    sha256 = "430c661de4b4afd93309f90f246f7902c998d8140d405f9481ac3496af1e904b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/netavark-1.17.2-1.el9.aarch64.rpm",
    ],
)

rpm(
    name = "netavark-2__1.17.2-1.el9.s390x",
    sha256 = "be07e4252aed19450dc3e69c1d0889dfc0ba8698b55baf83d83d76b6ef871519",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/netavark-1.17.2-1.el9.s390x.rpm",
    ],
)

rpm(
    name = "netavark-2__1.17.2-1.el9.x86_64",
    sha256 = "1354a4ede09dc5d2400f0343c6f0dbbdd8ec3b5dc46759d4be71b8494e8b0ea5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/netavark-1.17.2-1.el9.x86_64.rpm",
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
    name = "nftables-1__1.0.9-7.el9.aarch64",
    sha256 = "b91eb3193da58eabccce8146270c9370550702e6590c02aa1371b21d2f198f76",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/nftables-1.0.9-7.el9.aarch64.rpm",
    ],
)

rpm(
    name = "nftables-1__1.0.9-7.el9.s390x",
    sha256 = "efb7e3971382ce36fa24a08b106cc726175aa71135e387a94c4d8b1d570fbce8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/nftables-1.0.9-7.el9.s390x.rpm",
    ],
)

rpm(
    name = "nftables-1__1.0.9-7.el9.x86_64",
    sha256 = "f315ae294239ab2486c817938d6ba30ca7e6eebaa66084203322fb5f245e129b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/nftables-1.0.9-7.el9.x86_64.rpm",
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
    name = "openldap-0__2.6.13-1.el9.aarch64",
    sha256 = "4ec33dce6b05c7f39f6d1d817fc5ee0460ddb2d30eb4ea55a3af4f13643d1ad4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openldap-2.6.13-1.el9.aarch64.rpm",
    ],
)

rpm(
    name = "openldap-0__2.6.13-1.el9.s390x",
    sha256 = "b8a4974ea6b1e8b307bf73054a22b6f4d3c34724ca6fa960d0b97978ab52290f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openldap-2.6.13-1.el9.s390x.rpm",
    ],
)

rpm(
    name = "openldap-0__2.6.13-1.el9.x86_64",
    sha256 = "6bed5684275d340e78f9300c4da665a6a0ea6779f7cee7217ddee868af81d8eb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openldap-2.6.13-1.el9.x86_64.rpm",
    ],
)

rpm(
    name = "openssl-1__3.5.5-3.el9.aarch64",
    sha256 = "652b9114598da895fb1aa59ff79ecd223b18b3917d892c8bf4272fb25896184f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openssl-3.5.5-3.el9.aarch64.rpm",
    ],
)

rpm(
    name = "openssl-1__3.5.5-3.el9.s390x",
    sha256 = "672b6c45d024e1fe4bfd33defae551994060651f8713b1d2e1d280b8ae4a5ad1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openssl-3.5.5-3.el9.s390x.rpm",
    ],
)

rpm(
    name = "openssl-1__3.5.5-3.el9.x86_64",
    sha256 = "c4afe380c4c7d027479fd16d2c9926d489156ffe1b5e5844aebe239a7fac6e09",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-3.5.5-3.el9.x86_64.rpm",
    ],
)

rpm(
    name = "openssl-fips-provider-1__3.5.5-3.el9.aarch64",
    sha256 = "16db538ae8f0f7ed9f64cbeebb90b2867e775795b783c1220417902e50883996",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openssl-fips-provider-3.5.5-3.el9.aarch64.rpm",
    ],
)

rpm(
    name = "openssl-fips-provider-1__3.5.5-3.el9.s390x",
    sha256 = "efdb7a7c7aca0f0fbcf27ae39f7922309da3548dcdcc2cf27b6657ec3539cee1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openssl-fips-provider-3.5.5-3.el9.s390x.rpm",
    ],
)

rpm(
    name = "openssl-fips-provider-1__3.5.5-3.el9.x86_64",
    sha256 = "203dc68c94582271b358860ecd79b63f2c6741b5f0f6e998be210e1d802a865d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-fips-provider-3.5.5-3.el9.x86_64.rpm",
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
    name = "openssl-libs-1__3.5.5-3.el9.aarch64",
    sha256 = "d28e252bcbbd3505ff603662c595f355eacd94116f6cd28616659ee28f230fee",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openssl-libs-3.5.5-3.el9.aarch64.rpm",
    ],
)

rpm(
    name = "openssl-libs-1__3.5.5-3.el9.s390x",
    sha256 = "d4e832d1a58b32c9e4129927df121801ef238cb83b6630c795b08a218f360488",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openssl-libs-3.5.5-3.el9.s390x.rpm",
    ],
)

rpm(
    name = "openssl-libs-1__3.5.5-3.el9.x86_64",
    sha256 = "a1b2f7f8dad2af4b2c9e3f5fecc37ca10d0f8ffba679b302df75c7a744173850",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-libs-3.5.5-3.el9.x86_64.rpm",
    ],
)

rpm(
    name = "ovirt-imageio-client-0__2.5.2-1.el9.aarch64",
    sha256 = "56a58715ac2a79bb678d3fbdc781900218402f9fe7187339b75422b77d39f265",
    urls = [
        "https://mirror.stream.centos.org/SIGs/9-stream/virt/aarch64/ovirt-45/Packages/o/ovirt-imageio-client-2.5.2-1.el9.aarch64.rpm",
    ],
)

rpm(
    name = "ovirt-imageio-client-0__2.5.3-0.202603060908.git0eb617c.el9.x86_64",
    sha256 = "c21e977f1387aef1d8fe04db1ce35eee310b2160bcc1ca3cda409f1990553497",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-9-x86_64/10195366-ovirt-imageio/ovirt-imageio-client-2.5.3-0.202603060908.git0eb617c.el9.x86_64.rpm",
    ],
)

rpm(
    name = "ovirt-imageio-common-0__2.5.2-1.el9.aarch64",
    sha256 = "1d4244f0979112c8f6768e689cf923f1313b3d6d885d512784276522be2485dc",
    urls = [
        "https://mirror.stream.centos.org/SIGs/9-stream/virt/aarch64/ovirt-45/Packages/o/ovirt-imageio-common-2.5.2-1.el9.aarch64.rpm",
    ],
)

rpm(
    name = "ovirt-imageio-common-0__2.5.3-0.202603060908.git0eb617c.el9.x86_64",
    sha256 = "85c1a1a6b4805a88a243388899cf363d8cc1f3b11a1dc7043b5b6e0cd080ad81",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-9-x86_64/10195366-ovirt-imageio/ovirt-imageio-common-2.5.3-0.202603060908.git0eb617c.el9.x86_64.rpm",
    ],
)

rpm(
    name = "ovirt-imageio-daemon-0__2.5.2-1.el9.aarch64",
    sha256 = "ddb50e2fad931c1dd8c9b88720ab94bb4e439fedfc4ec31918ceaca8c1310d50",
    urls = [
        "https://mirror.stream.centos.org/SIGs/9-stream/virt/aarch64/ovirt-45/Packages/o/ovirt-imageio-daemon-2.5.2-1.el9.aarch64.rpm",
    ],
)

rpm(
    name = "ovirt-imageio-daemon-0__2.5.3-0.202603060908.git0eb617c.el9.x86_64",
    sha256 = "190c4763d39b5b5f9f4051ecc6901ca8e04fc9362e1d9980d061a95ce0c9937a",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-9-x86_64/10195366-ovirt-imageio/ovirt-imageio-daemon-2.5.3-0.202603060908.git0eb617c.el9.x86_64.rpm",
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
    name = "p11-kit-0__0.26.2-1.el9.aarch64",
    sha256 = "078862b28f0e95c1464b8c8b85fd23a05351823acd3b60185af21a6ab5104271",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/p11-kit-0.26.2-1.el9.aarch64.rpm",
    ],
)

rpm(
    name = "p11-kit-0__0.26.2-1.el9.s390x",
    sha256 = "6743449ac49200da5f9ba3fcc8ef8f95880fbf8364ca67ccca5117dd9a126a0d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/p11-kit-0.26.2-1.el9.s390x.rpm",
    ],
)

rpm(
    name = "p11-kit-0__0.26.2-1.el9.x86_64",
    sha256 = "4e2f216f57ba90659679cb6cedcae7b38fb335a9d301c890ea7744b769ac15d8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/p11-kit-0.26.2-1.el9.x86_64.rpm",
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
    name = "p11-kit-trust-0__0.26.2-1.el9.aarch64",
    sha256 = "3db76997186c82a6c7b2ecf514b8098bfecf8db5358ebafdbed02b51b67465f6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/p11-kit-trust-0.26.2-1.el9.aarch64.rpm",
    ],
)

rpm(
    name = "p11-kit-trust-0__0.26.2-1.el9.s390x",
    sha256 = "30854a67c6e2bcc1584210f0991704c64323d9a367ea1a98429e9a6a2d25b9b0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/p11-kit-trust-0.26.2-1.el9.s390x.rpm",
    ],
)

rpm(
    name = "p11-kit-trust-0__0.26.2-1.el9.x86_64",
    sha256 = "d8dcb0fb0302e74bc2276e78d1bdcc2a512bcfaee86fe8b1d01e491bea6b250a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/p11-kit-trust-0.26.2-1.el9.x86_64.rpm",
    ],
)

rpm(
    name = "pam-0__1.5.1-29.el9.aarch64",
    sha256 = "090c497dc32e6bc3a95c0200f1aa1dfcd696f25ba5b082f0ff7ec249b25a8923",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/pam-1.5.1-29.el9.aarch64.rpm",
    ],
)

rpm(
    name = "pam-0__1.5.1-29.el9.s390x",
    sha256 = "692016ce57b3dd1a8a79640fc86c8ef6b2968e94ae59055532cf358b6704e652",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/pam-1.5.1-29.el9.s390x.rpm",
    ],
)

rpm(
    name = "pam-0__1.5.1-29.el9.x86_64",
    sha256 = "fb6521a7339de9b9be954d07aef4787867b85b45fdd78f65703bbd8819f6d585",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pam-1.5.1-29.el9.x86_64.rpm",
    ],
)

rpm(
    name = "passt-0__0__caret__20251210.gd04c480-3.el9.aarch64",
    sha256 = "79e82a17e04499d1903f8919d4faaa397208660d97dec233bb8fa4fa09a0a949",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/passt-0%5E20251210.gd04c480-3.el9.aarch64.rpm",
    ],
)

rpm(
    name = "passt-0__0__caret__20251210.gd04c480-3.el9.s390x",
    sha256 = "f7552b72b12988fe371a01b2b597f4dc1cc428fe0c97cec525cbe5b8f3f40c9c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/passt-0%5E20251210.gd04c480-3.el9.s390x.rpm",
    ],
)

rpm(
    name = "passt-0__0__caret__20251210.gd04c480-3.el9.x86_64",
    sha256 = "7071cee49af0aa56957ac9352224be6403e08753cd9bb0cf1f3ecff58bf9c0ea",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/passt-0%5E20251210.gd04c480-3.el9.x86_64.rpm",
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
    name = "python3-0__3.9.25-7.el9.aarch64",
    sha256 = "ce2840a142e3deef2dcd96642592a0301d9a3e11f2a70fd013e2f1a5fc9c1e4b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-3.9.25-7.el9.aarch64.rpm",
    ],
)

rpm(
    name = "python3-0__3.9.25-7.el9.s390x",
    sha256 = "7d998d6f76b41e2f25505459339ad0972bafd65bd874e335e6cd5e1c07ebab09",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-3.9.25-7.el9.s390x.rpm",
    ],
)

rpm(
    name = "python3-0__3.9.25-7.el9.x86_64",
    sha256 = "1ad043a0fe72a43612825abb9dca89432e03f223d38fb1410a3e0546dd5bdf85",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-3.9.25-7.el9.x86_64.rpm",
    ],
)

rpm(
    name = "python3-libs-0__3.9.25-7.el9.aarch64",
    sha256 = "5d5564faa281d97e1580035de69aa35a8b3869f170efa233e85dccf0374fd6d4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-libs-3.9.25-7.el9.aarch64.rpm",
    ],
)

rpm(
    name = "python3-libs-0__3.9.25-7.el9.s390x",
    sha256 = "860a3147b1f9f8c017694abe029ccb23133d1ecdb90a424c8c0ffc518e409703",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-libs-3.9.25-7.el9.s390x.rpm",
    ],
)

rpm(
    name = "python3-libs-0__3.9.25-7.el9.x86_64",
    sha256 = "83bb57e49f1a219e3ef6ebe07a7852b95bcaec63cec35c93b7923f53ea11da34",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-libs-3.9.25-7.el9.x86_64.rpm",
    ],
)

rpm(
    name = "python3-ovirt-engine-sdk4-0__4.6.3-1.el9.aarch64",
    sha256 = "fda97a26ffdcb3e6a136e7341d5c47a14dc1511a54a604709caa27c1fb096922",
    urls = [
        "https://mirror.stream.centos.org/SIGs/9-stream/virt/aarch64/ovirt-45/Packages/p/python3-ovirt-engine-sdk4-4.6.3-1.el9.aarch64.rpm",
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
    ],
)

rpm(
    name = "python3-pip-wheel-0__21.3.1-2.el9.s390x",
    sha256 = "c8a53917081942a659da7f98c64137c5a7aab2b25fc6cb948a3ce4bef0b59309",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-pip-wheel-21.3.1-2.el9.noarch.rpm",
    ],
)

rpm(
    name = "python3-pip-wheel-0__21.3.1-2.el9.x86_64",
    sha256 = "c8a53917081942a659da7f98c64137c5a7aab2b25fc6cb948a3ce4bef0b59309",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-pip-wheel-21.3.1-2.el9.noarch.rpm",
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
    name = "qemu-img-17__10.1.0-21.el9.aarch64",
    sha256 = "c90d04f06fb1b2c289feb4dc69d50ef35abcf3a7a3b32aa9af2df0d5b8a60ad2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/qemu-img-10.1.0-21.el9.aarch64.rpm",
    ],
)

rpm(
    name = "qemu-img-17__10.1.0-21.el9.s390x",
    sha256 = "5246b45cf414d96b0c517fee6b80fc870615f9a8e5f4b89ef4ef1ec0636995d4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/qemu-img-10.1.0-21.el9.s390x.rpm",
    ],
)

rpm(
    name = "qemu-img-17__10.1.0-21.el9.x86_64",
    sha256 = "15f15b9a7618524e0d83335881e155a5bd91f06d9ddfd95f84169daeb9638579",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-img-10.1.0-21.el9.x86_64.rpm",
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
    name = "sed-0__4.8-10.el9.aarch64",
    sha256 = "5a2930318f5ca770e800b2a42c05c945ccb02cd8ea3ed2b177d759d0e9090d5d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/sed-4.8-10.el9.aarch64.rpm",
    ],
)

rpm(
    name = "sed-0__4.8-10.el9.s390x",
    sha256 = "a515c69e92880844e6fbcf690421bd0d44304b642e5e56392a00ede362da5056",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/sed-4.8-10.el9.s390x.rpm",
    ],
)

rpm(
    name = "sed-0__4.8-10.el9.x86_64",
    sha256 = "8db670e1de34148e71c07f4ed8dbd5f41e1d6717325d5912a8651aa4e063b9e7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/sed-4.8-10.el9.x86_64.rpm",
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
    name = "shadow-utils-2__4.9-17.el9.aarch64",
    sha256 = "3edd4c583815a1e74b05972137144264bd2fe062106f63697fddffc4a5fc957d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/shadow-utils-4.9-17.el9.aarch64.rpm",
    ],
)

rpm(
    name = "shadow-utils-2__4.9-17.el9.s390x",
    sha256 = "df349811eb3501d6653321753b4bd37f15c69a024f9601208c974b53058b66ae",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/shadow-utils-4.9-17.el9.s390x.rpm",
    ],
)

rpm(
    name = "shadow-utils-2__4.9-17.el9.x86_64",
    sha256 = "1b9b0829668ce68f0ff0904fa651005e9c0c5e53b7481adb41f8f6b758d9e36a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/shadow-utils-4.9-17.el9.x86_64.rpm",
    ],
)

rpm(
    name = "shadow-utils-subid-2__4.9-17.el9.aarch64",
    sha256 = "28b8b6e8ec917190a43d14893afa977b4cf01c26422f3686e474c4bfb668c28d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/shadow-utils-subid-4.9-17.el9.aarch64.rpm",
    ],
)

rpm(
    name = "shadow-utils-subid-2__4.9-17.el9.s390x",
    sha256 = "d0b301c682c784d471138040de9c74e372ae0ffcf2dba8d9925d41ccd417391f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/shadow-utils-subid-4.9-17.el9.s390x.rpm",
    ],
)

rpm(
    name = "shadow-utils-subid-2__4.9-17.el9.x86_64",
    sha256 = "8792e56a4a0502fce0a15fdbe2eadcdef1a467874666a2ca15ad95a3112878c1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/shadow-utils-subid-4.9-17.el9.x86_64.rpm",
    ],
)

rpm(
    name = "sqlite-libs-0__3.34.1-10.el9.aarch64",
    sha256 = "249e02ba4ebd53311c9fa9e5604d88e9a6642edfa8873f274463feec0438d24d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/sqlite-libs-3.34.1-10.el9.aarch64.rpm",
    ],
)

rpm(
    name = "sqlite-libs-0__3.34.1-10.el9.s390x",
    sha256 = "46ddfde17c746f5c93e562064f1f9759a9c334fd65e199ef4f2a0fd32d70e077",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/sqlite-libs-3.34.1-10.el9.s390x.rpm",
    ],
)

rpm(
    name = "sqlite-libs-0__3.34.1-10.el9.x86_64",
    sha256 = "33e446234418090d66106865df8d65aa32d9021c9105cd3029e7a2a912fffac9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/sqlite-libs-3.34.1-10.el9.x86_64.rpm",
    ],
)

rpm(
    name = "systemd-0__252-70.el9.aarch64",
    sha256 = "56a3ba1ab100865efac0aec7ee725b0d33ade37865a6f568a353cb2737152161",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-252-70.el9.aarch64.rpm",
    ],
)

rpm(
    name = "systemd-0__252-70.el9.s390x",
    sha256 = "cd9e02788308afb50983838588dbe3584831a537dc3df7b27a380eeaf1ae9cae",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-252-70.el9.s390x.rpm",
    ],
)

rpm(
    name = "systemd-0__252-70.el9.x86_64",
    sha256 = "0dc961909989c36d432ceb04f45a753f228a35f19009f145b492074d593adbb1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-252-70.el9.x86_64.rpm",
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
    name = "systemd-libs-0__252-70.el9.aarch64",
    sha256 = "b260454352abcad0ba431ae42cf526527a02ae8c38df593d7b1fc485bd822529",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-libs-252-70.el9.aarch64.rpm",
    ],
)

rpm(
    name = "systemd-libs-0__252-70.el9.s390x",
    sha256 = "1afd6fb64ad26eae8d19b65accb0bce1fc99907cca1af32a27c3e48436270336",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-libs-252-70.el9.s390x.rpm",
    ],
)

rpm(
    name = "systemd-libs-0__252-70.el9.x86_64",
    sha256 = "7d742a81629f3f15b03f7cbed189c1fa0150894f9546b044e2183962d0f0f087",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-libs-252-70.el9.x86_64.rpm",
    ],
)

rpm(
    name = "systemd-pam-0__252-70.el9.aarch64",
    sha256 = "8374098d8f00d73b8217ba84e61e0f9eafae87af20593cc05acbe86554da7776",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-pam-252-70.el9.aarch64.rpm",
    ],
)

rpm(
    name = "systemd-pam-0__252-70.el9.s390x",
    sha256 = "d221c71ba3e61c5f84bc9381f23be20e5d35b259315cb93260fb2c43b6b8113d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-pam-252-70.el9.s390x.rpm",
    ],
)

rpm(
    name = "systemd-pam-0__252-70.el9.x86_64",
    sha256 = "ef873330d85eddaa4b634b02bd73e3c3347b2853b9f152f000c219cfb9759827",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-pam-252-70.el9.x86_64.rpm",
    ],
)

rpm(
    name = "systemd-rpm-macros-0__252-70.el9.aarch64",
    sha256 = "b051b1d8275f6b8a80f0943eb853aa6045c69de60c8c18d0237de8dcf334fc42",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-rpm-macros-252-70.el9.noarch.rpm",
    ],
)

rpm(
    name = "systemd-rpm-macros-0__252-70.el9.s390x",
    sha256 = "b051b1d8275f6b8a80f0943eb853aa6045c69de60c8c18d0237de8dcf334fc42",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-rpm-macros-252-70.el9.noarch.rpm",
    ],
)

rpm(
    name = "systemd-rpm-macros-0__252-70.el9.x86_64",
    sha256 = "b051b1d8275f6b8a80f0943eb853aa6045c69de60c8c18d0237de8dcf334fc42",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-rpm-macros-252-70.el9.noarch.rpm",
    ],
)

rpm(
    name = "tar-2__1.34-11.el9.aarch64",
    sha256 = "c9df1ef5362dca84f7731244d7cf09f70ccaf5ffdae6a45f78be6c0edb168330",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/tar-1.34-11.el9.aarch64.rpm",
    ],
)

rpm(
    name = "tar-2__1.34-11.el9.s390x",
    sha256 = "b309cdde22cd13ac6c89924b0b7e891d900c19a9181a2bb2b9e7c143924a940a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/tar-1.34-11.el9.s390x.rpm",
    ],
)

rpm(
    name = "tar-2__1.34-11.el9.x86_64",
    sha256 = "bd851918dd8d5df94f8a88a2e1825125fdc9bc7c6d8e8961f7b50d8299df9906",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/tar-1.34-11.el9.x86_64.rpm",
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
    name = "tzdata-0__2026b-1.el9.aarch64",
    sha256 = "579c30aeaede82f71525e9252f22dd5b1ad41e5ecc3bfa13393c4f8d2baaca46",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/tzdata-2026b-1.el9.noarch.rpm",
    ],
)

rpm(
    name = "tzdata-0__2026b-1.el9.s390x",
    sha256 = "579c30aeaede82f71525e9252f22dd5b1ad41e5ecc3bfa13393c4f8d2baaca46",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/tzdata-2026b-1.el9.noarch.rpm",
    ],
)

rpm(
    name = "tzdata-0__2026b-1.el9.x86_64",
    sha256 = "579c30aeaede82f71525e9252f22dd5b1ad41e5ecc3bfa13393c4f8d2baaca46",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/tzdata-2026b-1.el9.noarch.rpm",
    ],
)

rpm(
    name = "util-linux-0__2.37.4-25.el9.aarch64",
    sha256 = "619d39f84e40856b19475294d7e50417541261f852d5feeab75028a9a8f2fb20",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/util-linux-2.37.4-25.el9.aarch64.rpm",
    ],
)

rpm(
    name = "util-linux-0__2.37.4-25.el9.s390x",
    sha256 = "46a49c017dd8aefaa0d2f9353ecde0477fb9acf048e8e5c9d99ebf404775de05",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/util-linux-2.37.4-25.el9.s390x.rpm",
    ],
)

rpm(
    name = "util-linux-0__2.37.4-25.el9.x86_64",
    sha256 = "2d2b2ba4dea25b829031788e6afdc640412a42ac9b9e70a691aad219f744d0ec",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/util-linux-2.37.4-25.el9.x86_64.rpm",
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
    name = "util-linux-core-0__2.37.4-25.el9.aarch64",
    sha256 = "a31732e9e6c968665ff53330435674fdaa12f9812b309bda9babb29e0d2ca62d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/util-linux-core-2.37.4-25.el9.aarch64.rpm",
    ],
)

rpm(
    name = "util-linux-core-0__2.37.4-25.el9.s390x",
    sha256 = "a9c0f4b1c76cc105f42d9763d7a7df522e76f3668086a9cbf2b8318a4a4688e5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/util-linux-core-2.37.4-25.el9.s390x.rpm",
    ],
)

rpm(
    name = "util-linux-core-0__2.37.4-25.el9.x86_64",
    sha256 = "15c9e658afed9d50ce20908fd4080cd12042f4bf508f67b2ecbc889ae41c7414",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/util-linux-core-2.37.4-25.el9.x86_64.rpm",
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
    name = "vim-minimal-2__8.2.2637-31.el9.aarch64",
    sha256 = "de52f15dc69a763d8264f2416c84cd88fbf4944a32fe2b35b62e7136ebc22ae6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/vim-minimal-8.2.2637-31.el9.aarch64.rpm",
    ],
)

rpm(
    name = "vim-minimal-2__8.2.2637-31.el9.s390x",
    sha256 = "5c12af0ee160414916ef342b229287696da0eb296469f5c5810a356a220af535",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/vim-minimal-8.2.2637-31.el9.s390x.rpm",
    ],
)

rpm(
    name = "vim-minimal-2__8.2.2637-31.el9.x86_64",
    sha256 = "b7311b04f63c5c4c8cc055d3f0c2dc9c2aa7bb569a35f5aedb997c3dbd8c9f28",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/vim-minimal-8.2.2637-31.el9.x86_64.rpm",
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
    name = "yajl-0__2.1.0-25.el9.aarch64",
    sha256 = "d29c33e14aaa4b6685df599f6bd490010b1e270bbedf002ce6dd028ee9559c74",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/yajl-2.1.0-25.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/d29c33e14aaa4b6685df599f6bd490010b1e270bbedf002ce6dd028ee9559c74",
    ],
)

rpm(
    name = "yajl-0__2.1.0-25.el9.s390x",
    sha256 = "e4e05c1fad6db9a4a6dae85c8b851e5249d0c969b9ab00389f5d94fbfeff3a4f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/yajl-2.1.0-25.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/e4e05c1fad6db9a4a6dae85c8b851e5249d0c969b9ab00389f5d94fbfeff3a4f",
    ],
)

rpm(
    name = "yajl-0__2.1.0-25.el9.x86_64",
    sha256 = "f15f09fad761093398f946d4c218738fc10184930061c24554c98e098c01f01f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/yajl-2.1.0-25.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f15f09fad761093398f946d4c218738fc10184930061c24554c98e098c01f01f",
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
