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
    sha256 = "9a44f457810ce64ec36a244cc7c807607541ab88f2535e07e0bf2976ef4b73fe",
    strip_prefix = "bazel-lib-2.19.4",
    urls = [
        "https://github.com/bazel-contrib/bazel-lib/releases/download/v2.19.4/bazel-lib-v2.19.4.tar.gz",
        "https://storage.googleapis.com/builddeps/9a44f457810ce64ec36a244cc7c807607541ab88f2535e07e0bf2976ef4b73fe",
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

# bazel oci rules
http_archive(
    name = "rules_oci",
    sha256 = "5994ec0e8df92c319ef5da5e1f9b514628ceb8fc5824b4234f2fe635abb8cc2e",
    strip_prefix = "rules_oci-2.2.6",
    urls = [
        "https://github.com/bazel-contrib/rules_oci/releases/download/v2.2.6/rules_oci-v2.2.6.tar.gz",
        "https://storage.googleapis.com/builddeps/5994ec0e8df92c319ef5da5e1f9b514628ceb8fc5824b4234f2fe635abb8cc2e",
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
    name = "audit-libs-0__3.1.5-7.el9.aarch64",
    sha256 = "0687c4b4d23ad6219bb3d557266b0adad1e9a24c8061e2579bcc285cc53cf106",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/audit-libs-3.1.5-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/0687c4b4d23ad6219bb3d557266b0adad1e9a24c8061e2579bcc285cc53cf106",
    ],
)

rpm(
    name = "audit-libs-0__3.1.5-7.el9.s390x",
    sha256 = "441a308a28b5abdb631284dd849a2a1101d7b1e5034c50db1587e302157cbe48",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/audit-libs-3.1.5-7.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/441a308a28b5abdb631284dd849a2a1101d7b1e5034c50db1587e302157cbe48",
    ],
)

rpm(
    name = "audit-libs-0__3.1.5-7.el9.x86_64",
    sha256 = "7d503abfc0f88258b39d518ae9a1e8c25af4c77370c784f37553a2dd18c222e5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/audit-libs-3.1.5-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/7d503abfc0f88258b39d518ae9a1e8c25af4c77370c784f37553a2dd18c222e5",
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
    name = "buildah-2__1.41.3-1.el9.aarch64",
    sha256 = "6ac92ee095ae030038f8b4f2a2d1c0b2b66d8e2f85dd5b01f5c7b4d44bf7b14f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/buildah-1.41.3-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/6ac92ee095ae030038f8b4f2a2d1c0b2b66d8e2f85dd5b01f5c7b4d44bf7b14f",
    ],
)

rpm(
    name = "buildah-2__1.41.3-1.el9.s390x",
    sha256 = "2c8564bf93e03e7acb98bffd2cf22de502d7106efbb457c94661fdf621937143",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/buildah-1.41.3-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/2c8564bf93e03e7acb98bffd2cf22de502d7106efbb457c94661fdf621937143",
    ],
)

rpm(
    name = "buildah-2__1.41.3-1.el9.x86_64",
    sha256 = "74bc4bc951924f7812c72e2cc51ec200eb11e6725bfaee40c4d741db6e7d89b8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/buildah-1.41.3-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/74bc4bc951924f7812c72e2cc51ec200eb11e6725bfaee40c4d741db6e7d89b8",
    ],
)

rpm(
    name = "bzip2-libs-0__1.0.8-10.el9.aarch64",
    sha256 = "065787a932991bd8e7a705d8a977658cafab06f78cf2e405b68978a02718998e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/bzip2-libs-1.0.8-10.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/065787a932991bd8e7a705d8a977658cafab06f78cf2e405b68978a02718998e",
    ],
)

rpm(
    name = "bzip2-libs-0__1.0.8-10.el9.s390x",
    sha256 = "26d36d213959fba230d4c8550410d66e04b279ac8ccee7b8600680a87dde2d73",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/bzip2-libs-1.0.8-10.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/26d36d213959fba230d4c8550410d66e04b279ac8ccee7b8600680a87dde2d73",
    ],
)

rpm(
    name = "bzip2-libs-0__1.0.8-10.el9.x86_64",
    sha256 = "84392815cc1a8f01c651edd17f570aa449ef6f397ae48d773d655606ea7b4c96",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/bzip2-libs-1.0.8-10.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/84392815cc1a8f01c651edd17f570aa449ef6f397ae48d773d655606ea7b4c96",
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
    name = "centos-gpg-keys-0__9.0-32.el9.aarch64",
    sha256 = "f1e169c5596a4b2d3c860c8a7dfbcebf268455b3b2f01a94c1bb08d3f9a01477",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-gpg-keys-9.0-32.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/f1e169c5596a4b2d3c860c8a7dfbcebf268455b3b2f01a94c1bb08d3f9a01477",
    ],
)

rpm(
    name = "centos-gpg-keys-0__9.0-32.el9.s390x",
    sha256 = "f1e169c5596a4b2d3c860c8a7dfbcebf268455b3b2f01a94c1bb08d3f9a01477",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/centos-gpg-keys-9.0-32.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/f1e169c5596a4b2d3c860c8a7dfbcebf268455b3b2f01a94c1bb08d3f9a01477",
    ],
)

rpm(
    name = "centos-gpg-keys-0__9.0-32.el9.x86_64",
    sha256 = "f1e169c5596a4b2d3c860c8a7dfbcebf268455b3b2f01a94c1bb08d3f9a01477",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-gpg-keys-9.0-32.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/f1e169c5596a4b2d3c860c8a7dfbcebf268455b3b2f01a94c1bb08d3f9a01477",
    ],
)

rpm(
    name = "centos-logos-httpd-0__90.8-3.el9.aarch64",
    sha256 = "43492450e3f69bc51abea804c8eecd649cea09c686eb4259fe9acccd5fde9b24",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/centos-logos-httpd-90.8-3.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/43492450e3f69bc51abea804c8eecd649cea09c686eb4259fe9acccd5fde9b24",
    ],
)

rpm(
    name = "centos-logos-httpd-0__90.8-3.el9.s390x",
    sha256 = "43492450e3f69bc51abea804c8eecd649cea09c686eb4259fe9acccd5fde9b24",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/centos-logos-httpd-90.8-3.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/43492450e3f69bc51abea804c8eecd649cea09c686eb4259fe9acccd5fde9b24",
    ],
)

rpm(
    name = "centos-logos-httpd-0__90.8-3.el9.x86_64",
    sha256 = "43492450e3f69bc51abea804c8eecd649cea09c686eb4259fe9acccd5fde9b24",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/centos-logos-httpd-90.8-3.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/43492450e3f69bc51abea804c8eecd649cea09c686eb4259fe9acccd5fde9b24",
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
    name = "centos-stream-release-0__9.0-32.el9.aarch64",
    sha256 = "076a9d58d47102918123eac17af083e08e043305b872dd8d7086666b0924a5d2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-stream-release-9.0-32.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/076a9d58d47102918123eac17af083e08e043305b872dd8d7086666b0924a5d2",
    ],
)

rpm(
    name = "centos-stream-release-0__9.0-32.el9.s390x",
    sha256 = "076a9d58d47102918123eac17af083e08e043305b872dd8d7086666b0924a5d2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/centos-stream-release-9.0-32.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/076a9d58d47102918123eac17af083e08e043305b872dd8d7086666b0924a5d2",
    ],
)

rpm(
    name = "centos-stream-release-0__9.0-32.el9.x86_64",
    sha256 = "076a9d58d47102918123eac17af083e08e043305b872dd8d7086666b0924a5d2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-stream-release-9.0-32.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/076a9d58d47102918123eac17af083e08e043305b872dd8d7086666b0924a5d2",
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
    name = "centos-stream-repos-0__9.0-32.el9.aarch64",
    sha256 = "7efda20c86aa3b1af66ecff598f5d800282654908553ba263ad9cdbbc203feab",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-stream-repos-9.0-32.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/7efda20c86aa3b1af66ecff598f5d800282654908553ba263ad9cdbbc203feab",
    ],
)

rpm(
    name = "centos-stream-repos-0__9.0-32.el9.s390x",
    sha256 = "7efda20c86aa3b1af66ecff598f5d800282654908553ba263ad9cdbbc203feab",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/centos-stream-repos-9.0-32.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/7efda20c86aa3b1af66ecff598f5d800282654908553ba263ad9cdbbc203feab",
    ],
)

rpm(
    name = "centos-stream-repos-0__9.0-32.el9.x86_64",
    sha256 = "7efda20c86aa3b1af66ecff598f5d800282654908553ba263ad9cdbbc203feab",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-stream-repos-9.0-32.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/7efda20c86aa3b1af66ecff598f5d800282654908553ba263ad9cdbbc203feab",
    ],
)

rpm(
    name = "containers-common-4__1-134.el9.aarch64",
    sha256 = "47436ef0599b6674790c2490ac38ed04cb5e4d835a59a558092b03a8eb4e7724",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/containers-common-1-134.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/47436ef0599b6674790c2490ac38ed04cb5e4d835a59a558092b03a8eb4e7724",
    ],
)

rpm(
    name = "containers-common-4__1-134.el9.s390x",
    sha256 = "e7c359e3bed653c431e863920bbf66d733d5aa0f6e32f927bf5dadca53d085cd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/containers-common-1-134.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/e7c359e3bed653c431e863920bbf66d733d5aa0f6e32f927bf5dadca53d085cd",
    ],
)

rpm(
    name = "containers-common-4__1-134.el9.x86_64",
    sha256 = "a33c56fe5f8939076641d2775b671c5e0240558ec1c74fecafb4117eaa3b97aa",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/containers-common-1-134.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/a33c56fe5f8939076641d2775b671c5e0240558ec1c74fecafb4117eaa3b97aa",
    ],
)

rpm(
    name = "containers-common-extra-4__1-134.el9.aarch64",
    sha256 = "de0c2f17d3f51f8b559d26458fe8393726687b3e68e1c26bd3dd3a6cf3d04f1c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/containers-common-extra-1-134.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/de0c2f17d3f51f8b559d26458fe8393726687b3e68e1c26bd3dd3a6cf3d04f1c",
    ],
)

rpm(
    name = "containers-common-extra-4__1-134.el9.s390x",
    sha256 = "328d2715691d6100a30e8248e32ad206d34c55783a26bcdc86f57a91009cee69",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/containers-common-extra-1-134.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/328d2715691d6100a30e8248e32ad206d34c55783a26bcdc86f57a91009cee69",
    ],
)

rpm(
    name = "containers-common-extra-4__1-134.el9.x86_64",
    sha256 = "c4609a2786504147ff25acd697404fd48c63e2e111e798f368fcf26c18b5ccda",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/containers-common-extra-1-134.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c4609a2786504147ff25acd697404fd48c63e2e111e798f368fcf26c18b5ccda",
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
    name = "coreutils-single-0__8.32-39.el9.aarch64",
    sha256 = "ff8039cbb4fc624462abb4f556535fff128c99685834f6137db564d1b5a24c95",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/coreutils-single-8.32-39.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ff8039cbb4fc624462abb4f556535fff128c99685834f6137db564d1b5a24c95",
    ],
)

rpm(
    name = "coreutils-single-0__8.32-39.el9.s390x",
    sha256 = "33f20a9d1a8dcbe9b6e587bda728a91b5b014cd0b0c979c7908135c5fed23115",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/coreutils-single-8.32-39.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/33f20a9d1a8dcbe9b6e587bda728a91b5b014cd0b0c979c7908135c5fed23115",
    ],
)

rpm(
    name = "coreutils-single-0__8.32-39.el9.x86_64",
    sha256 = "09f7d8250c478a2931678063068adb8fccd2048d29fe9df31ca4e12c68f2ec7a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/coreutils-single-8.32-39.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/09f7d8250c478a2931678063068adb8fccd2048d29fe9df31ca4e12c68f2ec7a",
    ],
)

rpm(
    name = "cracklib-0__2.9.6-27.el9.aarch64",
    sha256 = "d92900088b558cd3c96c63db24b048a0f3ea575a0f8bfe66c26df4acfcb2f811",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/cracklib-2.9.6-27.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/d92900088b558cd3c96c63db24b048a0f3ea575a0f8bfe66c26df4acfcb2f811",
    ],
)

rpm(
    name = "cracklib-0__2.9.6-27.el9.s390x",
    sha256 = "f090c83e4fa8e5d170aaf13fe5c7795213d9d2ac0af16f92c60d6425a7b23253",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/cracklib-2.9.6-27.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/f090c83e4fa8e5d170aaf13fe5c7795213d9d2ac0af16f92c60d6425a7b23253",
    ],
)

rpm(
    name = "cracklib-0__2.9.6-27.el9.x86_64",
    sha256 = "be9deb2efd06b4b2c1c130acae94c687161d04830119e65a989d904ba9fd1864",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/cracklib-2.9.6-27.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/be9deb2efd06b4b2c1c130acae94c687161d04830119e65a989d904ba9fd1864",
    ],
)

rpm(
    name = "cracklib-dicts-0__2.9.6-27.el9.aarch64",
    sha256 = "bfd16ac0aebb165d43d3139448ab8eac66d4d67c9eac506c3f3bef799f1352c2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/cracklib-dicts-2.9.6-27.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/bfd16ac0aebb165d43d3139448ab8eac66d4d67c9eac506c3f3bef799f1352c2",
    ],
)

rpm(
    name = "cracklib-dicts-0__2.9.6-27.el9.s390x",
    sha256 = "bac458a7a96be0b856d6c3294c5675fa159694d111fae63819f0a70dc3c6ccf0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/cracklib-dicts-2.9.6-27.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/bac458a7a96be0b856d6c3294c5675fa159694d111fae63819f0a70dc3c6ccf0",
    ],
)

rpm(
    name = "cracklib-dicts-0__2.9.6-27.el9.x86_64",
    sha256 = "01df2a72fcdf988132e82764ce1a22a5a9513fa253b54e17d23058bdb53c2d85",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/cracklib-dicts-2.9.6-27.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/01df2a72fcdf988132e82764ce1a22a5a9513fa253b54e17d23058bdb53c2d85",
    ],
)

rpm(
    name = "crun-0__1.24-1.el9.aarch64",
    sha256 = "37dbbe22d434bf0cf795941f3b92ff8d68f2369fb369b50cde2f4085271cd741",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/crun-1.24-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/37dbbe22d434bf0cf795941f3b92ff8d68f2369fb369b50cde2f4085271cd741",
    ],
)

rpm(
    name = "crun-0__1.24-1.el9.s390x",
    sha256 = "5dcf4766b4de4c2b20d3d88b8569a81c4c82db641687eb095cbedcc42cd4a7f9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/crun-1.24-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/5dcf4766b4de4c2b20d3d88b8569a81c4c82db641687eb095cbedcc42cd4a7f9",
    ],
)

rpm(
    name = "crun-0__1.24-1.el9.x86_64",
    sha256 = "01b2521faaf49ccd04681608f081c8a583617693b4bb3a321e2a648d48619ad7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/crun-1.24-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/01b2521faaf49ccd04681608f081c8a583617693b4bb3a321e2a648d48619ad7",
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
    name = "crypto-policies-0__20251126-1.gite9c4db2.el9.aarch64",
    sha256 = "38c1e40b477795017996db0683b72004a4810d88a320ae0554e6736b118c5c9a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/crypto-policies-20251126-1.gite9c4db2.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/38c1e40b477795017996db0683b72004a4810d88a320ae0554e6736b118c5c9a",
    ],
)

rpm(
    name = "crypto-policies-0__20251126-1.gite9c4db2.el9.s390x",
    sha256 = "38c1e40b477795017996db0683b72004a4810d88a320ae0554e6736b118c5c9a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/crypto-policies-20251126-1.gite9c4db2.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/38c1e40b477795017996db0683b72004a4810d88a320ae0554e6736b118c5c9a",
    ],
)

rpm(
    name = "crypto-policies-0__20251126-1.gite9c4db2.el9.x86_64",
    sha256 = "38c1e40b477795017996db0683b72004a4810d88a320ae0554e6736b118c5c9a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/crypto-policies-20251126-1.gite9c4db2.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/38c1e40b477795017996db0683b72004a4810d88a320ae0554e6736b118c5c9a",
    ],
)

rpm(
    name = "crypto-policies-scripts-0__20251126-1.gite9c4db2.el9.aarch64",
    sha256 = "3e66975146af2b48bdaa9bec11132f4fc8106b78b638c594b30a898a46a9639b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/crypto-policies-scripts-20251126-1.gite9c4db2.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/3e66975146af2b48bdaa9bec11132f4fc8106b78b638c594b30a898a46a9639b",
    ],
)

rpm(
    name = "crypto-policies-scripts-0__20251126-1.gite9c4db2.el9.s390x",
    sha256 = "3e66975146af2b48bdaa9bec11132f4fc8106b78b638c594b30a898a46a9639b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/crypto-policies-scripts-20251126-1.gite9c4db2.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/3e66975146af2b48bdaa9bec11132f4fc8106b78b638c594b30a898a46a9639b",
    ],
)

rpm(
    name = "crypto-policies-scripts-0__20251126-1.gite9c4db2.el9.x86_64",
    sha256 = "3e66975146af2b48bdaa9bec11132f4fc8106b78b638c594b30a898a46a9639b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/crypto-policies-scripts-20251126-1.gite9c4db2.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/3e66975146af2b48bdaa9bec11132f4fc8106b78b638c594b30a898a46a9639b",
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
    name = "curl-0__7.76.1-38.el9.aarch64",
    sha256 = "1731bba07b74a8edfbbb121ebe8d57c832783c68b6c297fdd37ba911e98354c2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/curl-7.76.1-38.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/1731bba07b74a8edfbbb121ebe8d57c832783c68b6c297fdd37ba911e98354c2",
    ],
)

rpm(
    name = "curl-0__7.76.1-38.el9.s390x",
    sha256 = "94da9420c536c7240cb37b2a6b895c97ed3afa1384823612755ba2d29dcb12ea",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/curl-7.76.1-38.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/94da9420c536c7240cb37b2a6b895c97ed3afa1384823612755ba2d29dcb12ea",
    ],
)

rpm(
    name = "curl-0__7.76.1-38.el9.x86_64",
    sha256 = "5f3f80026be5ee050adfb728b52adac91e2dd81f069cfb6ea811e5fc80423ca8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/curl-7.76.1-38.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/5f3f80026be5ee050adfb728b52adac91e2dd81f069cfb6ea811e5fc80423ca8",
    ],
)

rpm(
    name = "curl-minimal-0__7.76.1-38.el9.aarch64",
    sha256 = "1d3c6c9e7c30fccf05cd480dbc0774d724d68f9f225f0e969be07f9d3d3f1cf2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/curl-minimal-7.76.1-38.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/1d3c6c9e7c30fccf05cd480dbc0774d724d68f9f225f0e969be07f9d3d3f1cf2",
    ],
)

rpm(
    name = "curl-minimal-0__7.76.1-38.el9.s390x",
    sha256 = "02058c1f9a825e6459762cfc33c95e8909a4fc04c6a4d24539fc4881d0754657",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/curl-minimal-7.76.1-38.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/02058c1f9a825e6459762cfc33c95e8909a4fc04c6a4d24539fc4881d0754657",
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
    name = "dbus-1__1.12.20-8.el9.aarch64",
    sha256 = "29c244f31d9f3ae910a6b95d4d5534cdf1ea4870fc277e29876a10cf3bd193ae",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/dbus-1.12.20-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/29c244f31d9f3ae910a6b95d4d5534cdf1ea4870fc277e29876a10cf3bd193ae",
    ],
)

rpm(
    name = "dbus-1__1.12.20-8.el9.s390x",
    sha256 = "a99d278716899bb35100d4c9c26a66a795d309555d8d71ef6d1739e2f44cf44d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/dbus-1.12.20-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/a99d278716899bb35100d4c9c26a66a795d309555d8d71ef6d1739e2f44cf44d",
    ],
)

rpm(
    name = "dbus-1__1.12.20-8.el9.x86_64",
    sha256 = "d13d52df79bb9a0a1795530a5ce1134c9c92a2a7c401dfc3827ee8bf02f60018",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-1.12.20-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/d13d52df79bb9a0a1795530a5ce1134c9c92a2a7c401dfc3827ee8bf02f60018",
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
    name = "dbus-common-1__1.12.20-8.el9.aarch64",
    sha256 = "ff91286d9413256c50886a0c96b3d5d0773bd25284b9a94b28b98a5215f09a56",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/dbus-common-1.12.20-8.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/ff91286d9413256c50886a0c96b3d5d0773bd25284b9a94b28b98a5215f09a56",
    ],
)

rpm(
    name = "dbus-common-1__1.12.20-8.el9.s390x",
    sha256 = "ff91286d9413256c50886a0c96b3d5d0773bd25284b9a94b28b98a5215f09a56",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/dbus-common-1.12.20-8.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/ff91286d9413256c50886a0c96b3d5d0773bd25284b9a94b28b98a5215f09a56",
    ],
)

rpm(
    name = "dbus-common-1__1.12.20-8.el9.x86_64",
    sha256 = "ff91286d9413256c50886a0c96b3d5d0773bd25284b9a94b28b98a5215f09a56",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-common-1.12.20-8.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/ff91286d9413256c50886a0c96b3d5d0773bd25284b9a94b28b98a5215f09a56",
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
    name = "fips-provider-next-0__1.2.0-7.el9.aarch64",
    sha256 = "e8c9fcd61b0ade060233a0c9755b8ca59705838844bddebef0ab5cd48fbaab9e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/fips-provider-next-1.2.0-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/e8c9fcd61b0ade060233a0c9755b8ca59705838844bddebef0ab5cd48fbaab9e",
    ],
)

rpm(
    name = "fips-provider-next-0__1.2.0-7.el9.s390x",
    sha256 = "0f1f863a00b32b0517c785c0b68f839426cae4c087fde39c8c5d91ab3cdf2ee4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/fips-provider-next-1.2.0-7.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/0f1f863a00b32b0517c785c0b68f839426cae4c087fde39c8c5d91ab3cdf2ee4",
    ],
)

rpm(
    name = "fips-provider-next-0__1.2.0-7.el9.x86_64",
    sha256 = "1f78c1d7f88f2494994d8735676c2afd529606c8f59c2a7b830e993831b6ce2c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/fips-provider-next-1.2.0-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/1f78c1d7f88f2494994d8735676c2afd529606c8f59c2a7b830e993831b6ce2c",
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
    name = "glib2-0__2.68.4-18.el9.aarch64",
    sha256 = "853c09c092d7f03ebad72f3865f627743d8017592379dc185ab71bcd1c21ace0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glib2-2.68.4-18.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/853c09c092d7f03ebad72f3865f627743d8017592379dc185ab71bcd1c21ace0",
    ],
)

rpm(
    name = "glib2-0__2.68.4-18.el9.s390x",
    sha256 = "df038353062f506dfcab712f8a589952643ab674da0c9d5dcfbe796505facc31",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glib2-2.68.4-18.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/df038353062f506dfcab712f8a589952643ab674da0c9d5dcfbe796505facc31",
    ],
)

rpm(
    name = "glib2-0__2.68.4-18.el9.x86_64",
    sha256 = "17c3385c8d88811fa7ef5faadc61f8292be23987757fafeb9cf341ac5906d8f6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glib2-2.68.4-18.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/17c3385c8d88811fa7ef5faadc61f8292be23987757fafeb9cf341ac5906d8f6",
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
    name = "glibc-0__2.34-245.el9.aarch64",
    sha256 = "bf6a0f12d012662ae124aa18ed00d4f270709cda83bd60b16f6a93138bcd1e50",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-2.34-245.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/bf6a0f12d012662ae124aa18ed00d4f270709cda83bd60b16f6a93138bcd1e50",
    ],
)

rpm(
    name = "glibc-0__2.34-245.el9.s390x",
    sha256 = "b0682a36512f5e61724aba0e1af041481454ee23d290e5978f3e5016615ef962",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glibc-2.34-245.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/b0682a36512f5e61724aba0e1af041481454ee23d290e5978f3e5016615ef962",
    ],
)

rpm(
    name = "glibc-0__2.34-245.el9.x86_64",
    sha256 = "b4a8c93a28d59a4070ee7a3f7f517d0ce4e9c11886477e6d1b5707718892817e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-2.34-245.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/b4a8c93a28d59a4070ee7a3f7f517d0ce4e9c11886477e6d1b5707718892817e",
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
    name = "glibc-common-0__2.34-245.el9.aarch64",
    sha256 = "4645238ef9126cd91fd4ff37fc681693479779de7cf14fae852605caf9423156",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-common-2.34-245.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/4645238ef9126cd91fd4ff37fc681693479779de7cf14fae852605caf9423156",
    ],
)

rpm(
    name = "glibc-common-0__2.34-245.el9.s390x",
    sha256 = "2d58050c2282d9961bdf565a86e175fc14161369dc4a0631d491a23250b22990",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glibc-common-2.34-245.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/2d58050c2282d9961bdf565a86e175fc14161369dc4a0631d491a23250b22990",
    ],
)

rpm(
    name = "glibc-common-0__2.34-245.el9.x86_64",
    sha256 = "747ea3dbcdbe9b3b8c26ac70ea8e117a79e62f50df0b8754a645c2ef68e05be2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-common-2.34-245.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/747ea3dbcdbe9b3b8c26ac70ea8e117a79e62f50df0b8754a645c2ef68e05be2",
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
    name = "glibc-minimal-langpack-0__2.34-245.el9.aarch64",
    sha256 = "0b208faa1cefdafb7a71544746f0d858696d3c2e60861858c145098836ff3aa3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-minimal-langpack-2.34-245.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/0b208faa1cefdafb7a71544746f0d858696d3c2e60861858c145098836ff3aa3",
    ],
)

rpm(
    name = "glibc-minimal-langpack-0__2.34-245.el9.s390x",
    sha256 = "a325deb26609c8f9746e9143f8a3a1d993cd5acf707ba5870e75353fedc87d15",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glibc-minimal-langpack-2.34-245.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/a325deb26609c8f9746e9143f8a3a1d993cd5acf707ba5870e75353fedc87d15",
    ],
)

rpm(
    name = "glibc-minimal-langpack-0__2.34-245.el9.x86_64",
    sha256 = "fe736927c6e91ef4d6ba4f1b467f373e6ab8a47c490c2a1e1d5508ae402f45fa",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-minimal-langpack-2.34-245.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/fe736927c6e91ef4d6ba4f1b467f373e6ab8a47c490c2a1e1d5508ae402f45fa",
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
    name = "gnupg2-0__2.3.3-4.el9.aarch64",
    sha256 = "ea254e4f615d3865263236e433e3fe674fd58b842134e72f07db80a50df0f0cb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gnupg2-2.3.3-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ea254e4f615d3865263236e433e3fe674fd58b842134e72f07db80a50df0f0cb",
    ],
)

rpm(
    name = "gnupg2-0__2.3.3-4.el9.s390x",
    sha256 = "79f4d4ce2953babbca0f5ba558b633e8e2a03bb5745f6e9340dfe83c7181c782",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gnupg2-2.3.3-4.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/79f4d4ce2953babbca0f5ba558b633e8e2a03bb5745f6e9340dfe83c7181c782",
    ],
)

rpm(
    name = "gnupg2-0__2.3.3-4.el9.x86_64",
    sha256 = "03e7697ffc0ae9301c30adccfe28d3b100063e5d2c7c5f87dc21f1c56af4052f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gnupg2-2.3.3-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/03e7697ffc0ae9301c30adccfe28d3b100063e5d2c7c5f87dc21f1c56af4052f",
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
    name = "gnutls-0__3.8.10-1.el9.aarch64",
    sha256 = "1fc349d0b52a990b5d8c48d198fa0f803d64c44178bc5bdee7c2f955bfc9db3e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gnutls-3.8.10-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/1fc349d0b52a990b5d8c48d198fa0f803d64c44178bc5bdee7c2f955bfc9db3e",
    ],
)

rpm(
    name = "gnutls-0__3.8.10-1.el9.s390x",
    sha256 = "5954883818d3db398a6bd41aecb90abaa77add033ed44c38dbb0b0614136991c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gnutls-3.8.10-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/5954883818d3db398a6bd41aecb90abaa77add033ed44c38dbb0b0614136991c",
    ],
)

rpm(
    name = "gnutls-0__3.8.10-1.el9.x86_64",
    sha256 = "36c81bdd8a256606063dbf854d67a9764828844d274285a712aa8493bc6d2479",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gnutls-3.8.10-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/36c81bdd8a256606063dbf854d67a9764828844d274285a712aa8493bc6d2479",
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
    name = "krb5-libs-0__1.21.1-8.el9.aarch64",
    sha256 = "7671147fe79cb1fa2fd011cee451be325a8e4aca6290ff67eb9e4d01f0c1edfd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/krb5-libs-1.21.1-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/7671147fe79cb1fa2fd011cee451be325a8e4aca6290ff67eb9e4d01f0c1edfd",
    ],
)

rpm(
    name = "krb5-libs-0__1.21.1-8.el9.s390x",
    sha256 = "ea2d05d119e5b9072dd4c58d74079accfd24bae196d012e0de105ab3e0bcc892",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/krb5-libs-1.21.1-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/ea2d05d119e5b9072dd4c58d74079accfd24bae196d012e0de105ab3e0bcc892",
    ],
)

rpm(
    name = "krb5-libs-0__1.21.1-8.el9.x86_64",
    sha256 = "d5d3473637d3453c3216b1f14fef500e10bd8158c99bbadb48b788b88d92786f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/krb5-libs-1.21.1-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/d5d3473637d3453c3216b1f14fef500e10bd8158c99bbadb48b788b88d92786f",
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
    name = "libblkid-0__2.37.4-21.el9.aarch64",
    sha256 = "ddbde77138e33d01fe88e275f351f8f31754eb26ac5665190437049e867f2b17",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libblkid-2.37.4-21.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ddbde77138e33d01fe88e275f351f8f31754eb26ac5665190437049e867f2b17",
    ],
)

rpm(
    name = "libblkid-0__2.37.4-21.el9.s390x",
    sha256 = "28b355ddec6b299b6526c80032cd3f9a74fbf7f965bb3c425352a7e616cab287",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libblkid-2.37.4-21.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/28b355ddec6b299b6526c80032cd3f9a74fbf7f965bb3c425352a7e616cab287",
    ],
)

rpm(
    name = "libblkid-0__2.37.4-21.el9.x86_64",
    sha256 = "2433f8829f894c7c5ba0639eb37a18a92632d4f9383551c901434b4353f96fc4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libblkid-2.37.4-21.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/2433f8829f894c7c5ba0639eb37a18a92632d4f9383551c901434b4353f96fc4",
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
    name = "libcurl-minimal-0__7.76.1-38.el9.aarch64",
    sha256 = "3875300de7f37b3711aa53508c11547141e7e415020ec397377328c90a978271",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libcurl-minimal-7.76.1-38.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/3875300de7f37b3711aa53508c11547141e7e415020ec397377328c90a978271",
    ],
)

rpm(
    name = "libcurl-minimal-0__7.76.1-38.el9.s390x",
    sha256 = "63bd401440d939c2f5ea18012a5c07b352b26d2e28efb0c0759484caa816d42c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libcurl-minimal-7.76.1-38.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/63bd401440d939c2f5ea18012a5c07b352b26d2e28efb0c0759484caa816d42c",
    ],
)

rpm(
    name = "libcurl-minimal-0__7.76.1-38.el9.x86_64",
    sha256 = "15a372db1e01940e235eb1109e00c49e382aadac467425dc361610de1e57ef1b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcurl-minimal-7.76.1-38.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/15a372db1e01940e235eb1109e00c49e382aadac467425dc361610de1e57ef1b",
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
    name = "libeconf-0__0.4.1-5.el9.aarch64",
    sha256 = "40675233785bf5ffc0e97cd559efd2e9f4b8b8c392013bb6c9441c6e01b332d6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libeconf-0.4.1-5.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/40675233785bf5ffc0e97cd559efd2e9f4b8b8c392013bb6c9441c6e01b332d6",
    ],
)

rpm(
    name = "libeconf-0__0.4.1-5.el9.s390x",
    sha256 = "025025697e1f9d222fb6224be5f6a463ad630971c67f4a55ff8d70fc25780443",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libeconf-0.4.1-5.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/025025697e1f9d222fb6224be5f6a463ad630971c67f4a55ff8d70fc25780443",
    ],
)

rpm(
    name = "libeconf-0__0.4.1-5.el9.x86_64",
    sha256 = "aba2474e57729f395e1918638270e7aa7cf8de8f3fc31b81f9412888320459e8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libeconf-0.4.1-5.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/aba2474e57729f395e1918638270e7aa7cf8de8f3fc31b81f9412888320459e8",
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
    name = "libfdisk-0__2.37.4-21.el9.aarch64",
    sha256 = "e8c908711f60ab15256f368ee1a5d9b4570e2c19139ec52bb73876d379f98f7a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libfdisk-2.37.4-21.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/e8c908711f60ab15256f368ee1a5d9b4570e2c19139ec52bb73876d379f98f7a",
    ],
)

rpm(
    name = "libfdisk-0__2.37.4-21.el9.s390x",
    sha256 = "67f5d8d46714139b75cd84941201e0fa0e1eebd0b26762accfb11d6b4c8b51bb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libfdisk-2.37.4-21.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/67f5d8d46714139b75cd84941201e0fa0e1eebd0b26762accfb11d6b4c8b51bb",
    ],
)

rpm(
    name = "libfdisk-0__2.37.4-21.el9.x86_64",
    sha256 = "9a594c51e3bf09cb5016485ee2f143de6db960ff1c7e135c0097f59fa51b2edb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libfdisk-2.37.4-21.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/9a594c51e3bf09cb5016485ee2f143de6db960ff1c7e135c0097f59fa51b2edb",
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
    name = "libmount-0__2.37.4-21.el9.aarch64",
    sha256 = "84c61be8eee5f148ece4b17cab7b9774bd4e5e51377ff459c278dadc99d733d6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libmount-2.37.4-21.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/84c61be8eee5f148ece4b17cab7b9774bd4e5e51377ff459c278dadc99d733d6",
    ],
)

rpm(
    name = "libmount-0__2.37.4-21.el9.s390x",
    sha256 = "d7f982570169708f7ff3ba976dd3df4c64cf1d9a197f5da98d7dae2e9f5f2cb2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libmount-2.37.4-21.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/d7f982570169708f7ff3ba976dd3df4c64cf1d9a197f5da98d7dae2e9f5f2cb2",
    ],
)

rpm(
    name = "libmount-0__2.37.4-21.el9.x86_64",
    sha256 = "d8bfc70d1a9a594569c8c95bda682804a20bb4ee602db3efa7b6e76d289ecc66",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libmount-2.37.4-21.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/d8bfc70d1a9a594569c8c95bda682804a20bb4ee602db3efa7b6e76d289ecc66",
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
    name = "libnghttp2-0__1.43.0-6.el9.aarch64",
    sha256 = "b9c3685701dc2ad11adac83055811bb8c4909bd73469f31953ef7d534c747b83",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libnghttp2-1.43.0-6.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/b9c3685701dc2ad11adac83055811bb8c4909bd73469f31953ef7d534c747b83",
    ],
)

rpm(
    name = "libnghttp2-0__1.43.0-6.el9.s390x",
    sha256 = "6d9ea7820d952bb492ff575b87fd46c606acf12bd368a5b4c8df3efc6a054c57",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libnghttp2-1.43.0-6.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/6d9ea7820d952bb492ff575b87fd46c606acf12bd368a5b4c8df3efc6a054c57",
    ],
)

rpm(
    name = "libnghttp2-0__1.43.0-6.el9.x86_64",
    sha256 = "fc1cadbc6cf37cbea60112b7ae6f92fabfd5a7f76fa526bb5a1ea82746455ec7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnghttp2-1.43.0-6.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/fc1cadbc6cf37cbea60112b7ae6f92fabfd5a7f76fa526bb5a1ea82746455ec7",
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
    name = "libseccomp-0__2.5.2-2.el9.aarch64",
    sha256 = "ee31abd3d1325b05c5ba336158ba3b235a718a99ad5cec5e6ab498ca99b688b5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libseccomp-2.5.2-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ee31abd3d1325b05c5ba336158ba3b235a718a99ad5cec5e6ab498ca99b688b5",
    ],
)

rpm(
    name = "libseccomp-0__2.5.2-2.el9.s390x",
    sha256 = "1479993c13970d0a69826051948a080ea216fb74f0717d8718801065edf1a1de",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libseccomp-2.5.2-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/1479993c13970d0a69826051948a080ea216fb74f0717d8718801065edf1a1de",
    ],
)

rpm(
    name = "libseccomp-0__2.5.2-2.el9.x86_64",
    sha256 = "d5c1c4473ebf5fd9c605eb866118d7428cdec9b188db18e45545801cc2a689c3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libseccomp-2.5.2-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/d5c1c4473ebf5fd9c605eb866118d7428cdec9b188db18e45545801cc2a689c3",
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
    name = "libselinux-0__3.6-3.el9.aarch64",
    sha256 = "42b6190d9e4ea6019059991f50965ac6267012343241f0cc64fd24c6e20aaa2a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libselinux-3.6-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/42b6190d9e4ea6019059991f50965ac6267012343241f0cc64fd24c6e20aaa2a",
    ],
)

rpm(
    name = "libselinux-0__3.6-3.el9.s390x",
    sha256 = "16b3c0c73dcfff8b54a5554a4bcbd639603508d8502857c05ff9aa2360690094",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libselinux-3.6-3.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/16b3c0c73dcfff8b54a5554a4bcbd639603508d8502857c05ff9aa2360690094",
    ],
)

rpm(
    name = "libselinux-0__3.6-3.el9.x86_64",
    sha256 = "79abe72ea8dccb4134286fd1aae79827f10bde0cc1c35224886e93b293d282d1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libselinux-3.6-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/79abe72ea8dccb4134286fd1aae79827f10bde0cc1c35224886e93b293d282d1",
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
    name = "libsmartcols-0__2.37.4-21.el9.aarch64",
    sha256 = "022b23b2500972666db40ab9d5278ca704c54600e8443dea8e327910afa5b411",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libsmartcols-2.37.4-21.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/022b23b2500972666db40ab9d5278ca704c54600e8443dea8e327910afa5b411",
    ],
)

rpm(
    name = "libsmartcols-0__2.37.4-21.el9.s390x",
    sha256 = "94929db8ee7027fca21e8ac17519dfcf9366a5a7ba8c8ba627dcb5976b199036",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libsmartcols-2.37.4-21.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/94929db8ee7027fca21e8ac17519dfcf9366a5a7ba8c8ba627dcb5976b199036",
    ],
)

rpm(
    name = "libsmartcols-0__2.37.4-21.el9.x86_64",
    sha256 = "30e2a071ad6f1939f14fc89c827d61ccb28a6cbf6e443db39e8019a18c7e18d4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsmartcols-2.37.4-21.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/30e2a071ad6f1939f14fc89c827d61ccb28a6cbf6e443db39e8019a18c7e18d4",
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
    name = "libtasn1-0__4.16.0-9.el9.aarch64",
    sha256 = "7b99e8f1081ba2c511021b666b9f8176abb31168920e86c392cd45299f400b59",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libtasn1-4.16.0-9.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/7b99e8f1081ba2c511021b666b9f8176abb31168920e86c392cd45299f400b59",
    ],
)

rpm(
    name = "libtasn1-0__4.16.0-9.el9.s390x",
    sha256 = "0ebbc12c3ae3f270efef2965bb77d6e806733eb07505ec7a33468f0fd72360bd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libtasn1-4.16.0-9.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/0ebbc12c3ae3f270efef2965bb77d6e806733eb07505ec7a33468f0fd72360bd",
    ],
)

rpm(
    name = "libtasn1-0__4.16.0-9.el9.x86_64",
    sha256 = "addd155d4abc41529d7e8588f442e50a87db3a1314bd2162fbb4950d898a2e28",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libtasn1-4.16.0-9.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/addd155d4abc41529d7e8588f442e50a87db3a1314bd2162fbb4950d898a2e28",
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
    name = "libuuid-0__2.37.4-21.el9.aarch64",
    sha256 = "9ef80033bf2bbba7aca3a8f789a48a7597f2113271c6da9d9f71bca213aae259",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libuuid-2.37.4-21.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/9ef80033bf2bbba7aca3a8f789a48a7597f2113271c6da9d9f71bca213aae259",
    ],
)

rpm(
    name = "libuuid-0__2.37.4-21.el9.s390x",
    sha256 = "b604d6ef2f49c29d93601f6cad1ac04d83f14ec719ba0f561d6912948d6f2b56",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libuuid-2.37.4-21.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/b604d6ef2f49c29d93601f6cad1ac04d83f14ec719ba0f561d6912948d6f2b56",
    ],
)

rpm(
    name = "libuuid-0__2.37.4-21.el9.x86_64",
    sha256 = "be4793be5af11772206abe023746ec4021a8b7bc124fdc7e7cdb92b57c46d125",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libuuid-2.37.4-21.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/be4793be5af11772206abe023746ec4021a8b7bc124fdc7e7cdb92b57c46d125",
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
    name = "libxml2-0__2.9.13-14.el9.aarch64",
    sha256 = "f62d552977c2b1d53cc4f6d4e9ea91fa7c0351dcd3a5bec8ceb7f91bc1157aaf",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libxml2-2.9.13-14.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f62d552977c2b1d53cc4f6d4e9ea91fa7c0351dcd3a5bec8ceb7f91bc1157aaf",
    ],
)

rpm(
    name = "libxml2-0__2.9.13-14.el9.s390x",
    sha256 = "78256fb046360c848f4f4d2a0419705ce747e87de18bc0d3994c6d5865656992",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libxml2-2.9.13-14.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/78256fb046360c848f4f4d2a0419705ce747e87de18bc0d3994c6d5865656992",
    ],
)

rpm(
    name = "libxml2-0__2.9.13-14.el9.x86_64",
    sha256 = "6e3e385bd23d1f0ecf30859d65eaaaa9583c814a9afb8e04379b1eeca21a54c3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libxml2-2.9.13-14.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/6e3e385bd23d1f0ecf30859d65eaaaa9583c814a9afb8e04379b1eeca21a54c3",
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
    name = "mpfr-0__4.1.0-7.el9.aarch64",
    sha256 = "f3bd8510505a53450abe05dc34edbc5313fe89a6f88d0252624205dc7bb884c7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/mpfr-4.1.0-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f3bd8510505a53450abe05dc34edbc5313fe89a6f88d0252624205dc7bb884c7",
    ],
)

rpm(
    name = "mpfr-0__4.1.0-7.el9.s390x",
    sha256 = "7297fc0b6869453925eed12b13c17ed76379352f63e0303644bef64386b034f1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/mpfr-4.1.0-7.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/7297fc0b6869453925eed12b13c17ed76379352f63e0303644bef64386b034f1",
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
    name = "netavark-2__1.16.0-1.el9.aarch64",
    sha256 = "66407b2fddee934a857585a112e62f721945c2a738dedf7f94e74185fda811fc",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/netavark-1.16.0-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/66407b2fddee934a857585a112e62f721945c2a738dedf7f94e74185fda811fc",
    ],
)

rpm(
    name = "netavark-2__1.16.0-1.el9.s390x",
    sha256 = "973047b55a61340b75a432ee3088f1fc414cffec11b6914361697ef43b230c6e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/netavark-1.16.0-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/973047b55a61340b75a432ee3088f1fc414cffec11b6914361697ef43b230c6e",
    ],
)

rpm(
    name = "netavark-2__1.16.0-1.el9.x86_64",
    sha256 = "27a92c3bdbbb53ccbff4831a6b89821bbb24124f43e804287f0c680130345b0d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/netavark-1.16.0-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/27a92c3bdbbb53ccbff4831a6b89821bbb24124f43e804287f0c680130345b0d",
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
    name = "nftables-1__1.0.9-5.el9.aarch64",
    sha256 = "011751772267a732b9a43dd49eb452585976fa6a0f16f5e9f18afec5a1d8c84b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/nftables-1.0.9-5.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/011751772267a732b9a43dd49eb452585976fa6a0f16f5e9f18afec5a1d8c84b",
    ],
)

rpm(
    name = "nftables-1__1.0.9-5.el9.s390x",
    sha256 = "5a28370b8aaa521298bf11330a6b2120dbb334514e6fb29a9ea866b41afecf1d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/nftables-1.0.9-5.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/5a28370b8aaa521298bf11330a6b2120dbb334514e6fb29a9ea866b41afecf1d",
    ],
)

rpm(
    name = "nftables-1__1.0.9-5.el9.x86_64",
    sha256 = "857422e089a8a64ffde8d16a1d96ad25266a0856afa6c26d74869298fc61f949",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/nftables-1.0.9-5.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/857422e089a8a64ffde8d16a1d96ad25266a0856afa6c26d74869298fc61f949",
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
    name = "openldap-0__2.6.8-4.el9.aarch64",
    sha256 = "f1ed58d5d8f8571832d62b6e5c62995abc54e92f7fe0bc684e5514eb967e26cb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openldap-2.6.8-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f1ed58d5d8f8571832d62b6e5c62995abc54e92f7fe0bc684e5514eb967e26cb",
    ],
)

rpm(
    name = "openldap-0__2.6.8-4.el9.s390x",
    sha256 = "67a9f53b14250f7934faebcc7961b2f9c06875b9c8fb16d4ac19c0e110d0e446",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openldap-2.6.8-4.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/67a9f53b14250f7934faebcc7961b2f9c06875b9c8fb16d4ac19c0e110d0e446",
    ],
)

rpm(
    name = "openldap-0__2.6.8-4.el9.x86_64",
    sha256 = "606794339fa964e9c68a94fe756566e0651c52da47ebe7df5fb66f700c6a0421",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openldap-2.6.8-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/606794339fa964e9c68a94fe756566e0651c52da47ebe7df5fb66f700c6a0421",
    ],
)

rpm(
    name = "openssl-1__3.5.1-6.el9.aarch64",
    sha256 = "2ccbccea48480d5c472421293e954cd6e62650b21ad96b1b742fde60ea3e97b6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openssl-3.5.1-6.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/2ccbccea48480d5c472421293e954cd6e62650b21ad96b1b742fde60ea3e97b6",
    ],
)

rpm(
    name = "openssl-1__3.5.1-6.el9.s390x",
    sha256 = "f1ae3fc9f6161add50b625e80db89b2b9a055f6ca83ca6875d2858ecadb0c589",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openssl-3.5.1-6.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/f1ae3fc9f6161add50b625e80db89b2b9a055f6ca83ca6875d2858ecadb0c589",
    ],
)

rpm(
    name = "openssl-1__3.5.1-6.el9.x86_64",
    sha256 = "e4af5e44ea1a1180bec9fb46fc10fa6ee79e6719a56a6f879b61d94feb27e6b8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-3.5.1-6.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e4af5e44ea1a1180bec9fb46fc10fa6ee79e6719a56a6f879b61d94feb27e6b8",
    ],
)

rpm(
    name = "openssl-fips-provider-1__3.5.1-6.el9.aarch64",
    sha256 = "c9778f5e918fcae3e71781d8df4ce7766bb725f3c8d1641b1f119b9d515c9e88",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openssl-fips-provider-3.5.1-6.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c9778f5e918fcae3e71781d8df4ce7766bb725f3c8d1641b1f119b9d515c9e88",
    ],
)

rpm(
    name = "openssl-fips-provider-1__3.5.1-6.el9.s390x",
    sha256 = "4b0e5272bc8d93abd657186b7053234fef1976ccea3d63a15acb16ad3d5ea5a1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openssl-fips-provider-3.5.1-6.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/4b0e5272bc8d93abd657186b7053234fef1976ccea3d63a15acb16ad3d5ea5a1",
    ],
)

rpm(
    name = "openssl-fips-provider-1__3.5.1-6.el9.x86_64",
    sha256 = "161f584784ac6a7f2b2a01802fe62038dc1685755cf0b35053e82401b35191f9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-fips-provider-3.5.1-6.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/161f584784ac6a7f2b2a01802fe62038dc1685755cf0b35053e82401b35191f9",
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
    name = "openssl-libs-1__3.5.1-6.el9.aarch64",
    sha256 = "ff45d27bdb19ccfaf03b4409f43b8b7309b7c497e13f93c7c2e83e94ea9fbb2d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openssl-libs-3.5.1-6.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ff45d27bdb19ccfaf03b4409f43b8b7309b7c497e13f93c7c2e83e94ea9fbb2d",
    ],
)

rpm(
    name = "openssl-libs-1__3.5.1-6.el9.s390x",
    sha256 = "38006d9d1a561fc69d5f66ab57d5cea8ae58ce75578d435fd0e87583469ebb27",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openssl-libs-3.5.1-6.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/38006d9d1a561fc69d5f66ab57d5cea8ae58ce75578d435fd0e87583469ebb27",
    ],
)

rpm(
    name = "openssl-libs-1__3.5.1-6.el9.x86_64",
    sha256 = "53c2206ba2503ae256b26815cd2e9ed80edb4f9030f544072765dcb30bb2c63b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-libs-3.5.1-6.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/53c2206ba2503ae256b26815cd2e9ed80edb4f9030f544072765dcb30bb2c63b",
    ],
)

rpm(
    name = "ovirt-imageio-client-0__2.5.0-1.el9.aarch64",
    sha256 = "1c4052438bc21a3e27054f96f5fc8296839a965624a709d3ff77fc8a50bcea1c",
    urls = [
        "https://mirror.stream.centos.org/SIGs/9-stream/virt/aarch64/ovirt-45/Packages/o/ovirt-imageio-client-2.5.0-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/1c4052438bc21a3e27054f96f5fc8296839a965624a709d3ff77fc8a50bcea1c",
    ],
)

rpm(
    name = "ovirt-imageio-client-0__2.5.3-0.202512031333.gitd245f11.el9.x86_64",
    sha256 = "38ce96c072791176844830df404198ce0c6bd6de8e7414ceee476279b1cc8e68",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-9-x86_64/09860507-ovirt-imageio/ovirt-imageio-client-2.5.3-0.202512031333.gitd245f11.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/38ce96c072791176844830df404198ce0c6bd6de8e7414ceee476279b1cc8e68",
    ],
)

rpm(
    name = "ovirt-imageio-common-0__2.5.0-1.el9.aarch64",
    sha256 = "c019c42107de69e37331ea0493225e1eb5ffd55d70b45551b7d1b5456d2bdcfa",
    urls = [
        "https://mirror.stream.centos.org/SIGs/9-stream/virt/aarch64/ovirt-45/Packages/o/ovirt-imageio-common-2.5.0-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c019c42107de69e37331ea0493225e1eb5ffd55d70b45551b7d1b5456d2bdcfa",
    ],
)

rpm(
    name = "ovirt-imageio-common-0__2.5.3-0.202512031333.gitd245f11.el9.x86_64",
    sha256 = "c7c5c7133b1ecd624f2a08e82ffe71f8466eea492ca75c871f4029b532ca0f99",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-9-x86_64/09860507-ovirt-imageio/ovirt-imageio-common-2.5.3-0.202512031333.gitd245f11.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c7c5c7133b1ecd624f2a08e82ffe71f8466eea492ca75c871f4029b532ca0f99",
    ],
)

rpm(
    name = "ovirt-imageio-daemon-0__2.5.0-1.el9.aarch64",
    sha256 = "402f4490db47113a4298eb227a774ad9dd299eb052c7ad72a1ac2b3cdc649ba0",
    urls = [
        "https://mirror.stream.centos.org/SIGs/9-stream/virt/aarch64/ovirt-45/Packages/o/ovirt-imageio-daemon-2.5.0-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/402f4490db47113a4298eb227a774ad9dd299eb052c7ad72a1ac2b3cdc649ba0",
    ],
)

rpm(
    name = "ovirt-imageio-daemon-0__2.5.3-0.202512031333.gitd245f11.el9.x86_64",
    sha256 = "e13bae7048c8992634accaeae793c450e2e9d585ae740ca33ed0f42f30a0fd87",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-9-x86_64/09860507-ovirt-imageio/ovirt-imageio-daemon-2.5.3-0.202512031333.gitd245f11.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e13bae7048c8992634accaeae793c450e2e9d585ae740ca33ed0f42f30a0fd87",
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
    name = "p11-kit-0__0.25.10-1.el9.aarch64",
    sha256 = "7261dae50bbdb554d377474ff7e63335a4ce24b7c7e4c260b6222d576d5eab4c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/p11-kit-0.25.10-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/7261dae50bbdb554d377474ff7e63335a4ce24b7c7e4c260b6222d576d5eab4c",
    ],
)

rpm(
    name = "p11-kit-0__0.25.10-1.el9.s390x",
    sha256 = "d092b1ae0a83ca945e883b5bc118689421125ae5b3d70bada835781128bcdcfd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/p11-kit-0.25.10-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/d092b1ae0a83ca945e883b5bc118689421125ae5b3d70bada835781128bcdcfd",
    ],
)

rpm(
    name = "p11-kit-0__0.25.10-1.el9.x86_64",
    sha256 = "74c93f38dfe580e230c17e6ff4c3750f7ded5bb2156e755b2880637e0bd826fb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/p11-kit-0.25.10-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/74c93f38dfe580e230c17e6ff4c3750f7ded5bb2156e755b2880637e0bd826fb",
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
    name = "p11-kit-trust-0__0.25.10-1.el9.aarch64",
    sha256 = "4740a15386d1e461cf1b097d2d0e96691f8cabacccb36da7114dd2093704c601",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/p11-kit-trust-0.25.10-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/4740a15386d1e461cf1b097d2d0e96691f8cabacccb36da7114dd2093704c601",
    ],
)

rpm(
    name = "p11-kit-trust-0__0.25.10-1.el9.s390x",
    sha256 = "4170d77a9b584f0291879dbcc93ef90f3e75cf83d8d2e30bcfe7add96a94c4df",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/p11-kit-trust-0.25.10-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/4170d77a9b584f0291879dbcc93ef90f3e75cf83d8d2e30bcfe7add96a94c4df",
    ],
)

rpm(
    name = "p11-kit-trust-0__0.25.10-1.el9.x86_64",
    sha256 = "9c3ae19ab9dcb1f2ec73e66f36d8fe562754028b32568b74c4e96a4a2f052bac",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/p11-kit-trust-0.25.10-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/9c3ae19ab9dcb1f2ec73e66f36d8fe562754028b32568b74c4e96a4a2f052bac",
    ],
)

rpm(
    name = "pam-0__1.5.1-28.el9.aarch64",
    sha256 = "598477ca76dadefb1c80d4322c2b074aac54d9ecf3d717353641b939147d8caa",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/pam-1.5.1-28.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/598477ca76dadefb1c80d4322c2b074aac54d9ecf3d717353641b939147d8caa",
    ],
)

rpm(
    name = "pam-0__1.5.1-28.el9.s390x",
    sha256 = "f823ac185f1a0c966e608301f09e22a645e37c447f4dd49b6b6930ff67fd8156",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/pam-1.5.1-28.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/f823ac185f1a0c966e608301f09e22a645e37c447f4dd49b6b6930ff67fd8156",
    ],
)

rpm(
    name = "pam-0__1.5.1-28.el9.x86_64",
    sha256 = "3c92fd1347d78fc3621cd5ae62f7a159588d86a8a0c22ba4a754dcd51926b6b7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pam-1.5.1-28.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/3c92fd1347d78fc3621cd5ae62f7a159588d86a8a0c22ba4a754dcd51926b6b7",
    ],
)

rpm(
    name = "passt-0__0__caret__20251210.gd04c480-1.el9.aarch64",
    sha256 = "0025a309b84289c8fe4279c9114b93a19ec9017a82cf7556874e07d3137916de",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/passt-0%5E20251210.gd04c480-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/0025a309b84289c8fe4279c9114b93a19ec9017a82cf7556874e07d3137916de",
    ],
)

rpm(
    name = "passt-0__0__caret__20251210.gd04c480-1.el9.s390x",
    sha256 = "ed71721855d066d9f23856012dc7b3facacfccb21851a5e0563863c082342b65",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/passt-0%5E20251210.gd04c480-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/ed71721855d066d9f23856012dc7b3facacfccb21851a5e0563863c082342b65",
    ],
)

rpm(
    name = "passt-0__0__caret__20251210.gd04c480-1.el9.x86_64",
    sha256 = "dd48328bb08bdf7d191195777e0e00c8295f40f52f74aab9e96e297f7407cbc8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/passt-0%5E20251210.gd04c480-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/dd48328bb08bdf7d191195777e0e00c8295f40f52f74aab9e96e297f7407cbc8",
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
    name = "python3-0__3.9.25-2.el9.aarch64",
    sha256 = "5e881547d46068b264ab3c48be7bbc694f507a4a686b01ac35cde24fa7e61695",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-3.9.25-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/5e881547d46068b264ab3c48be7bbc694f507a4a686b01ac35cde24fa7e61695",
    ],
)

rpm(
    name = "python3-0__3.9.25-2.el9.s390x",
    sha256 = "0ec8a873e271b43321ef2b9d83d5e89074333e53904d641f4304a27ba7c3b725",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-3.9.25-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/0ec8a873e271b43321ef2b9d83d5e89074333e53904d641f4304a27ba7c3b725",
    ],
)

rpm(
    name = "python3-0__3.9.25-2.el9.x86_64",
    sha256 = "460258e5c11e3aab69a0947e86088ce814882ab1da5b614e55685afd01d34182",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-3.9.25-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/460258e5c11e3aab69a0947e86088ce814882ab1da5b614e55685afd01d34182",
    ],
)

rpm(
    name = "python3-libs-0__3.9.25-2.el9.aarch64",
    sha256 = "ef4943bd444f4ad131da26e9022ca42de7237c6b8e243901bd57dff5f10eb48c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-libs-3.9.25-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ef4943bd444f4ad131da26e9022ca42de7237c6b8e243901bd57dff5f10eb48c",
    ],
)

rpm(
    name = "python3-libs-0__3.9.25-2.el9.s390x",
    sha256 = "2cade6778104c7ee617835d0e8a0980843be663910b5a0f5a8935167f05452e4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-libs-3.9.25-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/2cade6778104c7ee617835d0e8a0980843be663910b5a0f5a8935167f05452e4",
    ],
)

rpm(
    name = "python3-libs-0__3.9.25-2.el9.x86_64",
    sha256 = "67adff19349747e4143f25ab43c30933f01be3dc04647f676a43e5cb3dcff00c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-libs-3.9.25-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/67adff19349747e4143f25ab43c30933f01be3dc04647f676a43e5cb3dcff00c",
    ],
)

rpm(
    name = "python3-ovirt-engine-sdk4-0__4.6.2-1.el9.aarch64",
    sha256 = "c7348b63dac8c3514f3811aa18760e13ce3a81d6fbf6bfcdbb69ed9b2c9b9f56",
    urls = [
        "https://mirror.stream.centos.org/SIGs/9-stream/virt/aarch64/ovirt-45/Packages/p/python3-ovirt-engine-sdk4-4.6.2-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c7348b63dac8c3514f3811aa18760e13ce3a81d6fbf6bfcdbb69ed9b2c9b9f56",
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
    name = "python3-pip-wheel-0__21.3.1-1.el9.aarch64",
    sha256 = "1c8096f1dd57c5d6db4d1391cafb15326431923ba139f3119015773a307f80d9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-pip-wheel-21.3.1-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/1c8096f1dd57c5d6db4d1391cafb15326431923ba139f3119015773a307f80d9",
    ],
)

rpm(
    name = "python3-pip-wheel-0__21.3.1-1.el9.s390x",
    sha256 = "1c8096f1dd57c5d6db4d1391cafb15326431923ba139f3119015773a307f80d9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-pip-wheel-21.3.1-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/1c8096f1dd57c5d6db4d1391cafb15326431923ba139f3119015773a307f80d9",
    ],
)

rpm(
    name = "python3-pip-wheel-0__21.3.1-1.el9.x86_64",
    sha256 = "1c8096f1dd57c5d6db4d1391cafb15326431923ba139f3119015773a307f80d9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-pip-wheel-21.3.1-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/1c8096f1dd57c5d6db4d1391cafb15326431923ba139f3119015773a307f80d9",
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
    name = "qemu-img-17__10.1.0-9.el9.aarch64",
    sha256 = "1b0d7f6fb08ba2929c8771d388ba681ddcdbf7028e41e596c414b19b59873501",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/qemu-img-10.1.0-9.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/1b0d7f6fb08ba2929c8771d388ba681ddcdbf7028e41e596c414b19b59873501",
    ],
)

rpm(
    name = "qemu-img-17__10.1.0-9.el9.s390x",
    sha256 = "4dff72d35e5f122ae7f93d3137067c54b27e80a45b1c40e51b0b4dfcc0c32b5b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/qemu-img-10.1.0-9.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/4dff72d35e5f122ae7f93d3137067c54b27e80a45b1c40e51b0b4dfcc0c32b5b",
    ],
)

rpm(
    name = "qemu-img-17__10.1.0-9.el9.x86_64",
    sha256 = "d8ca31dbcd5ae2dfe2c65198534cd3daf54342877d83bcd3df94e0c2ce902575",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-img-10.1.0-9.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/d8ca31dbcd5ae2dfe2c65198534cd3daf54342877d83bcd3df94e0c2ce902575",
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
    name = "sed-0__4.8-9.el9.aarch64",
    sha256 = "cfdec0f026af984c11277ae613f16af7a86ea6170aac3da495a027599fdc8e3d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/sed-4.8-9.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/cfdec0f026af984c11277ae613f16af7a86ea6170aac3da495a027599fdc8e3d",
    ],
)

rpm(
    name = "sed-0__4.8-9.el9.s390x",
    sha256 = "7185b39912949fe56bc0a9bd6463b1c2dc1206efa00dadecfd6e37c9028e1575",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/sed-4.8-9.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/7185b39912949fe56bc0a9bd6463b1c2dc1206efa00dadecfd6e37c9028e1575",
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
    name = "shadow-utils-2__4.9-16.el9.aarch64",
    sha256 = "085a4d0d20ee46e72564939e92533fbf4c049658c58d4e7cc075d5da5baa7098",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/shadow-utils-4.9-16.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/085a4d0d20ee46e72564939e92533fbf4c049658c58d4e7cc075d5da5baa7098",
    ],
)

rpm(
    name = "shadow-utils-2__4.9-16.el9.s390x",
    sha256 = "18c43c994a4c8f6c97c195f2bf30ffad338b3cf5ee08e3d813731dbcedf51b4e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/shadow-utils-4.9-16.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/18c43c994a4c8f6c97c195f2bf30ffad338b3cf5ee08e3d813731dbcedf51b4e",
    ],
)

rpm(
    name = "shadow-utils-2__4.9-16.el9.x86_64",
    sha256 = "f82dcf66ba99287eaebe3225cb01d252eea40202b0b263a2b2619f87d98918fd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/shadow-utils-4.9-16.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f82dcf66ba99287eaebe3225cb01d252eea40202b0b263a2b2619f87d98918fd",
    ],
)

rpm(
    name = "shadow-utils-subid-2__4.9-16.el9.aarch64",
    sha256 = "9e1db07548bab241bfbe7a1eb72a2d985afc2d21637b051b1f7b35eba1ea7c65",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/shadow-utils-subid-4.9-16.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/9e1db07548bab241bfbe7a1eb72a2d985afc2d21637b051b1f7b35eba1ea7c65",
    ],
)

rpm(
    name = "shadow-utils-subid-2__4.9-16.el9.s390x",
    sha256 = "1af1f5d0cf897c20ac45860cc8280a11ff8b61530bf24caeb4df93b10fb24ab5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/shadow-utils-subid-4.9-16.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/1af1f5d0cf897c20ac45860cc8280a11ff8b61530bf24caeb4df93b10fb24ab5",
    ],
)

rpm(
    name = "shadow-utils-subid-2__4.9-16.el9.x86_64",
    sha256 = "d8e770aefab1a3c3243d578b59b01ee9f76981af461ef31d95a42a3d1c2d5a9c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/shadow-utils-subid-4.9-16.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/d8e770aefab1a3c3243d578b59b01ee9f76981af461ef31d95a42a3d1c2d5a9c",
    ],
)

rpm(
    name = "sqlite-libs-0__3.34.1-9.el9.aarch64",
    sha256 = "115fbe01cb007257cbd277bb83e63eb211dd244f10e150e5e71b3edca68181c6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/sqlite-libs-3.34.1-9.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/115fbe01cb007257cbd277bb83e63eb211dd244f10e150e5e71b3edca68181c6",
    ],
)

rpm(
    name = "sqlite-libs-0__3.34.1-9.el9.s390x",
    sha256 = "759932a92153b14492308c8330aa398fcabec87f2b42635a231b455431032f3b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/sqlite-libs-3.34.1-9.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/759932a92153b14492308c8330aa398fcabec87f2b42635a231b455431032f3b",
    ],
)

rpm(
    name = "sqlite-libs-0__3.34.1-9.el9.x86_64",
    sha256 = "88abee2ce36a11d707d610e796afcd2919a7adc3bb5ba4e92b08ef06046e6970",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/sqlite-libs-3.34.1-9.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/88abee2ce36a11d707d610e796afcd2919a7adc3bb5ba4e92b08ef06046e6970",
    ],
)

rpm(
    name = "systemd-0__252-59.el9.aarch64",
    sha256 = "f3036e3e6e5864122aaa50a84752b39c9bfadb78adc7a1012723265c3ca01a7d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-252-59.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f3036e3e6e5864122aaa50a84752b39c9bfadb78adc7a1012723265c3ca01a7d",
    ],
)

rpm(
    name = "systemd-0__252-59.el9.s390x",
    sha256 = "70b777ae908a8f5b140fb8a828ce15fa33177e56967c9165a4e10406148cd84b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-252-59.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/70b777ae908a8f5b140fb8a828ce15fa33177e56967c9165a4e10406148cd84b",
    ],
)

rpm(
    name = "systemd-0__252-59.el9.x86_64",
    sha256 = "e3fdaa30e7ea1a1f073f49285d10f5788061aefd90aaea1fece8eb991245c3c8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-252-59.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e3fdaa30e7ea1a1f073f49285d10f5788061aefd90aaea1fece8eb991245c3c8",
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
    name = "systemd-libs-0__252-59.el9.aarch64",
    sha256 = "f048617c01d8def60d37694132723baf164d78a4aff9737925e1be3d2159a6a4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-libs-252-59.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f048617c01d8def60d37694132723baf164d78a4aff9737925e1be3d2159a6a4",
    ],
)

rpm(
    name = "systemd-libs-0__252-59.el9.s390x",
    sha256 = "0093a56ddb3e78024ae90daead255570b3e0463d35704ce69f73e03d1831dacd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-libs-252-59.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/0093a56ddb3e78024ae90daead255570b3e0463d35704ce69f73e03d1831dacd",
    ],
)

rpm(
    name = "systemd-libs-0__252-59.el9.x86_64",
    sha256 = "335d9aee756aaf623758098fdbec6868a4c557d5f7ebaedbbaf7d93e00fe0303",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-libs-252-59.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/335d9aee756aaf623758098fdbec6868a4c557d5f7ebaedbbaf7d93e00fe0303",
    ],
)

rpm(
    name = "systemd-pam-0__252-59.el9.aarch64",
    sha256 = "61c4c7a0d7e9abdbbd206f276cd6a2b7d8f193db4cb5defff0ef3596fab4a24e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-pam-252-59.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/61c4c7a0d7e9abdbbd206f276cd6a2b7d8f193db4cb5defff0ef3596fab4a24e",
    ],
)

rpm(
    name = "systemd-pam-0__252-59.el9.s390x",
    sha256 = "8debf0a6e318d897719de8650ef3632856502eb678ce01477de7e923fcab503b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-pam-252-59.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/8debf0a6e318d897719de8650ef3632856502eb678ce01477de7e923fcab503b",
    ],
)

rpm(
    name = "systemd-pam-0__252-59.el9.x86_64",
    sha256 = "86529840ec93fffe908358adb4fa44004f7be851960c4cc84f19b6b4ff300112",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-pam-252-59.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/86529840ec93fffe908358adb4fa44004f7be851960c4cc84f19b6b4ff300112",
    ],
)

rpm(
    name = "systemd-rpm-macros-0__252-59.el9.aarch64",
    sha256 = "29ec8fd209f5fd8a8b99481d4d625ef3f8c2a924c08e4b80493f869d4ea58f6d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-rpm-macros-252-59.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/29ec8fd209f5fd8a8b99481d4d625ef3f8c2a924c08e4b80493f869d4ea58f6d",
    ],
)

rpm(
    name = "systemd-rpm-macros-0__252-59.el9.s390x",
    sha256 = "29ec8fd209f5fd8a8b99481d4d625ef3f8c2a924c08e4b80493f869d4ea58f6d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-rpm-macros-252-59.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/29ec8fd209f5fd8a8b99481d4d625ef3f8c2a924c08e4b80493f869d4ea58f6d",
    ],
)

rpm(
    name = "systemd-rpm-macros-0__252-59.el9.x86_64",
    sha256 = "29ec8fd209f5fd8a8b99481d4d625ef3f8c2a924c08e4b80493f869d4ea58f6d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-rpm-macros-252-59.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/29ec8fd209f5fd8a8b99481d4d625ef3f8c2a924c08e4b80493f869d4ea58f6d",
    ],
)

rpm(
    name = "tar-2__1.34-7.el9.aarch64",
    sha256 = "e3ee12a44a68c84627e43c2512ad8904a4778a82b274d0e8147ca46645f4a1fb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/tar-1.34-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/e3ee12a44a68c84627e43c2512ad8904a4778a82b274d0e8147ca46645f4a1fb",
    ],
)

rpm(
    name = "tar-2__1.34-7.el9.s390x",
    sha256 = "304bca9dd546a39a59bd50b8ec5fb3f42898138f92e49945be09cab503cdf1a2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/tar-1.34-7.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/304bca9dd546a39a59bd50b8ec5fb3f42898138f92e49945be09cab503cdf1a2",
    ],
)

rpm(
    name = "tar-2__1.34-7.el9.x86_64",
    sha256 = "b90b0e6f70433d3935b1dd45a3c10a40768950b5c9121545034179bd7b55159f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/tar-1.34-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/b90b0e6f70433d3935b1dd45a3c10a40768950b5c9121545034179bd7b55159f",
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
    name = "tzdata-0__2025b-2.el9.aarch64",
    sha256 = "45afb63925e75626cdaf62a3fd81cdc85c38a97f9bc1460d5f8c69ff1c619c57",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/tzdata-2025b-2.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/45afb63925e75626cdaf62a3fd81cdc85c38a97f9bc1460d5f8c69ff1c619c57",
    ],
)

rpm(
    name = "tzdata-0__2025b-2.el9.s390x",
    sha256 = "45afb63925e75626cdaf62a3fd81cdc85c38a97f9bc1460d5f8c69ff1c619c57",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/tzdata-2025b-2.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/45afb63925e75626cdaf62a3fd81cdc85c38a97f9bc1460d5f8c69ff1c619c57",
    ],
)

rpm(
    name = "tzdata-0__2025b-2.el9.x86_64",
    sha256 = "45afb63925e75626cdaf62a3fd81cdc85c38a97f9bc1460d5f8c69ff1c619c57",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/tzdata-2025b-2.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/45afb63925e75626cdaf62a3fd81cdc85c38a97f9bc1460d5f8c69ff1c619c57",
    ],
)

rpm(
    name = "util-linux-0__2.37.4-21.el9.aarch64",
    sha256 = "434fe9c8a283246524ced5b6637ef2f95ad0e1ab20cbeabc582592b30f69b0b8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/util-linux-2.37.4-21.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/434fe9c8a283246524ced5b6637ef2f95ad0e1ab20cbeabc582592b30f69b0b8",
    ],
)

rpm(
    name = "util-linux-0__2.37.4-21.el9.s390x",
    sha256 = "bc5501f5a26586828a30cc97870f2f907d44c6107d7974c620ff920718b4d6bd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/util-linux-2.37.4-21.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/bc5501f5a26586828a30cc97870f2f907d44c6107d7974c620ff920718b4d6bd",
    ],
)

rpm(
    name = "util-linux-0__2.37.4-21.el9.x86_64",
    sha256 = "77f5aa59c85c1231bde7f64a7e348bb7b4675a04e385e219275abbd748037075",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/util-linux-2.37.4-21.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/77f5aa59c85c1231bde7f64a7e348bb7b4675a04e385e219275abbd748037075",
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
    name = "util-linux-core-0__2.37.4-21.el9.aarch64",
    sha256 = "1066ec56c02d69030fb95b0749f8d21e7748449ffa95bba4fb0e12ec938ca1d8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/util-linux-core-2.37.4-21.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/1066ec56c02d69030fb95b0749f8d21e7748449ffa95bba4fb0e12ec938ca1d8",
    ],
)

rpm(
    name = "util-linux-core-0__2.37.4-21.el9.s390x",
    sha256 = "62553c85f62156441de1b4e5da924182b3bafabef47fe803539396fd44fadb43",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/util-linux-core-2.37.4-21.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/62553c85f62156441de1b4e5da924182b3bafabef47fe803539396fd44fadb43",
    ],
)

rpm(
    name = "util-linux-core-0__2.37.4-21.el9.x86_64",
    sha256 = "1858fbea657a9edce414fd98b8260b37ef521769f06830fccc7831094ec04154",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/util-linux-core-2.37.4-21.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/1858fbea657a9edce414fd98b8260b37ef521769f06830fccc7831094ec04154",
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
    name = "vim-minimal-2__8.2.2637-23.el9.aarch64",
    sha256 = "68ab78b16338b49d9669b484c38ee5c09dd4b2901d5aba5d818152b2af4faf2e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/vim-minimal-8.2.2637-23.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/68ab78b16338b49d9669b484c38ee5c09dd4b2901d5aba5d818152b2af4faf2e",
    ],
)

rpm(
    name = "vim-minimal-2__8.2.2637-23.el9.s390x",
    sha256 = "331adbd8d706c421c9db79304486c9da17d399f5cb592ae8402bc8ab8763e51b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/vim-minimal-8.2.2637-23.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/331adbd8d706c421c9db79304486c9da17d399f5cb592ae8402bc8ab8763e51b",
    ],
)

rpm(
    name = "vim-minimal-2__8.2.2637-23.el9.x86_64",
    sha256 = "2c4291f0fbf23f507cd74c53fdd00ab740cd41b445024a074c4db7ec15421903",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/vim-minimal-8.2.2637-23.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/2c4291f0fbf23f507cd74c53fdd00ab740cd41b445024a074c4db7ec15421903",
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
