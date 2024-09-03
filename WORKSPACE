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
    sha256 = "80a98277ad1311dacd837f9b16db62887702e9f1d1c4c9f796d0121a46c8e184",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.46.0/rules_go-v0.46.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.46.0/rules_go-v0.46.0.zip",
        "https://storage.googleapis.com/builddeps/80a98277ad1311dacd837f9b16db62887702e9f1d1c4c9f796d0121a46c8e184",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(
    go_version = "host",
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
    sha256 = "d3fa66a39028e97d76f9e2db8f1b0c11c099e8e01bf363a923074784e451f809",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.33.0/bazel-gazelle-v0.33.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.33.0/bazel-gazelle-v0.33.0.tar.gz",
        "https://storage.googleapis.com/builddeps/d3fa66a39028e97d76f9e2db8f1b0c11c099e8e01bf363a923074784e451f809",
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
    sha256 = "6a2af09c6a598a3c4e4fec9af78334fbec2b3c16473f4e2c692fe2e567dc6f56",
    strip_prefix = "bazeldnf-0.5.1",
    urls = [
        "https://github.com/rmohr/bazeldnf/archive/v0.5.1.tar.gz",
        "https://storage.googleapis.com/builddeps/6a2af09c6a598a3c4e4fec9af78334fbec2b3c16473f4e2c692fe2e567dc6f56",
    ],
)

load("@bazeldnf//:deps.bzl", "bazeldnf_dependencies", "rpm")

bazeldnf_dependencies()

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
    architecture = "arm64",
    registry = "quay.io",
    repository = "centos/centos",
    tag = "stream9",
)

# Pull base image container registry
container_pull(
    name = "registry",
    digest = "sha256:5c98b00f91e8daed324cb680661e9d647f09d825778493ffb2618ff36bec2a9e",
    registry = "quay.io",
    repository = "libpod/registry",
    tag = "2.8",
)

container_pull(
    name = "registry-aarch64",
    digest = "sha256:f4e803a2d37afca6d059961f28d73c57cbe6fdb3a44ba6ae7ad463811f43b81c",
    registry = "quay.io",
    repository = "libpod/registry",
    tag = "2.8",
)

container_pull(
    name = "registry-s390x",
    digest = "sha256:7e1926b82e5b862a633b83acf8f456e1619be720aff346e1b634db2f843082b7",
    registry = "quay.io",
    repository = "libpod/registry",
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

#imageio rpms and dependencies
http_file(
    name = "ovirt-imageio-client",
    sha256 = "4447b2e6c659f0b486f8db82b415eaa065adb09092b00308eade30632be0c4bf",
    urls = ["https://storage.googleapis.com/builddeps/4447b2e6c659f0b486f8db82b415eaa065adb09092b00308eade30632be0c4bf"],
)

http_file(
    name = "ovirt-imageio-client-aarch64",
    sha256 = "ab2cdec494c6ed22cb718779bf1445d3ede79c051337e506b3c05c77c28f5b2d",
    urls = ["https://storage.googleapis.com/builddeps/ab2cdec494c6ed22cb718779bf1445d3ede79c051337e506b3c05c77c28f5b2d"],
)

http_file(
    name = "ovirt-imageio-common",
    sha256 = "d6562eb701afcbde7ab1f14e1cc5f3a8be3ecc7fa25b9f3b831e342c3e14bc2a",
    urls = ["https://storage.googleapis.com/builddeps/d6562eb701afcbde7ab1f14e1cc5f3a8be3ecc7fa25b9f3b831e342c3e14bc2a"],
)

http_file(
    name = "ovirt-imageio-common-aarch64",
    sha256 = "f8a9273463657244fdf34fef0077f2c79f1216891439472df0543eb5133d7922",
    urls = ["https://storage.googleapis.com/builddeps/f8a9273463657244fdf34fef0077f2c79f1216891439472df0543eb5133d7922"],
)

http_file(
    name = "ovirt-imageio-daemon",
    sha256 = "e0df3d43109769d2745a0d2befc05db8961b7de770047adf8ad60469d6e430f0",
    urls = ["https://storage.googleapis.com/builddeps/e0df3d43109769d2745a0d2befc05db8961b7de770047adf8ad60469d6e430f0"],
)

http_file(
    name = "ovirt-imageio-daemon-aarch64",
    sha256 = "5a6697a4fd9c8d52a8a9ead8a4281b3d208df221e0d87f4377e7a3f6a3a1608d",
    urls = ["https://storage.googleapis.com/builddeps/5a6697a4fd9c8d52a8a9ead8a4281b3d208df221e0d87f4377e7a3f6a3a1608d"],
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
    name = "alternatives-0__1.24-1.el9.aarch64",
    sha256 = "a9bba5fd3731426733609e996881cddb0775e979091fab91a3878178a63c7656",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/alternatives-1.24-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/a9bba5fd3731426733609e996881cddb0775e979091fab91a3878178a63c7656",
    ],
)

rpm(
    name = "alternatives-0__1.24-1.el9.s390x",
    sha256 = "009eeff2a85e9682beb3d576e2a2359c83efa71371464e6021e9b4e92f32af36",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/alternatives-1.24-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/009eeff2a85e9682beb3d576e2a2359c83efa71371464e6021e9b4e92f32af36",
    ],
)

rpm(
    name = "alternatives-0__1.24-1.el9.x86_64",
    sha256 = "b58e7ea30c27ecb321d9a279b95b62aef59d92173714fce859bfb359ee231ff3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/alternatives-1.24-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/b58e7ea30c27ecb321d9a279b95b62aef59d92173714fce859bfb359ee231ff3",
    ],
)

rpm(
    name = "audit-libs-0__3.1.5-1.el9.aarch64",
    sha256 = "ce97ff90c24105c48d6ef29b0643021f366048f10c79c7f3d81e3f0f9483d5e6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/audit-libs-3.1.5-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ce97ff90c24105c48d6ef29b0643021f366048f10c79c7f3d81e3f0f9483d5e6",
    ],
)

rpm(
    name = "audit-libs-0__3.1.5-1.el9.s390x",
    sha256 = "090ef1e4057d3235a050ad72728f40752faa6958a7f3ee6ebd0cd43e5f97d026",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/audit-libs-3.1.5-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/090ef1e4057d3235a050ad72728f40752faa6958a7f3ee6ebd0cd43e5f97d026",
    ],
)

rpm(
    name = "audit-libs-0__3.1.5-1.el9.x86_64",
    sha256 = "e1998c3847956ad86d846f8b857e5382897ef2f444b4a2ef8e82a0cb8b1aa1ad",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/audit-libs-3.1.5-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e1998c3847956ad86d846f8b857e5382897ef2f444b4a2ef8e82a0cb8b1aa1ad",
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
    name = "buildah-2__1.37.2-1.el9.aarch64",
    sha256 = "aa3556b21b45010a374f2cea9a6783d952a47f9f4b6030de1609ef602add0717",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/buildah-1.37.2-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/aa3556b21b45010a374f2cea9a6783d952a47f9f4b6030de1609ef602add0717",
    ],
)

rpm(
    name = "buildah-2__1.37.2-1.el9.s390x",
    sha256 = "9e160908764b353923bd1ac84cdb843a779f75b9a28ab3cc5218c4b61b7efded",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/buildah-1.37.2-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/9e160908764b353923bd1ac84cdb843a779f75b9a28ab3cc5218c4b61b7efded",
    ],
)

rpm(
    name = "buildah-2__1.37.2-1.el9.x86_64",
    sha256 = "bd9ca62ee4deb457c71f7d369a502370e6efc98013130582c7f299b6557f5bd7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/buildah-1.37.2-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/bd9ca62ee4deb457c71f7d369a502370e6efc98013130582c7f299b6557f5bd7",
    ],
)

rpm(
    name = "bzip2-libs-0__1.0.8-8.el9.aarch64",
    sha256 = "6c20f6f13c274fa2487f95f1e3dddcee9b931ce222abebd2f1d9b3f7eb69fcde",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/bzip2-libs-1.0.8-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/6c20f6f13c274fa2487f95f1e3dddcee9b931ce222abebd2f1d9b3f7eb69fcde",
    ],
)

rpm(
    name = "bzip2-libs-0__1.0.8-8.el9.s390x",
    sha256 = "187c9275d53ddd209339a5ae7ae7af4d2c80647a054197f7abe39c020a66262d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/bzip2-libs-1.0.8-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/187c9275d53ddd209339a5ae7ae7af4d2c80647a054197f7abe39c020a66262d",
    ],
)

rpm(
    name = "bzip2-libs-0__1.0.8-8.el9.x86_64",
    sha256 = "fabd6b5c065c2b9d4a8d39a938ae577d801de2ddc73c8cdf6f7803db29c28d0a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/bzip2-libs-1.0.8-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/fabd6b5c065c2b9d4a8d39a938ae577d801de2ddc73c8cdf6f7803db29c28d0a",
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
    name = "ca-certificates-0__2024.2.69_v8.0.303-91.4.el9.aarch64",
    sha256 = "d18c1b9763c22dc93da804f96ad3d92b3157195c9eff6e923c33e9011df3e246",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/ca-certificates-2024.2.69_v8.0.303-91.4.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/d18c1b9763c22dc93da804f96ad3d92b3157195c9eff6e923c33e9011df3e246",
    ],
)

rpm(
    name = "ca-certificates-0__2024.2.69_v8.0.303-91.4.el9.s390x",
    sha256 = "d18c1b9763c22dc93da804f96ad3d92b3157195c9eff6e923c33e9011df3e246",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/ca-certificates-2024.2.69_v8.0.303-91.4.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/d18c1b9763c22dc93da804f96ad3d92b3157195c9eff6e923c33e9011df3e246",
    ],
)

rpm(
    name = "ca-certificates-0__2024.2.69_v8.0.303-91.4.el9.x86_64",
    sha256 = "d18c1b9763c22dc93da804f96ad3d92b3157195c9eff6e923c33e9011df3e246",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ca-certificates-2024.2.69_v8.0.303-91.4.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/d18c1b9763c22dc93da804f96ad3d92b3157195c9eff6e923c33e9011df3e246",
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
    name = "centos-gpg-keys-0__9.0-26.el9.aarch64",
    sha256 = "8d601d9f96356a200ad6ed8e5cb49bbac4aa3c4b762d10a23e11311daa5711ca",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-gpg-keys-9.0-26.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/8d601d9f96356a200ad6ed8e5cb49bbac4aa3c4b762d10a23e11311daa5711ca",
    ],
)

rpm(
    name = "centos-gpg-keys-0__9.0-26.el9.s390x",
    sha256 = "8d601d9f96356a200ad6ed8e5cb49bbac4aa3c4b762d10a23e11311daa5711ca",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/centos-gpg-keys-9.0-26.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/8d601d9f96356a200ad6ed8e5cb49bbac4aa3c4b762d10a23e11311daa5711ca",
    ],
)

rpm(
    name = "centos-gpg-keys-0__9.0-26.el9.x86_64",
    sha256 = "8d601d9f96356a200ad6ed8e5cb49bbac4aa3c4b762d10a23e11311daa5711ca",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-gpg-keys-9.0-26.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/8d601d9f96356a200ad6ed8e5cb49bbac4aa3c4b762d10a23e11311daa5711ca",
    ],
)

rpm(
    name = "centos-logos-httpd-0__90.8-1.el9.aarch64",
    sha256 = "97173a26ab3315860acfc806a4547ac967e5bf1d19246de485595aaf8065d13c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/centos-logos-httpd-90.8-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/97173a26ab3315860acfc806a4547ac967e5bf1d19246de485595aaf8065d13c",
    ],
)

rpm(
    name = "centos-logos-httpd-0__90.8-1.el9.s390x",
    sha256 = "97173a26ab3315860acfc806a4547ac967e5bf1d19246de485595aaf8065d13c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/centos-logos-httpd-90.8-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/97173a26ab3315860acfc806a4547ac967e5bf1d19246de485595aaf8065d13c",
    ],
)

rpm(
    name = "centos-logos-httpd-0__90.8-1.el9.x86_64",
    sha256 = "97173a26ab3315860acfc806a4547ac967e5bf1d19246de485595aaf8065d13c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/centos-logos-httpd-90.8-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/97173a26ab3315860acfc806a4547ac967e5bf1d19246de485595aaf8065d13c",
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
    name = "centos-stream-release-0__9.0-26.el9.aarch64",
    sha256 = "3d60dc8ed86717f68394fc7468b8024557c43ac2ad97b8e40911d056cd6d64d3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-stream-release-9.0-26.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/3d60dc8ed86717f68394fc7468b8024557c43ac2ad97b8e40911d056cd6d64d3",
    ],
)

rpm(
    name = "centos-stream-release-0__9.0-26.el9.s390x",
    sha256 = "3d60dc8ed86717f68394fc7468b8024557c43ac2ad97b8e40911d056cd6d64d3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/centos-stream-release-9.0-26.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/3d60dc8ed86717f68394fc7468b8024557c43ac2ad97b8e40911d056cd6d64d3",
    ],
)

rpm(
    name = "centos-stream-release-0__9.0-26.el9.x86_64",
    sha256 = "3d60dc8ed86717f68394fc7468b8024557c43ac2ad97b8e40911d056cd6d64d3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-stream-release-9.0-26.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/3d60dc8ed86717f68394fc7468b8024557c43ac2ad97b8e40911d056cd6d64d3",
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
    name = "centos-stream-repos-0__9.0-26.el9.aarch64",
    sha256 = "eb3b55a5cf0e1a93a91cd2d39035bd1754b46f69ff3d062b3331e765b2345035",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-stream-repos-9.0-26.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/eb3b55a5cf0e1a93a91cd2d39035bd1754b46f69ff3d062b3331e765b2345035",
    ],
)

rpm(
    name = "centos-stream-repos-0__9.0-26.el9.s390x",
    sha256 = "eb3b55a5cf0e1a93a91cd2d39035bd1754b46f69ff3d062b3331e765b2345035",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/centos-stream-repos-9.0-26.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/eb3b55a5cf0e1a93a91cd2d39035bd1754b46f69ff3d062b3331e765b2345035",
    ],
)

rpm(
    name = "centos-stream-repos-0__9.0-26.el9.x86_64",
    sha256 = "eb3b55a5cf0e1a93a91cd2d39035bd1754b46f69ff3d062b3331e765b2345035",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-stream-repos-9.0-26.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/eb3b55a5cf0e1a93a91cd2d39035bd1754b46f69ff3d062b3331e765b2345035",
    ],
)

rpm(
    name = "containers-common-2__1-91.el9.aarch64",
    sha256 = "e9802e5400e614c0ac41b3d6d9b7e4ecc8ea02aef0e7b2be064e93ed68b4da02",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/containers-common-1-91.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/e9802e5400e614c0ac41b3d6d9b7e4ecc8ea02aef0e7b2be064e93ed68b4da02",
    ],
)

rpm(
    name = "containers-common-2__1-91.el9.s390x",
    sha256 = "01c219b60f01e3c1eb9e35ccd9d8cc189d9433b0a84a9b35b1b2041ee75a8cbf",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/containers-common-1-91.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/01c219b60f01e3c1eb9e35ccd9d8cc189d9433b0a84a9b35b1b2041ee75a8cbf",
    ],
)

rpm(
    name = "containers-common-2__1-91.el9.x86_64",
    sha256 = "d8ea0cdba33f4cdb7b9e0fa34cb5c61323146ffaa1e3ea311bbde06d93c9d265",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/containers-common-1-91.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/d8ea0cdba33f4cdb7b9e0fa34cb5c61323146ffaa1e3ea311bbde06d93c9d265",
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
    name = "coreutils-single-0__8.32-36.el9.aarch64",
    sha256 = "ce8631f44b1f486900e4572f03317d351050b85abd0ec15b098f46ca47b039dd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/coreutils-single-8.32-36.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ce8631f44b1f486900e4572f03317d351050b85abd0ec15b098f46ca47b039dd",
    ],
)

rpm(
    name = "coreutils-single-0__8.32-36.el9.s390x",
    sha256 = "77705a4b37599f291c31c4160659e35c19546b8b56f1fae372f35cb25f97dabf",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/coreutils-single-8.32-36.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/77705a4b37599f291c31c4160659e35c19546b8b56f1fae372f35cb25f97dabf",
    ],
)

rpm(
    name = "coreutils-single-0__8.32-36.el9.x86_64",
    sha256 = "2d42cef83d48e0827ebbfdffe04f39d2f1148bb8228db293b9271579e9f13308",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/coreutils-single-8.32-36.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/2d42cef83d48e0827ebbfdffe04f39d2f1148bb8228db293b9271579e9f13308",
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
    name = "crun-0__1.16.1-1.el9.aarch64",
    sha256 = "f713f2ae1ef06b06a4a871e0df08f113f7b0d889bdd4cc7ceb447fcab494b4e7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/crun-1.16.1-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f713f2ae1ef06b06a4a871e0df08f113f7b0d889bdd4cc7ceb447fcab494b4e7",
    ],
)

rpm(
    name = "crun-0__1.16.1-1.el9.s390x",
    sha256 = "527a0df933b6324ecec08338bd6275be6cf6217e5225724dd22afedff11da7e5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/crun-1.16.1-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/527a0df933b6324ecec08338bd6275be6cf6217e5225724dd22afedff11da7e5",
    ],
)

rpm(
    name = "crun-0__1.16.1-1.el9.x86_64",
    sha256 = "2375a3674fb8e80e14b4722926d71770df1e65bfb5c79f25fc55d74c6fa2331a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/crun-1.16.1-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/2375a3674fb8e80e14b4722926d71770df1e65bfb5c79f25fc55d74c6fa2331a",
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
    name = "crypto-policies-0__20240822-1.gitbaf3e06.el9.aarch64",
    sha256 = "b27b0ad9f9ecb77cabfd03b7e2450bb4ef9413ee0cd7be15bfa0ecf6f2d83b96",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/crypto-policies-20240822-1.gitbaf3e06.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/b27b0ad9f9ecb77cabfd03b7e2450bb4ef9413ee0cd7be15bfa0ecf6f2d83b96",
    ],
)

rpm(
    name = "crypto-policies-0__20240822-1.gitbaf3e06.el9.s390x",
    sha256 = "b27b0ad9f9ecb77cabfd03b7e2450bb4ef9413ee0cd7be15bfa0ecf6f2d83b96",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/crypto-policies-20240822-1.gitbaf3e06.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/b27b0ad9f9ecb77cabfd03b7e2450bb4ef9413ee0cd7be15bfa0ecf6f2d83b96",
    ],
)

rpm(
    name = "crypto-policies-0__20240822-1.gitbaf3e06.el9.x86_64",
    sha256 = "b27b0ad9f9ecb77cabfd03b7e2450bb4ef9413ee0cd7be15bfa0ecf6f2d83b96",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/crypto-policies-20240822-1.gitbaf3e06.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/b27b0ad9f9ecb77cabfd03b7e2450bb4ef9413ee0cd7be15bfa0ecf6f2d83b96",
    ],
)

rpm(
    name = "crypto-policies-scripts-0__20240822-1.gitbaf3e06.el9.aarch64",
    sha256 = "c9f4377cb80598a795ec51f48e6e4f7a9808b4423ad348a5dfd009fdcbc325c7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/crypto-policies-scripts-20240822-1.gitbaf3e06.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/c9f4377cb80598a795ec51f48e6e4f7a9808b4423ad348a5dfd009fdcbc325c7",
    ],
)

rpm(
    name = "crypto-policies-scripts-0__20240822-1.gitbaf3e06.el9.s390x",
    sha256 = "c9f4377cb80598a795ec51f48e6e4f7a9808b4423ad348a5dfd009fdcbc325c7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/crypto-policies-scripts-20240822-1.gitbaf3e06.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/c9f4377cb80598a795ec51f48e6e4f7a9808b4423ad348a5dfd009fdcbc325c7",
    ],
)

rpm(
    name = "crypto-policies-scripts-0__20240822-1.gitbaf3e06.el9.x86_64",
    sha256 = "c9f4377cb80598a795ec51f48e6e4f7a9808b4423ad348a5dfd009fdcbc325c7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/crypto-policies-scripts-20240822-1.gitbaf3e06.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/c9f4377cb80598a795ec51f48e6e4f7a9808b4423ad348a5dfd009fdcbc325c7",
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
    name = "curl-0__7.76.1-31.el9.aarch64",
    sha256 = "342e768bc4a54dbd45575b607d3e71c3b3011428a7bce0a3074300526f46f51a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/curl-7.76.1-31.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/342e768bc4a54dbd45575b607d3e71c3b3011428a7bce0a3074300526f46f51a",
    ],
)

rpm(
    name = "curl-0__7.76.1-31.el9.s390x",
    sha256 = "ee992346a68e550e38c68144df43d75def522f8d51b8e5aca3eb17d0c160e45a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/curl-7.76.1-31.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/ee992346a68e550e38c68144df43d75def522f8d51b8e5aca3eb17d0c160e45a",
    ],
)

rpm(
    name = "curl-0__7.76.1-31.el9.x86_64",
    sha256 = "7884a48b4198a915ec412bbfd32bedee955f11119ac1c55b4afa82dd269d22dd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/curl-7.76.1-31.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/7884a48b4198a915ec412bbfd32bedee955f11119ac1c55b4afa82dd269d22dd",
    ],
)

rpm(
    name = "curl-minimal-0__7.76.1-31.el9.aarch64",
    sha256 = "7cbda5bca46c13e80bd28391e998b8695e93fb450c40c99ffb52e3b3a74a2ac2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/curl-minimal-7.76.1-31.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/7cbda5bca46c13e80bd28391e998b8695e93fb450c40c99ffb52e3b3a74a2ac2",
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
    name = "diffutils-0__3.7-12.el9.aarch64",
    sha256 = "4fea2be2558981a55a569cc7b93f17afce86bba830ebce32a0aa320e4759293e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/diffutils-3.7-12.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/4fea2be2558981a55a569cc7b93f17afce86bba830ebce32a0aa320e4759293e",
    ],
)

rpm(
    name = "diffutils-0__3.7-12.el9.s390x",
    sha256 = "e0f62f72c6d24e0507fa16c23bb74ece2704aabfb902c3649c57dad090f0c1ae",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/diffutils-3.7-12.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/e0f62f72c6d24e0507fa16c23bb74ece2704aabfb902c3649c57dad090f0c1ae",
    ],
)

rpm(
    name = "diffutils-0__3.7-12.el9.x86_64",
    sha256 = "fdebefc46badf2e700e00582041a0e5f5183dd4fdc04badfe47c91f030cea0ce",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/diffutils-3.7-12.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/fdebefc46badf2e700e00582041a0e5f5183dd4fdc04badfe47c91f030cea0ce",
    ],
)

rpm(
    name = "expat-0__2.5.0-2.el9.aarch64",
    sha256 = "cc115aa6a973d7d7c5516c45a961a96ed149bf37349e51ab5d0d9a87b935b890",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/expat-2.5.0-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/cc115aa6a973d7d7c5516c45a961a96ed149bf37349e51ab5d0d9a87b935b890",
    ],
)

rpm(
    name = "expat-0__2.5.0-2.el9.s390x",
    sha256 = "30a6753dc782e0044c547ae9da6a3be7db12e23e147508dffdeea7180e3ffe02",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/expat-2.5.0-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/30a6753dc782e0044c547ae9da6a3be7db12e23e147508dffdeea7180e3ffe02",
    ],
)

rpm(
    name = "expat-0__2.5.0-2.el9.x86_64",
    sha256 = "00580404a4c3f59a32d7b9ae513ff48d7bbaa65d3a9bf9193dc33c9668dae6b4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/expat-2.5.0-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/00580404a4c3f59a32d7b9ae513ff48d7bbaa65d3a9bf9193dc33c9668dae6b4",
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
    name = "glib2-0__2.68.4-15.el9.aarch64",
    sha256 = "a2baecdb746f3312a5b37d4bc139182195b31eb4a1cb16b9b84db00a1e640042",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glib2-2.68.4-15.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/a2baecdb746f3312a5b37d4bc139182195b31eb4a1cb16b9b84db00a1e640042",
    ],
)

rpm(
    name = "glib2-0__2.68.4-15.el9.s390x",
    sha256 = "34a706adcd651397a5f6681c4cb4973aac48df23cf00c9457946d1f264376960",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glib2-2.68.4-15.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/34a706adcd651397a5f6681c4cb4973aac48df23cf00c9457946d1f264376960",
    ],
)

rpm(
    name = "glib2-0__2.68.4-15.el9.x86_64",
    sha256 = "85f10ac062e7c1dc1c4becd0f5287f7e5e0f17267e139a59cfb5eabf4632713e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glib2-2.68.4-15.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/85f10ac062e7c1dc1c4becd0f5287f7e5e0f17267e139a59cfb5eabf4632713e",
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
    name = "glibc-0__2.34-120.el9.aarch64",
    sha256 = "2c8abf05d9cc2fca4138dff784934e8adcb303c7f2517e0afdaef2215b3bd9e1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-2.34-120.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/2c8abf05d9cc2fca4138dff784934e8adcb303c7f2517e0afdaef2215b3bd9e1",
    ],
)

rpm(
    name = "glibc-0__2.34-120.el9.s390x",
    sha256 = "8cceb8b41ffc8653c0c361b0b075b307c9e7feba65c5e3fa724d34afc210da53",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glibc-2.34-120.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/8cceb8b41ffc8653c0c361b0b075b307c9e7feba65c5e3fa724d34afc210da53",
    ],
)

rpm(
    name = "glibc-0__2.34-120.el9.x86_64",
    sha256 = "5a8784660b6fdcb20a6bbd4203f306495bf1d8b976b0a59d19dca18f0589a5a1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-2.34-120.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/5a8784660b6fdcb20a6bbd4203f306495bf1d8b976b0a59d19dca18f0589a5a1",
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
    name = "glibc-common-0__2.34-120.el9.aarch64",
    sha256 = "c2b9cef59f34fe457eeeca1e66c8849af4153ea9c3687a09f96ac80c8e177fd4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-common-2.34-120.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c2b9cef59f34fe457eeeca1e66c8849af4153ea9c3687a09f96ac80c8e177fd4",
    ],
)

rpm(
    name = "glibc-common-0__2.34-120.el9.s390x",
    sha256 = "832d2698ee7451bad7f2dbd193acc362c3d52ecf4c6e0865e54c8402689513e0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glibc-common-2.34-120.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/832d2698ee7451bad7f2dbd193acc362c3d52ecf4c6e0865e54c8402689513e0",
    ],
)

rpm(
    name = "glibc-common-0__2.34-120.el9.x86_64",
    sha256 = "b72de8b46f0a7df5b161823027581c4aae642283c3fda30954dd06fccda27188",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-common-2.34-120.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/b72de8b46f0a7df5b161823027581c4aae642283c3fda30954dd06fccda27188",
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
    name = "glibc-minimal-langpack-0__2.34-120.el9.aarch64",
    sha256 = "3b2a5cbd5ca49d6ae7b5bbfbb610e3fa20c9d7c00f18d07ef529506e050eca46",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-minimal-langpack-2.34-120.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/3b2a5cbd5ca49d6ae7b5bbfbb610e3fa20c9d7c00f18d07ef529506e050eca46",
    ],
)

rpm(
    name = "glibc-minimal-langpack-0__2.34-120.el9.s390x",
    sha256 = "374644269bb52306fc5fb8430ece556d307adf62c5662133bda5777dfee6d105",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glibc-minimal-langpack-2.34-120.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/374644269bb52306fc5fb8430ece556d307adf62c5662133bda5777dfee6d105",
    ],
)

rpm(
    name = "glibc-minimal-langpack-0__2.34-120.el9.x86_64",
    sha256 = "d345bd69726071bbb5e68734fc7d93dc43c399181e1442aaafd7ae193632d28b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-minimal-langpack-2.34-120.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/d345bd69726071bbb5e68734fc7d93dc43c399181e1442aaafd7ae193632d28b",
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
    name = "gnutls-0__3.8.3-4.el9.aarch64",
    sha256 = "c7c658c2f2364f4fcbc056f3059c3a4f8a8fa5db3a34a56bbab8386e9f1a9ac5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gnutls-3.8.3-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c7c658c2f2364f4fcbc056f3059c3a4f8a8fa5db3a34a56bbab8386e9f1a9ac5",
    ],
)

rpm(
    name = "gnutls-0__3.8.3-4.el9.s390x",
    sha256 = "f71b6727e720d44781702bb37815cddbe7f0aab173174bbc7f555d88ca00160e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gnutls-3.8.3-4.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/f71b6727e720d44781702bb37815cddbe7f0aab173174bbc7f555d88ca00160e",
    ],
)

rpm(
    name = "gnutls-0__3.8.3-4.el9.x86_64",
    sha256 = "91e1e46e6f315445e715184237a69f4152359efa1a9ae54cc0524b9616d0741f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gnutls-3.8.3-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/91e1e46e6f315445e715184237a69f4152359efa1a9ae54cc0524b9616d0741f",
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
    name = "iptables-libs-0__1.8.10-5.el9.aarch64",
    sha256 = "0a1a62e87beefb172561f8c399ffd227a2200d9e75da6ee34e573e5535b21782",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/iptables-libs-1.8.10-5.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/0a1a62e87beefb172561f8c399ffd227a2200d9e75da6ee34e573e5535b21782",
    ],
)

rpm(
    name = "iptables-libs-0__1.8.10-5.el9.s390x",
    sha256 = "f70904187dba41332b65d6678d680ed6761c811c2c620662d6ee185716951f93",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/iptables-libs-1.8.10-5.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/f70904187dba41332b65d6678d680ed6761c811c2c620662d6ee185716951f93",
    ],
)

rpm(
    name = "iptables-libs-0__1.8.10-5.el9.x86_64",
    sha256 = "36823d15bd684acf2df31039914c186cb513bf2b9ad08603d6890ce785b96661",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/iptables-libs-1.8.10-5.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/36823d15bd684acf2df31039914c186cb513bf2b9ad08603d6890ce785b96661",
    ],
)

rpm(
    name = "iptables-nft-0__1.8.10-5.el9.aarch64",
    sha256 = "ab06da51b619802d55d4d9937e3a4ca45fe467c0bbccd6eecae96d650bf4d7ed",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/iptables-nft-1.8.10-5.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ab06da51b619802d55d4d9937e3a4ca45fe467c0bbccd6eecae96d650bf4d7ed",
    ],
)

rpm(
    name = "iptables-nft-0__1.8.10-5.el9.s390x",
    sha256 = "4b39fa12fca7bb3855f7e806a728307cc5333b7a33b5f7a1920fcf28d376079a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/iptables-nft-1.8.10-5.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/4b39fa12fca7bb3855f7e806a728307cc5333b7a33b5f7a1920fcf28d376079a",
    ],
)

rpm(
    name = "iptables-nft-0__1.8.10-5.el9.x86_64",
    sha256 = "680c661d4cbf577cbc9dbaa6e979bed81583abf66a41c65570f632f2380382c7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/iptables-nft-1.8.10-5.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/680c661d4cbf577cbc9dbaa6e979bed81583abf66a41c65570f632f2380382c7",
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
    name = "kmod-libs-0__28-10.el9.aarch64",
    sha256 = "5da40af25f9af3e6ce1ff8dd751da596073dd0adf15dcf44c393330ff0346355",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/kmod-libs-28-10.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/5da40af25f9af3e6ce1ff8dd751da596073dd0adf15dcf44c393330ff0346355",
    ],
)

rpm(
    name = "kmod-libs-0__28-10.el9.s390x",
    sha256 = "7011810fca95064c8d78e55071716ec1dd5bc7b9836f662c195a282f4f4e5d0a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/kmod-libs-28-10.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/7011810fca95064c8d78e55071716ec1dd5bc7b9836f662c195a282f4f4e5d0a",
    ],
)

rpm(
    name = "kmod-libs-0__28-10.el9.x86_64",
    sha256 = "79deb68a50b02b69df260fdb6e5c29f1b992290968ac6b07e7b249b2bdbc8ced",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/kmod-libs-28-10.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/79deb68a50b02b69df260fdb6e5c29f1b992290968ac6b07e7b249b2bdbc8ced",
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
    name = "krb5-libs-0__1.21.1-3.el9.aarch64",
    sha256 = "d551f9db7241d729210b8ca4ff5b32f6ac8122312bc09e7caf5584b1b1e311c8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/krb5-libs-1.21.1-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/d551f9db7241d729210b8ca4ff5b32f6ac8122312bc09e7caf5584b1b1e311c8",
    ],
)

rpm(
    name = "krb5-libs-0__1.21.1-3.el9.s390x",
    sha256 = "546f00d4eab306662ce8ce93a43a00f7f2c3e46dfe84a57df4402fadf14b3d9e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/krb5-libs-1.21.1-3.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/546f00d4eab306662ce8ce93a43a00f7f2c3e46dfe84a57df4402fadf14b3d9e",
    ],
)

rpm(
    name = "krb5-libs-0__1.21.1-3.el9.x86_64",
    sha256 = "76f4a5d0de611ad58a320c11dee853a7944b48f8adc4922a0e7ac7ee476d9716",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/krb5-libs-1.21.1-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/76f4a5d0de611ad58a320c11dee853a7944b48f8adc4922a0e7ac7ee476d9716",
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
    name = "libarchive-0__3.5.3-4.el9.aarch64",
    sha256 = "c043954972a8dea0b6cf5d3092c1eee90bb48b3fcb7cedf30aa861dc1d3f402c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libarchive-3.5.3-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c043954972a8dea0b6cf5d3092c1eee90bb48b3fcb7cedf30aa861dc1d3f402c",
    ],
)

rpm(
    name = "libarchive-0__3.5.3-4.el9.s390x",
    sha256 = "f95a05acd33d6f63a43ac2b065c45a3d2c9ef1923ec80d3a33946501dde0e751",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libarchive-3.5.3-4.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/f95a05acd33d6f63a43ac2b065c45a3d2c9ef1923ec80d3a33946501dde0e751",
    ],
)

rpm(
    name = "libarchive-0__3.5.3-4.el9.x86_64",
    sha256 = "4c53176eafd8c449aef704b8fbc2d5401bb7d2ea0a67961956f318f2e9a2c7a4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libarchive-3.5.3-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/4c53176eafd8c449aef704b8fbc2d5401bb7d2ea0a67961956f318f2e9a2c7a4",
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
    name = "libblkid-0__2.37.4-20.el9.aarch64",
    sha256 = "cebd26c399911e618eb2fa326cd0fd09ac8eb11884e9e4835aec01af79e18105",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libblkid-2.37.4-20.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/cebd26c399911e618eb2fa326cd0fd09ac8eb11884e9e4835aec01af79e18105",
    ],
)

rpm(
    name = "libblkid-0__2.37.4-20.el9.s390x",
    sha256 = "25e49a656a3eba08ef3041b90f18da2abfbc55f6e67257c192ccde9f4009cb56",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libblkid-2.37.4-20.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/25e49a656a3eba08ef3041b90f18da2abfbc55f6e67257c192ccde9f4009cb56",
    ],
)

rpm(
    name = "libblkid-0__2.37.4-20.el9.x86_64",
    sha256 = "5fa87671fdc5bb3e4e6c2b8e2253ac8fcf4add8ce44bf216864f952f10cdeeaa",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libblkid-2.37.4-20.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/5fa87671fdc5bb3e4e6c2b8e2253ac8fcf4add8ce44bf216864f952f10cdeeaa",
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
    name = "libcap-0__2.48-9.el9.aarch64",
    sha256 = "2d78c324f8f8d9a14042995ab6e4c063c7d0a6acec1be07ac0d0d2c1a6de0ca5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libcap-2.48-9.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/2d78c324f8f8d9a14042995ab6e4c063c7d0a6acec1be07ac0d0d2c1a6de0ca5",
    ],
)

rpm(
    name = "libcap-0__2.48-9.el9.s390x",
    sha256 = "5c0d3fa01feeda3389847de7c0cd8d2631c26f0e929f609f176cbb661e09a8a2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libcap-2.48-9.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/5c0d3fa01feeda3389847de7c0cd8d2631c26f0e929f609f176cbb661e09a8a2",
    ],
)

rpm(
    name = "libcap-0__2.48-9.el9.x86_64",
    sha256 = "7d07ec8a6a0975d84c66adf21c885c41a5571ecb631055959265c60fda314111",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcap-2.48-9.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/7d07ec8a6a0975d84c66adf21c885c41a5571ecb631055959265c60fda314111",
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
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libcom_err-1.46.5-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/77acee74fb925c5dc291691b23179a5b508372328696b8881627cc64f16bb2b5",
    ],
)

rpm(
    name = "libcom_err-0__1.46.5-2.el9.x86_64",
    sha256 = "579ca33574aca28a1c0de7951f6b183b5f2567cb01dfc40185e7b1f14da7f2c2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcom_err-1.46.5-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/579ca33574aca28a1c0de7951f6b183b5f2567cb01dfc40185e7b1f14da7f2c2",
    ],
)

rpm(
    name = "libcom_err-0__1.46.5-5.el9.aarch64",
    sha256 = "cd8b9b439b0434543cf0988567159bf9e6a329b7cbe8d9991a43375f88cc01d1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libcom_err-1.46.5-5.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/cd8b9b439b0434543cf0988567159bf9e6a329b7cbe8d9991a43375f88cc01d1",
    ],
)

rpm(
    name = "libcom_err-0__1.46.5-5.el9.s390x",
    sha256 = "3cca2a8ed3e319760a5935faf3d269288f0cea2cf2db2a5291e8996fc1ce7832",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libcom_err-1.46.5-5.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/3cca2a8ed3e319760a5935faf3d269288f0cea2cf2db2a5291e8996fc1ce7832",
    ],
)

rpm(
    name = "libcom_err-0__1.46.5-5.el9.x86_64",
    sha256 = "db2e675293b91b0f9b659cec0cad82c9c1b4af2112b6727e851d98a28ac83ed2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcom_err-1.46.5-5.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/db2e675293b91b0f9b659cec0cad82c9c1b4af2112b6727e851d98a28ac83ed2",
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
    name = "libcurl-minimal-0__7.76.1-31.el9.aarch64",
    sha256 = "9c0ec87af11f82ac5a2a4e6be45617b80737435a89c2be6a90a0e4b380e63053",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libcurl-minimal-7.76.1-31.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/9c0ec87af11f82ac5a2a4e6be45617b80737435a89c2be6a90a0e4b380e63053",
    ],
)

rpm(
    name = "libcurl-minimal-0__7.76.1-31.el9.s390x",
    sha256 = "ece81fe8aa2bfd5ff0c98cfdafe110a5e023184101ace9196d38a49665639b6f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libcurl-minimal-7.76.1-31.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/ece81fe8aa2bfd5ff0c98cfdafe110a5e023184101ace9196d38a49665639b6f",
    ],
)

rpm(
    name = "libcurl-minimal-0__7.76.1-31.el9.x86_64",
    sha256 = "6438485e38465ee944e25abedcf4a1761564fe5202f05a02c71e4c880255b539",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcurl-minimal-7.76.1-31.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/6438485e38465ee944e25abedcf4a1761564fe5202f05a02c71e4c880255b539",
    ],
)

rpm(
    name = "libdb-0__5.3.28-54.el9.aarch64",
    sha256 = "fdba5b07c422da16412014be63b29407423024a8c0aa367d0577c73600c65c93",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libdb-5.3.28-54.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/fdba5b07c422da16412014be63b29407423024a8c0aa367d0577c73600c65c93",
    ],
)

rpm(
    name = "libdb-0__5.3.28-54.el9.s390x",
    sha256 = "2dcb7dc38dff10884a00276da793b80f177327c49b4ebb81132575ebd09ed686",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libdb-5.3.28-54.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/2dcb7dc38dff10884a00276da793b80f177327c49b4ebb81132575ebd09ed686",
    ],
)

rpm(
    name = "libdb-0__5.3.28-54.el9.x86_64",
    sha256 = "3a54bfa30eab6a76d11ce543db49f697273aad0dbb54c20668a62e36a68aa32b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libdb-5.3.28-54.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/3a54bfa30eab6a76d11ce543db49f697273aad0dbb54c20668a62e36a68aa32b",
    ],
)

rpm(
    name = "libeconf-0__0.4.1-4.el9.aarch64",
    sha256 = "c221c71bfd8f6692e305a4e0c0025c4789ab04661c11a1a18c34c3f873f1276f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libeconf-0.4.1-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c221c71bfd8f6692e305a4e0c0025c4789ab04661c11a1a18c34c3f873f1276f",
    ],
)

rpm(
    name = "libeconf-0__0.4.1-4.el9.s390x",
    sha256 = "1ee2d8e7b48a5e9616c1f7a5b019e0aa054a80b5962d972104d78d095b2e926d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libeconf-0.4.1-4.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/1ee2d8e7b48a5e9616c1f7a5b019e0aa054a80b5962d972104d78d095b2e926d",
    ],
)

rpm(
    name = "libeconf-0__0.4.1-4.el9.x86_64",
    sha256 = "ed519cc2e9031e2bf03275b28c7cca6520ae916d0a7edbbc69f327c1b70ed6cc",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libeconf-0.4.1-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/ed519cc2e9031e2bf03275b28c7cca6520ae916d0a7edbbc69f327c1b70ed6cc",
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
    name = "libfdisk-0__2.37.4-20.el9.aarch64",
    sha256 = "c61bf4906bdd46399d50b453b557533060c5a3c344ac1bb0a9bb94ce41246e6f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libfdisk-2.37.4-20.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c61bf4906bdd46399d50b453b557533060c5a3c344ac1bb0a9bb94ce41246e6f",
    ],
)

rpm(
    name = "libfdisk-0__2.37.4-20.el9.s390x",
    sha256 = "bf3c3200f0a1e1b1b2fcd0e53b65226d562aee9762cabedd2471bdf2a402b454",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libfdisk-2.37.4-20.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/bf3c3200f0a1e1b1b2fcd0e53b65226d562aee9762cabedd2471bdf2a402b454",
    ],
)

rpm(
    name = "libfdisk-0__2.37.4-20.el9.x86_64",
    sha256 = "d1fcceb55185b4d898c8df3d0b9177126be0144b8829f908f40d2b58d44ad268",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libfdisk-2.37.4-20.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/d1fcceb55185b4d898c8df3d0b9177126be0144b8829f908f40d2b58d44ad268",
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
    name = "libgcc-0__11.5.0-2.el9.aarch64",
    sha256 = "f668e90e60502b349c33996abff84694c407c87e004b74df020f07ad030b846d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libgcc-11.5.0-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f668e90e60502b349c33996abff84694c407c87e004b74df020f07ad030b846d",
    ],
)

rpm(
    name = "libgcc-0__11.5.0-2.el9.s390x",
    sha256 = "ac6c003d9fe74072a7b3b34fc33fe649b5ec98cb0d8f08efb5239002cbd578c8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libgcc-11.5.0-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/ac6c003d9fe74072a7b3b34fc33fe649b5ec98cb0d8f08efb5239002cbd578c8",
    ],
)

rpm(
    name = "libgcc-0__11.5.0-2.el9.x86_64",
    sha256 = "ff344c9aaf0ef773230411b64e58d35d372314641b69113229afa6c539aa270a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libgcc-11.5.0-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/ff344c9aaf0ef773230411b64e58d35d372314641b69113229afa6c539aa270a",
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
    name = "libmount-0__2.37.4-20.el9.aarch64",
    sha256 = "84f9ee04bb2f3957e927dceaa9c36b3d3e009892b08741e1b45817b6eb6ca30c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libmount-2.37.4-20.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/84f9ee04bb2f3957e927dceaa9c36b3d3e009892b08741e1b45817b6eb6ca30c",
    ],
)

rpm(
    name = "libmount-0__2.37.4-20.el9.s390x",
    sha256 = "a917e4342e7934d4a6d361734e69e42694e59bca82d617305bd8f6aed9c2d7d4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libmount-2.37.4-20.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/a917e4342e7934d4a6d361734e69e42694e59bca82d617305bd8f6aed9c2d7d4",
    ],
)

rpm(
    name = "libmount-0__2.37.4-20.el9.x86_64",
    sha256 = "f602bea553bf92e512a39af33c3e8ee289dd9584e37d2ca02b69cb51b64dc623",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libmount-2.37.4-20.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f602bea553bf92e512a39af33c3e8ee289dd9584e37d2ca02b69cb51b64dc623",
    ],
)

rpm(
    name = "libnbd-0__1.20.2-2.el9.aarch64",
    sha256 = "77761819fff8b8e9a71542484d0117ffae7057151dd04b73f06c4908477249aa",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/libnbd-1.20.2-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/77761819fff8b8e9a71542484d0117ffae7057151dd04b73f06c4908477249aa",
    ],
)

rpm(
    name = "libnbd-0__1.20.2-2.el9.s390x",
    sha256 = "9c45112cbfa18bfd2b92588a592c5bb3d39c83744168b1dfa3b6997a7ae9a754",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/libnbd-1.20.2-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/9c45112cbfa18bfd2b92588a592c5bb3d39c83744168b1dfa3b6997a7ae9a754",
    ],
)

rpm(
    name = "libnbd-0__1.20.2-2.el9.x86_64",
    sha256 = "cda5a0f85ad33681d86b3fd2f33642769def489e8970f9a78c8dfe46163dd805",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libnbd-1.20.2-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/cda5a0f85ad33681d86b3fd2f33642769def489e8970f9a78c8dfe46163dd805",
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
    name = "libnfnetlink-0__1.0.1-21.el9.aarch64",
    sha256 = "682c4cca565ce483ff0749dbb39b154bc080ac531c418d05890e454114c11821",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libnfnetlink-1.0.1-21.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/682c4cca565ce483ff0749dbb39b154bc080ac531c418d05890e454114c11821",
    ],
)

rpm(
    name = "libnfnetlink-0__1.0.1-21.el9.s390x",
    sha256 = "30dc6e1a8e1a026ff5a59759cf1cf8456f478c81fa11bc44aa69b9e80d7c3b5b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libnfnetlink-1.0.1-21.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/30dc6e1a8e1a026ff5a59759cf1cf8456f478c81fa11bc44aa69b9e80d7c3b5b",
    ],
)

rpm(
    name = "libnfnetlink-0__1.0.1-21.el9.x86_64",
    sha256 = "64f54f412cc0ee6fe82be7557f471a06f6bf1f5bba1d6fe0ad1879e5a62d7c95",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnfnetlink-1.0.1-21.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/64f54f412cc0ee6fe82be7557f471a06f6bf1f5bba1d6fe0ad1879e5a62d7c95",
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
    name = "libselinux-0__3.6-2.el9.aarch64",
    sha256 = "a3286f9e68923cc7acf33297b90cf39b4ead485f044cc97b0d1dc8daa9aed086",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libselinux-3.6-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/a3286f9e68923cc7acf33297b90cf39b4ead485f044cc97b0d1dc8daa9aed086",
    ],
)

rpm(
    name = "libselinux-0__3.6-2.el9.s390x",
    sha256 = "c9db29eceb5f4c5aae0e823ebe99729512434260b71426bc6ccdc1177d0958d5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libselinux-3.6-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/c9db29eceb5f4c5aae0e823ebe99729512434260b71426bc6ccdc1177d0958d5",
    ],
)

rpm(
    name = "libselinux-0__3.6-2.el9.x86_64",
    sha256 = "25730cb1b020298f50c681249479b418edd54fb68732e765012ab90e67b77479",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libselinux-3.6-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/25730cb1b020298f50c681249479b418edd54fb68732e765012ab90e67b77479",
    ],
)

rpm(
    name = "libselinux-utils-0__3.6-2.el9.aarch64",
    sha256 = "84d2614f351ad674d64fed4600bcbf4129ebfe2b098a64e1f9772f3daf0af32d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libselinux-utils-3.6-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/84d2614f351ad674d64fed4600bcbf4129ebfe2b098a64e1f9772f3daf0af32d",
    ],
)

rpm(
    name = "libselinux-utils-0__3.6-2.el9.s390x",
    sha256 = "a32d36fcff35315c74192d7b0c8410f81c8d8e6ff698009180704039b932286f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libselinux-utils-3.6-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/a32d36fcff35315c74192d7b0c8410f81c8d8e6ff698009180704039b932286f",
    ],
)

rpm(
    name = "libselinux-utils-0__3.6-2.el9.x86_64",
    sha256 = "f7bd1cd6202c47cb1a7299d8de08199ec991f07a21560446de06d1d6a7cb1615",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libselinux-utils-3.6-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f7bd1cd6202c47cb1a7299d8de08199ec991f07a21560446de06d1d6a7cb1615",
    ],
)

rpm(
    name = "libsemanage-0__3.6-2.el9.aarch64",
    sha256 = "db257bae76907d7ca180c8683ca1d3b0fdab248e7ad075b69d7c020d8ad0fbec",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libsemanage-3.6-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/db257bae76907d7ca180c8683ca1d3b0fdab248e7ad075b69d7c020d8ad0fbec",
    ],
)

rpm(
    name = "libsemanage-0__3.6-2.el9.s390x",
    sha256 = "e1d7415e124c5c0373b78cf720f568a70da9cca1f4fd544f8601da3a6c5d9642",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libsemanage-3.6-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/e1d7415e124c5c0373b78cf720f568a70da9cca1f4fd544f8601da3a6c5d9642",
    ],
)

rpm(
    name = "libsemanage-0__3.6-2.el9.x86_64",
    sha256 = "4d7ca4fcab7fa013f911e00c0a6c1103960eac6d81fb666d78ba4498d50f05b5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsemanage-3.6-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/4d7ca4fcab7fa013f911e00c0a6c1103960eac6d81fb666d78ba4498d50f05b5",
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
    name = "libsepol-0__3.6-1.el9.aarch64",
    sha256 = "d5fbf72e47423eadf245d8cf8ecc3fb8bec2725ea0504c2cec8d68120603783a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libsepol-3.6-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/d5fbf72e47423eadf245d8cf8ecc3fb8bec2725ea0504c2cec8d68120603783a",
    ],
)

rpm(
    name = "libsepol-0__3.6-1.el9.s390x",
    sha256 = "58df3e6e550cded42d31f51140e7d0adc427bc4efbb6737e8efe3b6a30680369",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libsepol-3.6-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/58df3e6e550cded42d31f51140e7d0adc427bc4efbb6737e8efe3b6a30680369",
    ],
)

rpm(
    name = "libsepol-0__3.6-1.el9.x86_64",
    sha256 = "834f9dd59bf8bd0cf5047c672b1d610b722a0981f53c15dd36cc3daffaba0230",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsepol-3.6-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/834f9dd59bf8bd0cf5047c672b1d610b722a0981f53c15dd36cc3daffaba0230",
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
    name = "libslirp-0__4.4.0-8.el9.aarch64",
    sha256 = "52a73957cdbce4484adc9755e42393aeb31443e199fbcdcf3ae867dee82145bf",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/libslirp-4.4.0-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/52a73957cdbce4484adc9755e42393aeb31443e199fbcdcf3ae867dee82145bf",
    ],
)

rpm(
    name = "libslirp-0__4.4.0-8.el9.s390x",
    sha256 = "d47be3b8520589ff857b0264075f98b0483863762a0d3b0ebb1fba7c870edba6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/libslirp-4.4.0-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/d47be3b8520589ff857b0264075f98b0483863762a0d3b0ebb1fba7c870edba6",
    ],
)

rpm(
    name = "libslirp-0__4.4.0-8.el9.x86_64",
    sha256 = "aa5c4568ef12b3324e28e2353a97e5d531892e9e0682a035a5669819c7fd6dc3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libslirp-4.4.0-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/aa5c4568ef12b3324e28e2353a97e5d531892e9e0682a035a5669819c7fd6dc3",
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
    name = "libsmartcols-0__2.37.4-20.el9.aarch64",
    sha256 = "e81543e1ac16943bf49fb9a74526ffa6f0cee41e902f93282b9d8787154ba08b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libsmartcols-2.37.4-20.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/e81543e1ac16943bf49fb9a74526ffa6f0cee41e902f93282b9d8787154ba08b",
    ],
)

rpm(
    name = "libsmartcols-0__2.37.4-20.el9.s390x",
    sha256 = "afc481221d6f3adc1727289ca543ee40bb410a9c564fba75d356c8a51131ece0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libsmartcols-2.37.4-20.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/afc481221d6f3adc1727289ca543ee40bb410a9c564fba75d356c8a51131ece0",
    ],
)

rpm(
    name = "libsmartcols-0__2.37.4-20.el9.x86_64",
    sha256 = "e51f3a4fac42fe95d4a7fb1128afd99d9cb7cfdb6ab2ec5e68089bbb72af13ca",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsmartcols-2.37.4-20.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e51f3a4fac42fe95d4a7fb1128afd99d9cb7cfdb6ab2ec5e68089bbb72af13ca",
    ],
)

rpm(
    name = "libstdc__plus____plus__-0__11.5.0-2.el9.aarch64",
    sha256 = "fff7f00d26008ab09b566c3b14d446b4b0b3df08bedbeee29142d62278568c82",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libstdc++-11.5.0-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/fff7f00d26008ab09b566c3b14d446b4b0b3df08bedbeee29142d62278568c82",
    ],
)

rpm(
    name = "libstdc__plus____plus__-0__11.5.0-2.el9.s390x",
    sha256 = "3644bcebe706602976b4d1596eedefcb0af0cfdb74141ca9084e0b34e8d22890",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libstdc++-11.5.0-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/3644bcebe706602976b4d1596eedefcb0af0cfdb74141ca9084e0b34e8d22890",
    ],
)

rpm(
    name = "libstdc__plus____plus__-0__11.5.0-2.el9.x86_64",
    sha256 = "dcd7090c2a37f13b2d4a1a2bc2d1fedc514c745efc4f2619783bbd1979b5e82f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libstdc++-11.5.0-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/dcd7090c2a37f13b2d4a1a2bc2d1fedc514c745efc4f2619783bbd1979b5e82f",
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
    name = "libtasn1-0__4.16.0-8.el9.aarch64",
    sha256 = "1046c07821506ef6a84291b093de0d62dcc9873142e1ac2c66aaa72abd08532c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libtasn1-4.16.0-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/1046c07821506ef6a84291b093de0d62dcc9873142e1ac2c66aaa72abd08532c",
    ],
)

rpm(
    name = "libtasn1-0__4.16.0-8.el9.s390x",
    sha256 = "1a03374dd2825e0cc9dacddb31c9537835138b0c12713faed4d38890bb1a3882",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libtasn1-4.16.0-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/1a03374dd2825e0cc9dacddb31c9537835138b0c12713faed4d38890bb1a3882",
    ],
)

rpm(
    name = "libtasn1-0__4.16.0-8.el9.x86_64",
    sha256 = "c8b13c9e1292de474e76ab80f230f86cce2e8f5f53592e168bdcaa604ed1b37d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libtasn1-4.16.0-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c8b13c9e1292de474e76ab80f230f86cce2e8f5f53592e168bdcaa604ed1b37d",
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
    name = "liburing-0__2.5-1.el9.aarch64",
    sha256 = "12f91bd14e1eb7e2b37783561c1a0658d85c7ee2a9259391ed15e01bf4186649",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/liburing-2.5-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/12f91bd14e1eb7e2b37783561c1a0658d85c7ee2a9259391ed15e01bf4186649",
    ],
)

rpm(
    name = "liburing-0__2.5-1.el9.s390x",
    sha256 = "f45d4fcccfd217d5aa394a317d4d2645b79edb50cd7ad01dc14ad0d1b1bdb2f0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/liburing-2.5-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/f45d4fcccfd217d5aa394a317d4d2645b79edb50cd7ad01dc14ad0d1b1bdb2f0",
    ],
)

rpm(
    name = "liburing-0__2.5-1.el9.x86_64",
    sha256 = "12558038d4226495da372e5f4369d02c144c759a621d27116299ce0a794e849f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/liburing-2.5-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/12558038d4226495da372e5f4369d02c144c759a621d27116299ce0a794e849f",
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
    name = "libuuid-0__2.37.4-20.el9.aarch64",
    sha256 = "f1c54eeed0c892cb9cc3bea42e8c09b5a4b515381eb5d0fe6e5eb84346c51839",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libuuid-2.37.4-20.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f1c54eeed0c892cb9cc3bea42e8c09b5a4b515381eb5d0fe6e5eb84346c51839",
    ],
)

rpm(
    name = "libuuid-0__2.37.4-20.el9.s390x",
    sha256 = "6021fe138b00f88d32a7745efac96331e7302e11c41aa302e04dd7283df8ab36",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libuuid-2.37.4-20.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/6021fe138b00f88d32a7745efac96331e7302e11c41aa302e04dd7283df8ab36",
    ],
)

rpm(
    name = "libuuid-0__2.37.4-20.el9.x86_64",
    sha256 = "10754bbddc76e88458ae6e9fd7b00cd6e5102c9e493eb2df73372b8f1d88dc1b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libuuid-2.37.4-20.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/10754bbddc76e88458ae6e9fd7b00cd6e5102c9e493eb2df73372b8f1d88dc1b",
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
    name = "libxml2-0__2.9.13-6.el9.aarch64",
    sha256 = "d567f4bcf953cffe949be6d11d5597bf1a8c806c89c999e7943c240da40122b8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libxml2-2.9.13-6.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/d567f4bcf953cffe949be6d11d5597bf1a8c806c89c999e7943c240da40122b8",
    ],
)

rpm(
    name = "libxml2-0__2.9.13-6.el9.s390x",
    sha256 = "2ba167d1c5fe690868d32c2f09645a080297ca7f731c9793c9ac89ff8043455d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libxml2-2.9.13-6.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/2ba167d1c5fe690868d32c2f09645a080297ca7f731c9793c9ac89ff8043455d",
    ],
)

rpm(
    name = "libxml2-0__2.9.13-6.el9.x86_64",
    sha256 = "7b23a9ca73db2ec13ee983594d4d0f4a85160ef8d05484f65c247801cb808a29",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libxml2-2.9.13-6.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/7b23a9ca73db2ec13ee983594d4d0f4a85160ef8d05484f65c247801cb808a29",
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
    name = "libzstd-0__1.5.1-2.el9.s390x",
    sha256 = "a84659a6861d44aaa063e69d58c1a582c34431b2e168965ac9e717ce7efb5b4a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libzstd-1.5.1-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/a84659a6861d44aaa063e69d58c1a582c34431b2e168965ac9e717ce7efb5b4a",
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
    name = "lua-libs-0__5.4.4-4.el9.aarch64",
    sha256 = "bd72283eb56206de91a71b1b7dbdcca1201fdaea4a08faf7b92d8ef9a600a88a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/lua-libs-5.4.4-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/bd72283eb56206de91a71b1b7dbdcca1201fdaea4a08faf7b92d8ef9a600a88a",
    ],
)

rpm(
    name = "lua-libs-0__5.4.4-4.el9.s390x",
    sha256 = "616111e91869993d6db2fec066d5b5b29b2c17bfbce87748a51ed772dbc4d4ca",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/lua-libs-5.4.4-4.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/616111e91869993d6db2fec066d5b5b29b2c17bfbce87748a51ed772dbc4d4ca",
    ],
)

rpm(
    name = "lua-libs-0__5.4.4-4.el9.x86_64",
    sha256 = "a24f7e08163b012cdbbdaba70788331050c2b7bdb9bc2fdc261c5c1f3cd3960d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/lua-libs-5.4.4-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/a24f7e08163b012cdbbdaba70788331050c2b7bdb9bc2fdc261c5c1f3cd3960d",
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
    name = "nbdkit-basic-filters-0__1.38.3-1.el9.aarch64",
    sha256 = "54aa15abcd99b477c5c09e8f583ea7a7ef65456d7d4b1c5238836d886a7a5061",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-basic-filters-1.38.3-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/54aa15abcd99b477c5c09e8f583ea7a7ef65456d7d4b1c5238836d886a7a5061",
    ],
)

rpm(
    name = "nbdkit-basic-filters-0__1.38.3-1.el9.s390x",
    sha256 = "d034cf81f3bfc29ce43e4dd6485ae174ac1ec165f5995336481c91d689d91f4b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nbdkit-basic-filters-1.38.3-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/d034cf81f3bfc29ce43e4dd6485ae174ac1ec165f5995336481c91d689d91f4b",
    ],
)

rpm(
    name = "nbdkit-basic-filters-0__1.38.3-1.el9.x86_64",
    sha256 = "2ad146adcb3e97799c753b77ec62a053fff431c2729293641f8802ba3345aee3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-basic-filters-1.38.3-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/2ad146adcb3e97799c753b77ec62a053fff431c2729293641f8802ba3345aee3",
    ],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.38.3-1.el9.aarch64",
    sha256 = "48e087e299e117bc40a0e537d050a90dd856d189919146199660397b427bc672",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-curl-plugin-1.38.3-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/48e087e299e117bc40a0e537d050a90dd856d189919146199660397b427bc672",
    ],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.38.3-1.el9.s390x",
    sha256 = "be9620c68f9a2f3ddb060b737ff94fcfc9b9d8b7bdd75b590a09b40160aedfb5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nbdkit-curl-plugin-1.38.3-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/be9620c68f9a2f3ddb060b737ff94fcfc9b9d8b7bdd75b590a09b40160aedfb5",
    ],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.38.3-1.el9.x86_64",
    sha256 = "50399bc6593ef1ebedba72d336e7321c2f1b4fbaacbad1665c6975995dd13089",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-curl-plugin-1.38.3-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/50399bc6593ef1ebedba72d336e7321c2f1b4fbaacbad1665c6975995dd13089",
    ],
)

rpm(
    name = "nbdkit-gzip-filter-0__1.38.3-1.el9.aarch64",
    sha256 = "69155a4aacac2311e8c53c94e9e18c6db9f226e4099a4417c959ded7a0b0989b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-gzip-filter-1.38.3-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/69155a4aacac2311e8c53c94e9e18c6db9f226e4099a4417c959ded7a0b0989b",
    ],
)

rpm(
    name = "nbdkit-gzip-filter-0__1.38.3-1.el9.s390x",
    sha256 = "f245e3a35de1839d7c0f0c9a4fbfa03672ac8820739a4631b8babc90e7e2f033",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nbdkit-gzip-filter-1.38.3-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/f245e3a35de1839d7c0f0c9a4fbfa03672ac8820739a4631b8babc90e7e2f033",
    ],
)

rpm(
    name = "nbdkit-gzip-filter-0__1.38.3-1.el9.x86_64",
    sha256 = "b74c8b6f273f55909d498cba30f02740785dae1be3e081dd4635c69b1bf63e4a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-gzip-filter-1.38.3-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/b74c8b6f273f55909d498cba30f02740785dae1be3e081dd4635c69b1bf63e4a",
    ],
)

rpm(
    name = "nbdkit-server-0__1.38.3-1.el9.aarch64",
    sha256 = "747ad87c46fd8b73e0f94408debcce76025cc9b8ffa1dc0bccd0db1f16e64a6f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-server-1.38.3-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/747ad87c46fd8b73e0f94408debcce76025cc9b8ffa1dc0bccd0db1f16e64a6f",
    ],
)

rpm(
    name = "nbdkit-server-0__1.38.3-1.el9.s390x",
    sha256 = "d5f685ba9144586833b00db95571abf5d5e1f6e98307fd00ef35c6ef3ca5dc54",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nbdkit-server-1.38.3-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/d5f685ba9144586833b00db95571abf5d5e1f6e98307fd00ef35c6ef3ca5dc54",
    ],
)

rpm(
    name = "nbdkit-server-0__1.38.3-1.el9.x86_64",
    sha256 = "c7d5c86229a21c1deb0df0826cbbce8599e5fc802d44e30c1fe9ead1ab668ced",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-server-1.38.3-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c7d5c86229a21c1deb0df0826cbbce8599e5fc802d44e30c1fe9ead1ab668ced",
    ],
)

rpm(
    name = "nbdkit-vddk-plugin-0__1.38.3-1.el9.x86_64",
    sha256 = "9edc882953871a0aafbab48f4466d2b7c41fc59d393a7c04c7fad00bd6acc542",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-vddk-plugin-1.38.3-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/9edc882953871a0aafbab48f4466d2b7c41fc59d393a7c04c7fad00bd6acc542",
    ],
)

rpm(
    name = "nbdkit-xz-filter-0__1.38.3-1.el9.aarch64",
    sha256 = "cc9178311ff0da758736d72d8a25e6ea866a1bf13ae01c5ab81f637f9cc4edc1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-xz-filter-1.38.3-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/cc9178311ff0da758736d72d8a25e6ea866a1bf13ae01c5ab81f637f9cc4edc1",
    ],
)

rpm(
    name = "nbdkit-xz-filter-0__1.38.3-1.el9.s390x",
    sha256 = "9d7cfd87165cf04892c92e65eff48dc5c1f906b72900b8124e03c67ec2b8522b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nbdkit-xz-filter-1.38.3-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/9d7cfd87165cf04892c92e65eff48dc5c1f906b72900b8124e03c67ec2b8522b",
    ],
)

rpm(
    name = "nbdkit-xz-filter-0__1.38.3-1.el9.x86_64",
    sha256 = "61091c8765e22a06c0d6a8d89956794bc47afb3f2081ed8787641117b8190975",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-xz-filter-1.38.3-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/61091c8765e22a06c0d6a8d89956794bc47afb3f2081ed8787641117b8190975",
    ],
)

rpm(
    name = "ncurses-base-0__6.2-10.20210508.el9.aarch64",
    sha256 = "00ba56b28a3a85c3c03387bb7abeca92597c8a5fac7f53d48410ca2a20fd8065",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/ncurses-base-6.2-10.20210508.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/00ba56b28a3a85c3c03387bb7abeca92597c8a5fac7f53d48410ca2a20fd8065",
    ],
)

rpm(
    name = "ncurses-base-0__6.2-10.20210508.el9.s390x",
    sha256 = "00ba56b28a3a85c3c03387bb7abeca92597c8a5fac7f53d48410ca2a20fd8065",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/ncurses-base-6.2-10.20210508.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/00ba56b28a3a85c3c03387bb7abeca92597c8a5fac7f53d48410ca2a20fd8065",
    ],
)

rpm(
    name = "ncurses-base-0__6.2-10.20210508.el9.x86_64",
    sha256 = "00ba56b28a3a85c3c03387bb7abeca92597c8a5fac7f53d48410ca2a20fd8065",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ncurses-base-6.2-10.20210508.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/00ba56b28a3a85c3c03387bb7abeca92597c8a5fac7f53d48410ca2a20fd8065",
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
    name = "ncurses-libs-0__6.2-10.20210508.el9.aarch64",
    sha256 = "0ccfc9eeb99be404367bf6157db2d1a6fb9ed479247f578501594e08e8f7080c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/ncurses-libs-6.2-10.20210508.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/0ccfc9eeb99be404367bf6157db2d1a6fb9ed479247f578501594e08e8f7080c",
    ],
)

rpm(
    name = "ncurses-libs-0__6.2-10.20210508.el9.s390x",
    sha256 = "6ff5f715d02fa044b431b4766e13a424961faa04795f3189b05bf5c58b13dee2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/ncurses-libs-6.2-10.20210508.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/6ff5f715d02fa044b431b4766e13a424961faa04795f3189b05bf5c58b13dee2",
    ],
)

rpm(
    name = "ncurses-libs-0__6.2-10.20210508.el9.x86_64",
    sha256 = "f4ead70a508051ed338499b35605b5b2b5bccde19c9e83f7e4b948f171b542ff",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ncurses-libs-6.2-10.20210508.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f4ead70a508051ed338499b35605b5b2b5bccde19c9e83f7e4b948f171b542ff",
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
    name = "netavark-2__1.12.2-1.el9.aarch64",
    sha256 = "6f1d4753ced34347bb4b98b712c131143466922faa4d2b5d9a3b861ac4236b51",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/netavark-1.12.2-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/6f1d4753ced34347bb4b98b712c131143466922faa4d2b5d9a3b861ac4236b51",
    ],
)

rpm(
    name = "netavark-2__1.12.2-1.el9.s390x",
    sha256 = "748a936a7e94d8a451738e027f8fdcb74abb406c1ced582b49df76f257b4557a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/netavark-1.12.2-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/748a936a7e94d8a451738e027f8fdcb74abb406c1ced582b49df76f257b4557a",
    ],
)

rpm(
    name = "netavark-2__1.12.2-1.el9.x86_64",
    sha256 = "acb2597ced875efb97894e00f765dfbdea8edf50191dbc78609850a62b0c90a6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/netavark-1.12.2-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/acb2597ced875efb97894e00f765dfbdea8edf50191dbc78609850a62b0c90a6",
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
    name = "nettle-0__3.9.1-1.el9.aarch64",
    sha256 = "991294c5c3f1544172cbc0c3bf27540036e0d09f42c161ef8bdf231c97d9ced0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/nettle-3.9.1-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/991294c5c3f1544172cbc0c3bf27540036e0d09f42c161ef8bdf231c97d9ced0",
    ],
)

rpm(
    name = "nettle-0__3.9.1-1.el9.s390x",
    sha256 = "3b13fd8975ebb5bf3eff89eeb0d5ec0dc6f65d8bd8776b1dae8d2c8ce99b54bb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/nettle-3.9.1-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/3b13fd8975ebb5bf3eff89eeb0d5ec0dc6f65d8bd8776b1dae8d2c8ce99b54bb",
    ],
)

rpm(
    name = "nettle-0__3.9.1-1.el9.x86_64",
    sha256 = "ffeeab0a6b0caaf457ad77a64bb1dfac6c1144343f1057de64a89b5ae4b58bf5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/nettle-3.9.1-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/ffeeab0a6b0caaf457ad77a64bb1dfac6c1144343f1057de64a89b5ae4b58bf5",
    ],
)

rpm(
    name = "nftables-1__1.0.9-3.el9.aarch64",
    sha256 = "979faab3c0c318f4f1df5edd8b06efb20898461003237af3838f937d63b12d98",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/nftables-1.0.9-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/979faab3c0c318f4f1df5edd8b06efb20898461003237af3838f937d63b12d98",
    ],
)

rpm(
    name = "nftables-1__1.0.9-3.el9.s390x",
    sha256 = "a8d9bd2a045a06a50756af71d41a3d4d15677d120bb1cf833907db2e990adad0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/nftables-1.0.9-3.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/a8d9bd2a045a06a50756af71d41a3d4d15677d120bb1cf833907db2e990adad0",
    ],
)

rpm(
    name = "nftables-1__1.0.9-3.el9.x86_64",
    sha256 = "3f72eee1c40da5fa1f2eb59a77723f781ff27c53411b2aca1aee8bd6a577915b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/nftables-1.0.9-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/3f72eee1c40da5fa1f2eb59a77723f781ff27c53411b2aca1aee8bd6a577915b",
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
    name = "numactl-libs-0__2.0.16-3.el9.aarch64",
    sha256 = "018b1f427fd576c1acd7ba2dd79f74a49ee8afab5670a2519241260ef1466562",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/numactl-libs-2.0.16-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/018b1f427fd576c1acd7ba2dd79f74a49ee8afab5670a2519241260ef1466562",
    ],
)

rpm(
    name = "numactl-libs-0__2.0.16-3.el9.x86_64",
    sha256 = "56167ea50d70d737d28da028f279f42ac4b624f95ef8f5cce05944cb804230af",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/numactl-libs-2.0.16-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/56167ea50d70d737d28da028f279f42ac4b624f95ef8f5cce05944cb804230af",
    ],
)

rpm(
    name = "openldap-0__2.6.6-3.el9.aarch64",
    sha256 = "3553d2a8ef3901444aae018a0f1a17f41d3ffba77af79735e8422c37f6ac57d8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openldap-2.6.6-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/3553d2a8ef3901444aae018a0f1a17f41d3ffba77af79735e8422c37f6ac57d8",
    ],
)

rpm(
    name = "openldap-0__2.6.6-3.el9.s390x",
    sha256 = "3c675112c68a0aadab98d1f33ca35eaa182a473000433590d5a86b9c55fdd6cb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openldap-2.6.6-3.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/3c675112c68a0aadab98d1f33ca35eaa182a473000433590d5a86b9c55fdd6cb",
    ],
)

rpm(
    name = "openldap-0__2.6.6-3.el9.x86_64",
    sha256 = "da4c54a99c4556ab6c95f91ac0f472e8e96509fd97a59f45e196c0f613a1dbab",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openldap-2.6.6-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/da4c54a99c4556ab6c95f91ac0f472e8e96509fd97a59f45e196c0f613a1dbab",
    ],
)

rpm(
    name = "openssl-1__3.2.2-4.el9.aarch64",
    sha256 = "82972aca54c30e46986e1baee00cf67df01917eda7b1e7dc7c4a830ad931008c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openssl-3.2.2-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/82972aca54c30e46986e1baee00cf67df01917eda7b1e7dc7c4a830ad931008c",
    ],
)

rpm(
    name = "openssl-1__3.2.2-4.el9.s390x",
    sha256 = "0caff3bbe2de13535cd24e892a2baef40325f097e6300e665fe33ce75710ed38",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openssl-3.2.2-4.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/0caff3bbe2de13535cd24e892a2baef40325f097e6300e665fe33ce75710ed38",
    ],
)

rpm(
    name = "openssl-1__3.2.2-4.el9.x86_64",
    sha256 = "6573d68236ecc35c5b129db840f6fe96fba54f75b82e7212b65364c9755f60ac",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-3.2.2-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/6573d68236ecc35c5b129db840f6fe96fba54f75b82e7212b65364c9755f60ac",
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
    name = "openssl-libs-1__3.2.2-4.el9.aarch64",
    sha256 = "37fd63901616edda7d342b312503ee6fe453eb0605df76d267adb5f8becb6077",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openssl-libs-3.2.2-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/37fd63901616edda7d342b312503ee6fe453eb0605df76d267adb5f8becb6077",
    ],
)

rpm(
    name = "openssl-libs-1__3.2.2-4.el9.s390x",
    sha256 = "7e2328e623a5e032f97b4a448c48595e3050cd97cc3bc64a6ec7cfefaefeb544",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openssl-libs-3.2.2-4.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/7e2328e623a5e032f97b4a448c48595e3050cd97cc3bc64a6ec7cfefaefeb544",
    ],
)

rpm(
    name = "openssl-libs-1__3.2.2-4.el9.x86_64",
    sha256 = "b320a2add7cb49a777f5f90a80c254b278d71236be9a8b09d1c666137c680406",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-libs-3.2.2-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/b320a2add7cb49a777f5f90a80c254b278d71236be9a8b09d1c666137c680406",
    ],
)

rpm(
    name = "ovirt-imageio-client-0__2.5.1-0.202402212039.git21dd9f7.el9.x86_64",
    sha256 = "981f26c8227e25669770ca293b34eab1bfd17781e0f43e258804eef071c7a5ab",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-9-x86_64/07047346-ovirt-imageio/ovirt-imageio-client-2.5.1-0.202402212039.git21dd9f7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/981f26c8227e25669770ca293b34eab1bfd17781e0f43e258804eef071c7a5ab",
    ],
)

rpm(
    name = "ovirt-imageio-client-0__2.5.1-0.202402212040.git21dd9f7.el9.aarch64",
    sha256 = "e090bdf62b0ae862878306c2b0e259f0845a9cbdc234dbd652e274507380362f",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/nsoffer/ovirt-imageio-preview/centos-stream-9-aarch64/07047345-ovirt-imageio/ovirt-imageio-client-2.5.1-0.202402212040.git21dd9f7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/e090bdf62b0ae862878306c2b0e259f0845a9cbdc234dbd652e274507380362f",
    ],
)

rpm(
    name = "ovirt-imageio-common-0__2.5.1-0.202402212039.git21dd9f7.el9.x86_64",
    sha256 = "75b91c3844d572df1ce83b02acc27f3f96912c9275abe202a51ceca94db10c81",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-9-x86_64/07047346-ovirt-imageio/ovirt-imageio-common-2.5.1-0.202402212039.git21dd9f7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/75b91c3844d572df1ce83b02acc27f3f96912c9275abe202a51ceca94db10c81",
    ],
)

rpm(
    name = "ovirt-imageio-common-0__2.5.1-0.202402212040.git21dd9f7.el9.aarch64",
    sha256 = "7127fea08ebc8a5985ddda539afe7b0e45a18be7212c8e91bbd2ed672ee8d2e6",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/nsoffer/ovirt-imageio-preview/centos-stream-9-aarch64/07047345-ovirt-imageio/ovirt-imageio-common-2.5.1-0.202402212040.git21dd9f7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/7127fea08ebc8a5985ddda539afe7b0e45a18be7212c8e91bbd2ed672ee8d2e6",
    ],
)

rpm(
    name = "p11-kit-0__0.24.1-2.el9.aarch64",
    sha256 = "98e7f00d012549fa8fbaba21626388a0b07731f3f25a5801418247d66a5a985f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/p11-kit-0.24.1-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/98e7f00d012549fa8fbaba21626388a0b07731f3f25a5801418247d66a5a985f",
    ],
)

rpm(
    name = "p11-kit-0__0.24.1-2.el9.x86_64",
    sha256 = "da167e41efd19cf25fd1c708b6f123d0203824324b14dd32401d49f2aa0ef0a6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/p11-kit-0.24.1-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/da167e41efd19cf25fd1c708b6f123d0203824324b14dd32401d49f2aa0ef0a6",
    ],
)

rpm(
    name = "p11-kit-0__0.25.3-2.el9.aarch64",
    sha256 = "bdd4c7f279730c079b5f766a5c9f1297ee02120840bd12f084ab6f22e50c2203",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/p11-kit-0.25.3-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/bdd4c7f279730c079b5f766a5c9f1297ee02120840bd12f084ab6f22e50c2203",
    ],
)

rpm(
    name = "p11-kit-0__0.25.3-2.el9.s390x",
    sha256 = "fc238839eff36402fdc4fabb7b1a2298f512c384e62c2de5f931f62943a8d595",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/p11-kit-0.25.3-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/fc238839eff36402fdc4fabb7b1a2298f512c384e62c2de5f931f62943a8d595",
    ],
)

rpm(
    name = "p11-kit-0__0.25.3-2.el9.x86_64",
    sha256 = "0839ee9854251e66f3109b6c685f1e6b3cce6d2a1415e9f71a03d71f03eeb708",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/p11-kit-0.25.3-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/0839ee9854251e66f3109b6c685f1e6b3cce6d2a1415e9f71a03d71f03eeb708",
    ],
)

rpm(
    name = "p11-kit-trust-0__0.24.1-2.el9.aarch64",
    sha256 = "80e288a5b62f20f7794674c6fdf2f0765a322cd0e81df9359e37582fe950289c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/p11-kit-trust-0.24.1-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/80e288a5b62f20f7794674c6fdf2f0765a322cd0e81df9359e37582fe950289c",
    ],
)

rpm(
    name = "p11-kit-trust-0__0.24.1-2.el9.x86_64",
    sha256 = "ae9a633c58980328bef6358c6aa3c9ce0a65130c66fbfa4249922ddf5a3e2bb1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/p11-kit-trust-0.24.1-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/ae9a633c58980328bef6358c6aa3c9ce0a65130c66fbfa4249922ddf5a3e2bb1",
    ],
)

rpm(
    name = "p11-kit-trust-0__0.25.3-2.el9.aarch64",
    sha256 = "f49208d939702ade5ff36a42af67be05ca5c3125665c23275520880a07f5d16a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/p11-kit-trust-0.25.3-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f49208d939702ade5ff36a42af67be05ca5c3125665c23275520880a07f5d16a",
    ],
)

rpm(
    name = "p11-kit-trust-0__0.25.3-2.el9.s390x",
    sha256 = "3e506a1cd02aa89b7de59cfb2b1370a8130fa45dd9034f426d6d18bfee42943c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/p11-kit-trust-0.25.3-2.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/3e506a1cd02aa89b7de59cfb2b1370a8130fa45dd9034f426d6d18bfee42943c",
    ],
)

rpm(
    name = "p11-kit-trust-0__0.25.3-2.el9.x86_64",
    sha256 = "177b963e62a19a2539138c1e5828a331bdf04c3675829a0dc88699765a4e0e63",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/p11-kit-trust-0.25.3-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/177b963e62a19a2539138c1e5828a331bdf04c3675829a0dc88699765a4e0e63",
    ],
)

rpm(
    name = "pam-0__1.5.1-20.el9.aarch64",
    sha256 = "6b2e3c4959d31c7137b81a96188d2314866cc1229937978b8fb7c9e1a5b83704",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/pam-1.5.1-20.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/6b2e3c4959d31c7137b81a96188d2314866cc1229937978b8fb7c9e1a5b83704",
    ],
)

rpm(
    name = "pam-0__1.5.1-20.el9.s390x",
    sha256 = "c56873f126b79e8f9585a7b84999fdab1e4119d1463cc837bb881a1dab9b58bd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/pam-1.5.1-20.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/c56873f126b79e8f9585a7b84999fdab1e4119d1463cc837bb881a1dab9b58bd",
    ],
)

rpm(
    name = "pam-0__1.5.1-20.el9.x86_64",
    sha256 = "090321330a9e9bf608158c22ad2aaaa35b6841d02692159a4e6e0126db52c73f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pam-1.5.1-20.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/090321330a9e9bf608158c22ad2aaaa35b6841d02692159a4e6e0126db52c73f",
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
    name = "policycoreutils-0__3.6-2.1.el9.aarch64",
    sha256 = "93270211cc317bdd44706c3a216ebc8155942e349510a3906f26df0d10328d78",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/policycoreutils-3.6-2.1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/93270211cc317bdd44706c3a216ebc8155942e349510a3906f26df0d10328d78",
    ],
)

rpm(
    name = "policycoreutils-0__3.6-2.1.el9.s390x",
    sha256 = "7ccadb5f8c3ecea0e24447211179c90abbb56cb8d52b97e811137a4588d9ce79",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/policycoreutils-3.6-2.1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/7ccadb5f8c3ecea0e24447211179c90abbb56cb8d52b97e811137a4588d9ce79",
    ],
)

rpm(
    name = "policycoreutils-0__3.6-2.1.el9.x86_64",
    sha256 = "a87874363af6432b1c96b40f8b79b90616df22bff3bd4f9aa39da24f5bddd3e9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/policycoreutils-3.6-2.1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/a87874363af6432b1c96b40f8b79b90616df22bff3bd4f9aa39da24f5bddd3e9",
    ],
)

rpm(
    name = "popt-0__1.18-8.el9.aarch64",
    sha256 = "032427adaa37d2a1c6d2f3cab42ccbdce2c6d9b3c1f3cd91c05a92c99198babb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/popt-1.18-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/032427adaa37d2a1c6d2f3cab42ccbdce2c6d9b3c1f3cd91c05a92c99198babb",
    ],
)

rpm(
    name = "popt-0__1.18-8.el9.s390x",
    sha256 = "b2bc4dbd78a6c3b9458cbc022e80d860fb2c6022fa308604f553289b62cb9511",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/popt-1.18-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/b2bc4dbd78a6c3b9458cbc022e80d860fb2c6022fa308604f553289b62cb9511",
    ],
)

rpm(
    name = "popt-0__1.18-8.el9.x86_64",
    sha256 = "d864419035e99f8bb06f5d1c767608ed81f942cb128a98b590c1dbc4afbd54d4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/popt-1.18-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/d864419035e99f8bb06f5d1c767608ed81f942cb128a98b590c1dbc4afbd54d4",
    ],
)

rpm(
    name = "python3-0__3.9.19-8.el9.aarch64",
    sha256 = "66a2b62975aa1eb3080fb4a95890551ff31057db9635b687812618c93a1ab661",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-3.9.19-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/66a2b62975aa1eb3080fb4a95890551ff31057db9635b687812618c93a1ab661",
    ],
)

rpm(
    name = "python3-0__3.9.19-8.el9.s390x",
    sha256 = "9202a3d75da2124855662427934d5606a1dede45af995557ea28ce1c862297b0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-3.9.19-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/9202a3d75da2124855662427934d5606a1dede45af995557ea28ce1c862297b0",
    ],
)

rpm(
    name = "python3-0__3.9.19-8.el9.x86_64",
    sha256 = "acea3cdc554194669c2dde91c02effe2c4fbf7731a2ff7f949fa6d9391374eec",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-3.9.19-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/acea3cdc554194669c2dde91c02effe2c4fbf7731a2ff7f949fa6d9391374eec",
    ],
)

rpm(
    name = "python3-libs-0__3.9.19-8.el9.aarch64",
    sha256 = "8bb5650b73f40129c2785b7118217e0b895340bb943f35b8df8dd6eb66dfaa77",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-libs-3.9.19-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/8bb5650b73f40129c2785b7118217e0b895340bb943f35b8df8dd6eb66dfaa77",
    ],
)

rpm(
    name = "python3-libs-0__3.9.19-8.el9.s390x",
    sha256 = "226a9899ab0402c1e2835887dcb0136b4b3a4c8b0881d276e9bb14d7eab5e311",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-libs-3.9.19-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/226a9899ab0402c1e2835887dcb0136b4b3a4c8b0881d276e9bb14d7eab5e311",
    ],
)

rpm(
    name = "python3-libs-0__3.9.19-8.el9.x86_64",
    sha256 = "91329d69048a252c8256fd8f9fc01dcc1d899b0156af84841d8cc24c0d01b95f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-libs-3.9.19-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/91329d69048a252c8256fd8f9fc01dcc1d899b0156af84841d8cc24c0d01b95f",
    ],
)

rpm(
    name = "python3-ovirt-engine-sdk4-0__4.6.3-0.1.master.20230324091708.el9.aarch64",
    sha256 = "c5df06df25aff5c99bfcda8373c95e3607e0624be64fe8369870aeb29f1b2fcf",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-9-aarch64/05703348-python-ovirt-engine-sdk4/python3-ovirt-engine-sdk4-4.6.3-0.1.master.20230324091708.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c5df06df25aff5c99bfcda8373c95e3607e0624be64fe8369870aeb29f1b2fcf",
    ],
)

rpm(
    name = "python3-ovirt-engine-sdk4-0__4.6.3-0.1.master.20230324091708.el9.x86_64",
    sha256 = "900c474489b51040051f3ff5eb289372b9f72fd8a39228fc7e6047d63be5b0ba",
    urls = [
        "https://download.copr.fedorainfracloud.org/results/ovirt/ovirt-master-snapshot/centos-stream-9-x86_64/05703348-python-ovirt-engine-sdk4/python3-ovirt-engine-sdk4-4.6.3-0.1.master.20230324091708.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/900c474489b51040051f3ff5eb289372b9f72fd8a39228fc7e6047d63be5b0ba",
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
    name = "python3-pycurl-0__7.43.0.6-8.el9.aarch64",
    sha256 = "4492580e5e98b7e095cf0b7798ff10029afb6b510ddc187933561be55289dd2b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/python3-pycurl-7.43.0.6-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/4492580e5e98b7e095cf0b7798ff10029afb6b510ddc187933561be55289dd2b",
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
    name = "python3-setuptools-wheel-0__53.0.0-13.el9.aarch64",
    sha256 = "a4dfbc2c514f58839d7704acc046eb0fc54cfb670413decebd9641b4d76439e8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-setuptools-wheel-53.0.0-13.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/a4dfbc2c514f58839d7704acc046eb0fc54cfb670413decebd9641b4d76439e8",
    ],
)

rpm(
    name = "python3-setuptools-wheel-0__53.0.0-13.el9.s390x",
    sha256 = "a4dfbc2c514f58839d7704acc046eb0fc54cfb670413decebd9641b4d76439e8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-setuptools-wheel-53.0.0-13.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/a4dfbc2c514f58839d7704acc046eb0fc54cfb670413decebd9641b4d76439e8",
    ],
)

rpm(
    name = "python3-setuptools-wheel-0__53.0.0-13.el9.x86_64",
    sha256 = "a4dfbc2c514f58839d7704acc046eb0fc54cfb670413decebd9641b4d76439e8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-setuptools-wheel-53.0.0-13.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/a4dfbc2c514f58839d7704acc046eb0fc54cfb670413decebd9641b4d76439e8",
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
    name = "qemu-img-17__9.0.0-8.el9.aarch64",
    sha256 = "25fac633ca2fa9fddf5e5c267315f2768b0be063704d6e0ebaa8f3ae912bfbee",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/qemu-img-9.0.0-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/25fac633ca2fa9fddf5e5c267315f2768b0be063704d6e0ebaa8f3ae912bfbee",
    ],
)

rpm(
    name = "qemu-img-17__9.0.0-8.el9.s390x",
    sha256 = "715a07b59920c5d1b5f2b48c4dcd05152f7c39982e2949d5d1136bcceb431347",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/qemu-img-9.0.0-8.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/715a07b59920c5d1b5f2b48c4dcd05152f7c39982e2949d5d1136bcceb431347",
    ],
)

rpm(
    name = "qemu-img-17__9.0.0-8.el9.x86_64",
    sha256 = "15c6a6264bab7d87a38b3e74dc86b6c545e045369224445d04460e95c245f510",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-img-9.0.0-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/15c6a6264bab7d87a38b3e74dc86b6c545e045369224445d04460e95c245f510",
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
    name = "rpm-0__4.16.1.3-34.el9.aarch64",
    sha256 = "924a652300df9cd3033a0e2f29963090ee1125a4f373099ee56e179fed6a623f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/rpm-4.16.1.3-34.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/924a652300df9cd3033a0e2f29963090ee1125a4f373099ee56e179fed6a623f",
    ],
)

rpm(
    name = "rpm-0__4.16.1.3-34.el9.s390x",
    sha256 = "2f94616fb6c12f708fce7b61d224462530eebdbf549b86aaf2b6efe3be85c511",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/rpm-4.16.1.3-34.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/2f94616fb6c12f708fce7b61d224462530eebdbf549b86aaf2b6efe3be85c511",
    ],
)

rpm(
    name = "rpm-0__4.16.1.3-34.el9.x86_64",
    sha256 = "be44e845149e2d1585d14e0330a62b54bf6ffcdcac5aa4443b7701b5de6ed199",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/rpm-4.16.1.3-34.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/be44e845149e2d1585d14e0330a62b54bf6ffcdcac5aa4443b7701b5de6ed199",
    ],
)

rpm(
    name = "rpm-libs-0__4.16.1.3-34.el9.aarch64",
    sha256 = "770978997ed19345fb383beed22b50d27d11d5fdee1e55abb294cf4c73f3a871",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/rpm-libs-4.16.1.3-34.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/770978997ed19345fb383beed22b50d27d11d5fdee1e55abb294cf4c73f3a871",
    ],
)

rpm(
    name = "rpm-libs-0__4.16.1.3-34.el9.s390x",
    sha256 = "51263dbdab8a002394bb41a2a86938548b8a26a02989b58f1fd1cec5373d2ac1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/rpm-libs-4.16.1.3-34.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/51263dbdab8a002394bb41a2a86938548b8a26a02989b58f1fd1cec5373d2ac1",
    ],
)

rpm(
    name = "rpm-libs-0__4.16.1.3-34.el9.x86_64",
    sha256 = "6a7a1900c4ce0e2e285719cb61da9f6d90aea1141f9b7e17cda7eaddb11cf943",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/rpm-libs-4.16.1.3-34.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/6a7a1900c4ce0e2e285719cb61da9f6d90aea1141f9b7e17cda7eaddb11cf943",
    ],
)

rpm(
    name = "rpm-plugin-selinux-0__4.16.1.3-34.el9.aarch64",
    sha256 = "a339b0359aee3613c4cdcde6e96a509cf586b24cf5a3914d7c9b902b9ffd7d5d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/rpm-plugin-selinux-4.16.1.3-34.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/a339b0359aee3613c4cdcde6e96a509cf586b24cf5a3914d7c9b902b9ffd7d5d",
    ],
)

rpm(
    name = "rpm-plugin-selinux-0__4.16.1.3-34.el9.s390x",
    sha256 = "5efc862441f15693232d49554d0d7a9e02b270c1b0a81299108e1a142a0c5eb8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/rpm-plugin-selinux-4.16.1.3-34.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/5efc862441f15693232d49554d0d7a9e02b270c1b0a81299108e1a142a0c5eb8",
    ],
)

rpm(
    name = "rpm-plugin-selinux-0__4.16.1.3-34.el9.x86_64",
    sha256 = "1987ea77ca9b71fdfd251653fbb2ddc98986a0e7c616f063fa06159f3ec72fa3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/rpm-plugin-selinux-4.16.1.3-34.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/1987ea77ca9b71fdfd251653fbb2ddc98986a0e7c616f063fa06159f3ec72fa3",
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
    name = "selinux-policy-0__38.1.44-1.el9.aarch64",
    sha256 = "8294a55f3a71b08aec593e674c27b903ee6f0c35e83139a55f416af7accb10b6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/selinux-policy-38.1.44-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/8294a55f3a71b08aec593e674c27b903ee6f0c35e83139a55f416af7accb10b6",
    ],
)

rpm(
    name = "selinux-policy-0__38.1.44-1.el9.s390x",
    sha256 = "8294a55f3a71b08aec593e674c27b903ee6f0c35e83139a55f416af7accb10b6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/selinux-policy-38.1.44-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/8294a55f3a71b08aec593e674c27b903ee6f0c35e83139a55f416af7accb10b6",
    ],
)

rpm(
    name = "selinux-policy-0__38.1.44-1.el9.x86_64",
    sha256 = "8294a55f3a71b08aec593e674c27b903ee6f0c35e83139a55f416af7accb10b6",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/selinux-policy-38.1.44-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/8294a55f3a71b08aec593e674c27b903ee6f0c35e83139a55f416af7accb10b6",
    ],
)

rpm(
    name = "selinux-policy-targeted-0__38.1.44-1.el9.aarch64",
    sha256 = "66888f5ef55fe723dfd73ac5ce7bef7e84d7276d936fc72a0467a5f7032c6f67",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/selinux-policy-targeted-38.1.44-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/66888f5ef55fe723dfd73ac5ce7bef7e84d7276d936fc72a0467a5f7032c6f67",
    ],
)

rpm(
    name = "selinux-policy-targeted-0__38.1.44-1.el9.s390x",
    sha256 = "66888f5ef55fe723dfd73ac5ce7bef7e84d7276d936fc72a0467a5f7032c6f67",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/selinux-policy-targeted-38.1.44-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/66888f5ef55fe723dfd73ac5ce7bef7e84d7276d936fc72a0467a5f7032c6f67",
    ],
)

rpm(
    name = "selinux-policy-targeted-0__38.1.44-1.el9.x86_64",
    sha256 = "66888f5ef55fe723dfd73ac5ce7bef7e84d7276d936fc72a0467a5f7032c6f67",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/selinux-policy-targeted-38.1.44-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/66888f5ef55fe723dfd73ac5ce7bef7e84d7276d936fc72a0467a5f7032c6f67",
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
    name = "shadow-utils-2__4.9-9.el9.aarch64",
    sha256 = "149ed44d1326239ccaeb7db4b6a78f95c6cb39a27af2b7f70cb7531a1ab5285b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/shadow-utils-4.9-9.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/149ed44d1326239ccaeb7db4b6a78f95c6cb39a27af2b7f70cb7531a1ab5285b",
    ],
)

rpm(
    name = "shadow-utils-2__4.9-9.el9.s390x",
    sha256 = "aeee106d85e7550d08344bffdf2f833a87349722dbc45e777462bea8d9ea2cf2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/shadow-utils-4.9-9.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/aeee106d85e7550d08344bffdf2f833a87349722dbc45e777462bea8d9ea2cf2",
    ],
)

rpm(
    name = "shadow-utils-2__4.9-9.el9.x86_64",
    sha256 = "e3c73fa1856efa23068675638e535a414e97b8927358f4578569f3cdb9ce3ee9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/shadow-utils-4.9-9.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e3c73fa1856efa23068675638e535a414e97b8927358f4578569f3cdb9ce3ee9",
    ],
)

rpm(
    name = "shadow-utils-subid-2__4.9-9.el9.aarch64",
    sha256 = "738bb7e9466b30cca673099e75eb1f332c44c30742e007b54700cce4db955802",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/shadow-utils-subid-4.9-9.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/738bb7e9466b30cca673099e75eb1f332c44c30742e007b54700cce4db955802",
    ],
)

rpm(
    name = "shadow-utils-subid-2__4.9-9.el9.s390x",
    sha256 = "2da0e548767449d98b12c904558be0ce1ce48f240598c90cdf876bcad57f8922",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/shadow-utils-subid-4.9-9.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/2da0e548767449d98b12c904558be0ce1ce48f240598c90cdf876bcad57f8922",
    ],
)

rpm(
    name = "shadow-utils-subid-2__4.9-9.el9.x86_64",
    sha256 = "f3889cf52bb1432583338863ecbf9bf0a2a49df664555a6dda328aed18fa55eb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/shadow-utils-subid-4.9-9.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f3889cf52bb1432583338863ecbf9bf0a2a49df664555a6dda328aed18fa55eb",
    ],
)

rpm(
    name = "slirp4netns-0__1.3.1-1.el9.aarch64",
    sha256 = "b33da013f63e8d75fdb29f1383139b55021f912b3a414ed8b729821328eae5eb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/slirp4netns-1.3.1-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/b33da013f63e8d75fdb29f1383139b55021f912b3a414ed8b729821328eae5eb",
    ],
)

rpm(
    name = "slirp4netns-0__1.3.1-1.el9.s390x",
    sha256 = "9a6933cc841405daeb547dd6200d0b719f73c9cd655e1a39f89b9f42608c478c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/slirp4netns-1.3.1-1.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/9a6933cc841405daeb547dd6200d0b719f73c9cd655e1a39f89b9f42608c478c",
    ],
)

rpm(
    name = "slirp4netns-0__1.3.1-1.el9.x86_64",
    sha256 = "822eea36e2a390042bd22831ce6e6d9e6f596878395df31245354476e6f56f02",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/slirp4netns-1.3.1-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/822eea36e2a390042bd22831ce6e6d9e6f596878395df31245354476e6f56f02",
    ],
)

rpm(
    name = "sqlite-libs-0__3.34.1-7.el9.aarch64",
    sha256 = "f8ffaf1f7ca932f6565754d4c6327f58f41ff4fa7239394b6ad593641dd6ce74",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/sqlite-libs-3.34.1-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f8ffaf1f7ca932f6565754d4c6327f58f41ff4fa7239394b6ad593641dd6ce74",
    ],
)

rpm(
    name = "sqlite-libs-0__3.34.1-7.el9.s390x",
    sha256 = "00136bb1b209b112853b5e2217966276c1cf24c115028afa99f5eb1389984790",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/sqlite-libs-3.34.1-7.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/00136bb1b209b112853b5e2217966276c1cf24c115028afa99f5eb1389984790",
    ],
)

rpm(
    name = "sqlite-libs-0__3.34.1-7.el9.x86_64",
    sha256 = "eddc9570ff3c2f672034888a57eac371e166671fee8300c3c4976324d502a00f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/sqlite-libs-3.34.1-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/eddc9570ff3c2f672034888a57eac371e166671fee8300c3c4976324d502a00f",
    ],
)

rpm(
    name = "systemd-0__252-45.el9.aarch64",
    sha256 = "dacd393563d52fbe7bb88085c921705050fd2d9e2bb5d644046c2519db168d7c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-252-45.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/dacd393563d52fbe7bb88085c921705050fd2d9e2bb5d644046c2519db168d7c",
    ],
)

rpm(
    name = "systemd-0__252-45.el9.s390x",
    sha256 = "95df9a680528f50931a3f0b5af477a3bf0e39efaa012fa7f027d862c4ba13637",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-252-45.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/95df9a680528f50931a3f0b5af477a3bf0e39efaa012fa7f027d862c4ba13637",
    ],
)

rpm(
    name = "systemd-0__252-45.el9.x86_64",
    sha256 = "904cd75260f54da872df80c0aed1e1c4c98def7538b12d4e876edc5c4399a5de",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-252-45.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/904cd75260f54da872df80c0aed1e1c4c98def7538b12d4e876edc5c4399a5de",
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
    name = "systemd-libs-0__252-45.el9.aarch64",
    sha256 = "299c37207d80266245c620a43081f86c21193ca4ab11b20205489e803b632ea1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-libs-252-45.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/299c37207d80266245c620a43081f86c21193ca4ab11b20205489e803b632ea1",
    ],
)

rpm(
    name = "systemd-libs-0__252-45.el9.s390x",
    sha256 = "8d51bc838b3096291db6790661d8ed8356889a0a86527967fc2b4d7578673571",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-libs-252-45.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/8d51bc838b3096291db6790661d8ed8356889a0a86527967fc2b4d7578673571",
    ],
)

rpm(
    name = "systemd-libs-0__252-45.el9.x86_64",
    sha256 = "e98a389a17bd1044aab51707cfbb7940681c508213e04a439fbcf2c4cab475e9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-libs-252-45.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e98a389a17bd1044aab51707cfbb7940681c508213e04a439fbcf2c4cab475e9",
    ],
)

rpm(
    name = "systemd-pam-0__252-45.el9.aarch64",
    sha256 = "4abe383e8f3491461d213c686a13896431b762ba3fe8aa158f7fbf678bf57c83",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-pam-252-45.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/4abe383e8f3491461d213c686a13896431b762ba3fe8aa158f7fbf678bf57c83",
    ],
)

rpm(
    name = "systemd-pam-0__252-45.el9.s390x",
    sha256 = "04ccafe1972e37e42958bfd4ee10f19b2e7bfec47368b3903bf26c70696ecf0f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-pam-252-45.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/04ccafe1972e37e42958bfd4ee10f19b2e7bfec47368b3903bf26c70696ecf0f",
    ],
)

rpm(
    name = "systemd-pam-0__252-45.el9.x86_64",
    sha256 = "de200085975e2776804dff45b55e8045b4901aac9bcf708180ce8770db8bfd74",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-pam-252-45.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/de200085975e2776804dff45b55e8045b4901aac9bcf708180ce8770db8bfd74",
    ],
)

rpm(
    name = "systemd-rpm-macros-0__252-45.el9.aarch64",
    sha256 = "61b2ec228c69c15cd246c357bb1eee1d5d3606ab35c5df325005ab3d3ed00e25",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-rpm-macros-252-45.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/61b2ec228c69c15cd246c357bb1eee1d5d3606ab35c5df325005ab3d3ed00e25",
    ],
)

rpm(
    name = "systemd-rpm-macros-0__252-45.el9.s390x",
    sha256 = "61b2ec228c69c15cd246c357bb1eee1d5d3606ab35c5df325005ab3d3ed00e25",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-rpm-macros-252-45.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/61b2ec228c69c15cd246c357bb1eee1d5d3606ab35c5df325005ab3d3ed00e25",
    ],
)

rpm(
    name = "systemd-rpm-macros-0__252-45.el9.x86_64",
    sha256 = "61b2ec228c69c15cd246c357bb1eee1d5d3606ab35c5df325005ab3d3ed00e25",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-rpm-macros-252-45.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/61b2ec228c69c15cd246c357bb1eee1d5d3606ab35c5df325005ab3d3ed00e25",
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
    name = "tzdata-0__2024a-2.el9.aarch64",
    sha256 = "5f289c6c263a42f354051c3d7875d12f90eba6842a89f1cb2b0ed79c9956ab0d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/tzdata-2024a-2.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/5f289c6c263a42f354051c3d7875d12f90eba6842a89f1cb2b0ed79c9956ab0d",
    ],
)

rpm(
    name = "tzdata-0__2024a-2.el9.s390x",
    sha256 = "5f289c6c263a42f354051c3d7875d12f90eba6842a89f1cb2b0ed79c9956ab0d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/tzdata-2024a-2.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/5f289c6c263a42f354051c3d7875d12f90eba6842a89f1cb2b0ed79c9956ab0d",
    ],
)

rpm(
    name = "tzdata-0__2024a-2.el9.x86_64",
    sha256 = "5f289c6c263a42f354051c3d7875d12f90eba6842a89f1cb2b0ed79c9956ab0d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/tzdata-2024a-2.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/5f289c6c263a42f354051c3d7875d12f90eba6842a89f1cb2b0ed79c9956ab0d",
    ],
)

rpm(
    name = "util-linux-0__2.37.4-20.el9.aarch64",
    sha256 = "76ae6df88815700e14674fd1acd5d2162fd023374c98dc53c000e0f7b574288a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/util-linux-2.37.4-20.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/76ae6df88815700e14674fd1acd5d2162fd023374c98dc53c000e0f7b574288a",
    ],
)

rpm(
    name = "util-linux-0__2.37.4-20.el9.s390x",
    sha256 = "fd814b3b94ffe1f905a49308c8d5863b13d865ba48dcca68d6d2b2d09677d610",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/util-linux-2.37.4-20.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/fd814b3b94ffe1f905a49308c8d5863b13d865ba48dcca68d6d2b2d09677d610",
    ],
)

rpm(
    name = "util-linux-0__2.37.4-20.el9.x86_64",
    sha256 = "5011faf8c26d7402f1f0438687e3393b1d6a64eaa2ac7f30c1dcf472e8635ef5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/util-linux-2.37.4-20.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/5011faf8c26d7402f1f0438687e3393b1d6a64eaa2ac7f30c1dcf472e8635ef5",
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
    name = "util-linux-core-0__2.37.4-20.el9.aarch64",
    sha256 = "7f452299af4a3e656fc3aa59a3ce91f61ce1a57e9753a5fbbc5886db5e5fe36a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/util-linux-core-2.37.4-20.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/7f452299af4a3e656fc3aa59a3ce91f61ce1a57e9753a5fbbc5886db5e5fe36a",
    ],
)

rpm(
    name = "util-linux-core-0__2.37.4-20.el9.s390x",
    sha256 = "5c751a55026449698454e4de778bfbb5acb5d890e8fdace4a0d9826ad9423108",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/util-linux-core-2.37.4-20.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/5c751a55026449698454e4de778bfbb5acb5d890e8fdace4a0d9826ad9423108",
    ],
)

rpm(
    name = "util-linux-core-0__2.37.4-20.el9.x86_64",
    sha256 = "e4df98c254564404ae8750d6105290dedf18593ce53654b66ed9cb170bbfbcc7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/util-linux-core-2.37.4-20.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e4df98c254564404ae8750d6105290dedf18593ce53654b66ed9cb170bbfbcc7",
    ],
)

rpm(
    name = "vim-minimal-2__8.2.2637-15.el9.aarch64",
    sha256 = "14136f426b9425d7c66bc6a5cace746b84b0bcf436e58144d782d993998da7da",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/vim-minimal-8.2.2637-15.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/14136f426b9425d7c66bc6a5cace746b84b0bcf436e58144d782d993998da7da",
    ],
)

rpm(
    name = "vim-minimal-2__8.2.2637-15.el9.x86_64",
    sha256 = "062a1b85ecad3a9ea41e39f268f5660c1e6262999339fc18e77c797101b96461",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/vim-minimal-8.2.2637-15.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/062a1b85ecad3a9ea41e39f268f5660c1e6262999339fc18e77c797101b96461",
    ],
)

rpm(
    name = "vim-minimal-2__8.2.2637-21.el9.aarch64",
    sha256 = "2a06e6863cc4d8c699b727424f2e0a06c75f5c8265cb2bc576242054d1bff444",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/vim-minimal-8.2.2637-21.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/2a06e6863cc4d8c699b727424f2e0a06c75f5c8265cb2bc576242054d1bff444",
    ],
)

rpm(
    name = "vim-minimal-2__8.2.2637-21.el9.s390x",
    sha256 = "a04988c53eea9735bb2eb5106e7e2215f5a355af2c33dfbe20c643c811b9176f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/vim-minimal-8.2.2637-21.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/a04988c53eea9735bb2eb5106e7e2215f5a355af2c33dfbe20c643c811b9176f",
    ],
)

rpm(
    name = "vim-minimal-2__8.2.2637-21.el9.x86_64",
    sha256 = "1b15304790e4b2e7d4ff378b7bf0363b6ecb1c852fc42f984267296538de0c16",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/vim-minimal-8.2.2637-21.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/1b15304790e4b2e7d4ff378b7bf0363b6ecb1c852fc42f984267296538de0c16",
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
    name = "yajl-0__2.1.0-22.el9.aarch64",
    sha256 = "5f099ce8836377f6aba662e5835cc500b2e8f29cd8c9b56b22df7c564f7d209c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/yajl-2.1.0-22.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/5f099ce8836377f6aba662e5835cc500b2e8f29cd8c9b56b22df7c564f7d209c",
    ],
)

rpm(
    name = "yajl-0__2.1.0-22.el9.s390x",
    sha256 = "45c55fec973903149868133e4416265694f4589643337639211cac0db239db42",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/yajl-2.1.0-22.el9.s390x.rpm",
        "https://storage.googleapis.com/builddeps/45c55fec973903149868133e4416265694f4589643337639211cac0db239db42",
    ],
)

rpm(
    name = "yajl-0__2.1.0-22.el9.x86_64",
    sha256 = "907156eb13e2120402287396f92b7589515ab0cba802b99c3835dd36f6a12cdf",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/yajl-2.1.0-22.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/907156eb13e2120402287396f92b7589515ab0cba802b99c3835dd36f6a12cdf",
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
