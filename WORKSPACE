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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/acl-2.3.1-4.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/alternatives-1.24-1.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/audit-libs-3.1.5-1.el9.aarch64.rpm"],
)

rpm(
    name = "audit-libs-0__3.1.5-1.el9.s390x",
    sha256 = "090ef1e4057d3235a050ad72728f40752faa6958a7f3ee6ebd0cd43e5f97d026",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/audit-libs-3.1.5-1.el9.s390x.rpm"],
)

rpm(
    name = "audit-libs-0__3.1.5-1.el9.x86_64",
    sha256 = "e1998c3847956ad86d846f8b857e5382897ef2f444b4a2ef8e82a0cb8b1aa1ad",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/audit-libs-3.1.5-1.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/basesystem-11-13.el9.noarch.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/bash-5.1.8-9.el9.s390x.rpm"],
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
    name = "buildah-2__1.37.1-1.el9.aarch64",
    sha256 = "47015bda24d72e4ed568007c740ee0f49f8af2075fa21c0e3a343fc1c6a6adbf",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/buildah-1.37.1-1.el9.aarch64.rpm"],
)

rpm(
    name = "buildah-2__1.37.1-1.el9.s390x",
    sha256 = "e610267cfe2e0d07a30f6c01cc66bfeed854bfde67d9df2e287b249a65c693fb",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/buildah-1.37.1-1.el9.s390x.rpm"],
)

rpm(
    name = "buildah-2__1.37.1-1.el9.x86_64",
    sha256 = "5e1658d41021a4ea9ebf23dc3d42803f8e58bf87dd65fd430e0fc9acb0155f1e",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/buildah-1.37.1-1.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/bzip2-libs-1.0.8-8.el9.s390x.rpm"],
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
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/ca-certificates-2020.2.50-94.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/3099471d984fb7d9e1cf42406eb08c154b34b8560742ed1f5eb9139f059c2d09",
    ],
)

rpm(
    name = "ca-certificates-0__2020.2.50-94.el9.x86_64",
    sha256 = "3099471d984fb7d9e1cf42406eb08c154b34b8560742ed1f5eb9139f059c2d09",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ca-certificates-2020.2.50-94.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/3099471d984fb7d9e1cf42406eb08c154b34b8560742ed1f5eb9139f059c2d09",
    ],
)

rpm(
    name = "ca-certificates-0__2024.2.69_v8.0.303-91.3.el9.aarch64",
    sha256 = "11edeca836cc075a438b97ad25e04cc201c894f8ed8a78dcd7ebb7885ae441cd",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/ca-certificates-2024.2.69_v8.0.303-91.3.el9.noarch.rpm"],
)

rpm(
    name = "ca-certificates-0__2024.2.69_v8.0.303-91.3.el9.s390x",
    sha256 = "11edeca836cc075a438b97ad25e04cc201c894f8ed8a78dcd7ebb7885ae441cd",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/ca-certificates-2024.2.69_v8.0.303-91.3.el9.noarch.rpm"],
)

rpm(
    name = "ca-certificates-0__2024.2.69_v8.0.303-91.3.el9.x86_64",
    sha256 = "11edeca836cc075a438b97ad25e04cc201c894f8ed8a78dcd7ebb7885ae441cd",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ca-certificates-2024.2.69_v8.0.303-91.3.el9.noarch.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-gpg-keys-9.0-26.el9.noarch.rpm"],
)

rpm(
    name = "centos-gpg-keys-0__9.0-26.el9.s390x",
    sha256 = "8d601d9f96356a200ad6ed8e5cb49bbac4aa3c4b762d10a23e11311daa5711ca",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/centos-gpg-keys-9.0-26.el9.noarch.rpm"],
)

rpm(
    name = "centos-gpg-keys-0__9.0-26.el9.x86_64",
    sha256 = "8d601d9f96356a200ad6ed8e5cb49bbac4aa3c4b762d10a23e11311daa5711ca",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-gpg-keys-9.0-26.el9.noarch.rpm"],
)

rpm(
    name = "centos-logos-httpd-0__90.8-1.el9.aarch64",
    sha256 = "97173a26ab3315860acfc806a4547ac967e5bf1d19246de485595aaf8065d13c",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/centos-logos-httpd-90.8-1.el9.noarch.rpm"],
)

rpm(
    name = "centos-logos-httpd-0__90.8-1.el9.s390x",
    sha256 = "97173a26ab3315860acfc806a4547ac967e5bf1d19246de485595aaf8065d13c",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/centos-logos-httpd-90.8-1.el9.noarch.rpm"],
)

rpm(
    name = "centos-logos-httpd-0__90.8-1.el9.x86_64",
    sha256 = "97173a26ab3315860acfc806a4547ac967e5bf1d19246de485595aaf8065d13c",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/centos-logos-httpd-90.8-1.el9.noarch.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-stream-release-9.0-26.el9.noarch.rpm"],
)

rpm(
    name = "centos-stream-release-0__9.0-26.el9.s390x",
    sha256 = "3d60dc8ed86717f68394fc7468b8024557c43ac2ad97b8e40911d056cd6d64d3",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/centos-stream-release-9.0-26.el9.noarch.rpm"],
)

rpm(
    name = "centos-stream-release-0__9.0-26.el9.x86_64",
    sha256 = "3d60dc8ed86717f68394fc7468b8024557c43ac2ad97b8e40911d056cd6d64d3",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-stream-release-9.0-26.el9.noarch.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-stream-repos-9.0-26.el9.noarch.rpm"],
)

rpm(
    name = "centos-stream-repos-0__9.0-26.el9.s390x",
    sha256 = "eb3b55a5cf0e1a93a91cd2d39035bd1754b46f69ff3d062b3331e765b2345035",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/centos-stream-repos-9.0-26.el9.noarch.rpm"],
)

rpm(
    name = "centos-stream-repos-0__9.0-26.el9.x86_64",
    sha256 = "eb3b55a5cf0e1a93a91cd2d39035bd1754b46f69ff3d062b3331e765b2345035",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-stream-repos-9.0-26.el9.noarch.rpm"],
)

rpm(
    name = "containers-common-2__1-91.el9.aarch64",
    sha256 = "e9802e5400e614c0ac41b3d6d9b7e4ecc8ea02aef0e7b2be064e93ed68b4da02",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/containers-common-1-91.el9.aarch64.rpm"],
)

rpm(
    name = "containers-common-2__1-91.el9.s390x",
    sha256 = "01c219b60f01e3c1eb9e35ccd9d8cc189d9433b0a84a9b35b1b2041ee75a8cbf",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/containers-common-1-91.el9.s390x.rpm"],
)

rpm(
    name = "containers-common-2__1-91.el9.x86_64",
    sha256 = "d8ea0cdba33f4cdb7b9e0fa34cb5c61323146ffaa1e3ea311bbde06d93c9d265",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/containers-common-1-91.el9.x86_64.rpm"],
)

rpm(
    name = "coreutils-single-0__8.32-31.el9.aarch64",
    sha256 = "e2d2e94d4322f41cb7331b0e8c23f937b08f37514826d78fb4ed4d1bbea3ef5b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/coreutils-single-8.32-31.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/e2d2e94d4322f41cb7331b0e8c23f937b08f37514826d78fb4ed4d1bbea3ef5b",
    ],
)

rpm(
    name = "coreutils-single-0__8.32-31.el9.x86_64",
    sha256 = "fcae4e00df1cb3d0eb214d166045150aede7262559bd03fc585610fe1ea59c08",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/coreutils-single-8.32-31.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/fcae4e00df1cb3d0eb214d166045150aede7262559bd03fc585610fe1ea59c08",
    ],
)

rpm(
    name = "coreutils-single-0__8.32-35.el9.aarch64",
    sha256 = "bb97628ca49734e508c72631251b9a6737f47d85961a75f99c7ddebdf121443f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/coreutils-single-8.32-35.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/bb97628ca49734e508c72631251b9a6737f47d85961a75f99c7ddebdf121443f",
    ],
)

rpm(
    name = "coreutils-single-0__8.32-35.el9.s390x",
    sha256 = "4751affb09bc84155f12caa36692a6f5c3733f10dd6bea40d59bfa2f892226e8",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/coreutils-single-8.32-35.el9.s390x.rpm"],
)

rpm(
    name = "coreutils-single-0__8.32-35.el9.x86_64",
    sha256 = "09c1828de7a4a5c787eaa43204cdaa8d5ec52d7b11a29f911c2e960c554c19b4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/coreutils-single-8.32-35.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/09c1828de7a4a5c787eaa43204cdaa8d5ec52d7b11a29f911c2e960c554c19b4",
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/cracklib-2.9.6-27.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/cracklib-dicts-2.9.6-27.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/crun-1.16.1-1.el9.aarch64.rpm"],
)

rpm(
    name = "crun-0__1.16.1-1.el9.s390x",
    sha256 = "527a0df933b6324ecec08338bd6275be6cf6217e5225724dd22afedff11da7e5",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/crun-1.16.1-1.el9.s390x.rpm"],
)

rpm(
    name = "crun-0__1.16.1-1.el9.x86_64",
    sha256 = "2375a3674fb8e80e14b4722926d71770df1e65bfb5c79f25fc55d74c6fa2331a",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/crun-1.16.1-1.el9.x86_64.rpm"],
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
    name = "crypto-policies-0__20240815-1.gite217f03.el9.aarch64",
    sha256 = "aab37166117376d6f07af843bab57c2c048752f1b843fa06f6ba6d15924bf35d",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/crypto-policies-20240815-1.gite217f03.el9.noarch.rpm"],
)

rpm(
    name = "crypto-policies-0__20240815-1.gite217f03.el9.s390x",
    sha256 = "aab37166117376d6f07af843bab57c2c048752f1b843fa06f6ba6d15924bf35d",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/crypto-policies-20240815-1.gite217f03.el9.noarch.rpm"],
)

rpm(
    name = "crypto-policies-0__20240815-1.gite217f03.el9.x86_64",
    sha256 = "aab37166117376d6f07af843bab57c2c048752f1b843fa06f6ba6d15924bf35d",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/crypto-policies-20240815-1.gite217f03.el9.noarch.rpm"],
)

rpm(
    name = "crypto-policies-scripts-0__20240815-1.gite217f03.el9.aarch64",
    sha256 = "f7c39fbef4705a6f734a0752700d8bd07d5637512a5dde3177114c4753997ba2",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/crypto-policies-scripts-20240815-1.gite217f03.el9.noarch.rpm"],
)

rpm(
    name = "crypto-policies-scripts-0__20240815-1.gite217f03.el9.s390x",
    sha256 = "f7c39fbef4705a6f734a0752700d8bd07d5637512a5dde3177114c4753997ba2",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/crypto-policies-scripts-20240815-1.gite217f03.el9.noarch.rpm"],
)

rpm(
    name = "crypto-policies-scripts-0__20240815-1.gite217f03.el9.x86_64",
    sha256 = "f7c39fbef4705a6f734a0752700d8bd07d5637512a5dde3177114c4753997ba2",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/crypto-policies-scripts-20240815-1.gite217f03.el9.noarch.rpm"],
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
    name = "curl-0__7.76.1-29.el9.aarch64",
    sha256 = "fa2d40938747f7c10e04a9ec91e2d639cbda03fde40bc4c1e4b60b23e80451a2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/curl-7.76.1-29.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/fa2d40938747f7c10e04a9ec91e2d639cbda03fde40bc4c1e4b60b23e80451a2",
    ],
)

rpm(
    name = "curl-0__7.76.1-29.el9.s390x",
    sha256 = "78703546054b3adf94c67b22369d9189f77bffd8aeac08f303670bd7336c9484",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/curl-7.76.1-29.el9.s390x.rpm"],
)

rpm(
    name = "curl-0__7.76.1-29.el9.x86_64",
    sha256 = "24d4fe9ec6a184f9fc517a8037078dec9ca6b8424d721c34b9b9733df67af32a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/curl-7.76.1-29.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/24d4fe9ec6a184f9fc517a8037078dec9ca6b8424d721c34b9b9733df67af32a",
    ],
)

rpm(
    name = "curl-minimal-0__7.76.1-29.el9.aarch64",
    sha256 = "231cf035ef2ee6565e39bf84c4f6875b87efb84fa24079612a1d94ccb10d45bd",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/curl-minimal-7.76.1-29.el9.aarch64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/cyrus-sasl-lib-2.1.27-21.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/dbus-1.12.20-8.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/dbus-broker-28-7.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/dbus-common-1.12.20-8.el9.noarch.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/diffutils-3.7-12.el9.aarch64.rpm"],
)

rpm(
    name = "diffutils-0__3.7-12.el9.s390x",
    sha256 = "e0f62f72c6d24e0507fa16c23bb74ece2704aabfb902c3649c57dad090f0c1ae",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/diffutils-3.7-12.el9.s390x.rpm"],
)

rpm(
    name = "diffutils-0__3.7-12.el9.x86_64",
    sha256 = "fdebefc46badf2e700e00582041a0e5f5183dd4fdc04badfe47c91f030cea0ce",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/diffutils-3.7-12.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/expat-2.5.0-2.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/filesystem-3.16-5.el9.aarch64.rpm"],
)

rpm(
    name = "filesystem-0__3.16-5.el9.s390x",
    sha256 = "67a733fe124cda9da89f6946757800c0fe73b918a477adcf67dfbef15c995729",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/filesystem-3.16-5.el9.s390x.rpm"],
)

rpm(
    name = "filesystem-0__3.16-5.el9.x86_64",
    sha256 = "da7750fc31248ecc606016391c3f570e1abe7422f812b29a49d830c71884e6dc",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/filesystem-3.16-5.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gawk-5.1.0-6.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gdbm-libs-1.23-1.el9.aarch64.rpm"],
)

rpm(
    name = "gdbm-libs-1__1.23-1.el9.s390x",
    sha256 = "29c9ab72536be72b9c78285ef12117633cf3e2dfd18757bcf7587cd94eb9e055",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gdbm-libs-1.23-1.el9.s390x.rpm"],
)

rpm(
    name = "gdbm-libs-1__1.23-1.el9.x86_64",
    sha256 = "cada66331cc07a4f8a0701fc1ad13c346913a0d6f913e35c0257a68b6a1e6ce0",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gdbm-libs-1.23-1.el9.x86_64.rpm"],
)

rpm(
    name = "glib2-0__2.68.4-15.el9.aarch64",
    sha256 = "a2baecdb746f3312a5b37d4bc139182195b31eb4a1cb16b9b84db00a1e640042",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glib2-2.68.4-15.el9.aarch64.rpm"],
)

rpm(
    name = "glib2-0__2.68.4-15.el9.s390x",
    sha256 = "34a706adcd651397a5f6681c4cb4973aac48df23cf00c9457946d1f264376960",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glib2-2.68.4-15.el9.s390x.rpm"],
)

rpm(
    name = "glib2-0__2.68.4-15.el9.x86_64",
    sha256 = "85f10ac062e7c1dc1c4becd0f5287f7e5e0f17267e139a59cfb5eabf4632713e",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glib2-2.68.4-15.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-2.34-120.el9.aarch64.rpm"],
)

rpm(
    name = "glibc-0__2.34-120.el9.s390x",
    sha256 = "8cceb8b41ffc8653c0c361b0b075b307c9e7feba65c5e3fa724d34afc210da53",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glibc-2.34-120.el9.s390x.rpm"],
)

rpm(
    name = "glibc-0__2.34-120.el9.x86_64",
    sha256 = "5a8784660b6fdcb20a6bbd4203f306495bf1d8b976b0a59d19dca18f0589a5a1",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-2.34-120.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-common-2.34-120.el9.aarch64.rpm"],
)

rpm(
    name = "glibc-common-0__2.34-120.el9.s390x",
    sha256 = "832d2698ee7451bad7f2dbd193acc362c3d52ecf4c6e0865e54c8402689513e0",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glibc-common-2.34-120.el9.s390x.rpm"],
)

rpm(
    name = "glibc-common-0__2.34-120.el9.x86_64",
    sha256 = "b72de8b46f0a7df5b161823027581c4aae642283c3fda30954dd06fccda27188",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-common-2.34-120.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-minimal-langpack-2.34-120.el9.aarch64.rpm"],
)

rpm(
    name = "glibc-minimal-langpack-0__2.34-120.el9.s390x",
    sha256 = "374644269bb52306fc5fb8430ece556d307adf62c5662133bda5777dfee6d105",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/glibc-minimal-langpack-2.34-120.el9.s390x.rpm"],
)

rpm(
    name = "glibc-minimal-langpack-0__2.34-120.el9.x86_64",
    sha256 = "d345bd69726071bbb5e68734fc7d93dc43c399181e1442aaafd7ae193632d28b",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-minimal-langpack-2.34-120.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gmp-6.2.0-13.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gnupg2-2.3.3-4.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gnutls-3.8.3-4.el9.aarch64.rpm"],
)

rpm(
    name = "gnutls-0__3.8.3-4.el9.s390x",
    sha256 = "f71b6727e720d44781702bb37815cddbe7f0aab173174bbc7f555d88ca00160e",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gnutls-3.8.3-4.el9.s390x.rpm"],
)

rpm(
    name = "gnutls-0__3.8.3-4.el9.x86_64",
    sha256 = "91e1e46e6f315445e715184237a69f4152359efa1a9ae54cc0524b9616d0741f",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gnutls-3.8.3-4.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gpgme-1.15.1-6.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/grep-3.6-5.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/gzip-1.12-1.el9.s390x.rpm"],
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
    name = "iptables-libs-0__1.8.10-4.el9.aarch64",
    sha256 = "73d20e7dd85e9695fd16981d299f2d3055e946f1902c69cf9aa209d6c11ad89f",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/iptables-libs-1.8.10-4.el9.aarch64.rpm"],
)

rpm(
    name = "iptables-libs-0__1.8.10-4.el9.s390x",
    sha256 = "f2eaade068338167f7854e73aa0ff4895001f967177741148fab7e9a33a6404d",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/iptables-libs-1.8.10-4.el9.s390x.rpm"],
)

rpm(
    name = "iptables-libs-0__1.8.10-4.el9.x86_64",
    sha256 = "2d7792078d666e2e6cdf603c47e6985377b50e7b4c248264b9b0ba42e5c21d6d",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/iptables-libs-1.8.10-4.el9.x86_64.rpm"],
)

rpm(
    name = "iptables-nft-0__1.8.10-4.el9.aarch64",
    sha256 = "0ab6cb42b1380d68c131747abc9d23d6560627a6e3cf5d52ddd06f8256c85f83",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/iptables-nft-1.8.10-4.el9.aarch64.rpm"],
)

rpm(
    name = "iptables-nft-0__1.8.10-4.el9.s390x",
    sha256 = "0d2c96bbc944ac841f9c7de47b0fd543af98eaae6b43140f0afa666adc107906",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/iptables-nft-1.8.10-4.el9.s390x.rpm"],
)

rpm(
    name = "iptables-nft-0__1.8.10-4.el9.x86_64",
    sha256 = "78bc2997be7f28744cf09e8f1c50dad059007166fec578f04c16a1933fe6e8f1",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/iptables-nft-1.8.10-4.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/jansson-2.14-1.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/keyutils-libs-1.6.3-1.el9.s390x.rpm"],
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
    name = "kmod-libs-0__28-9.el9.aarch64",
    sha256 = "0e51fa74611d31585fb4e665fc4b24b0ff300821d109b3e0116ccdfc54c04789",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/kmod-libs-28-9.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/0e51fa74611d31585fb4e665fc4b24b0ff300821d109b3e0116ccdfc54c04789",
    ],
)

rpm(
    name = "kmod-libs-0__28-9.el9.s390x",
    sha256 = "88bc8e78b7d093f24177d5939384878142c3b4bcd9c0983e7f8b493f5d3a87ee",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/kmod-libs-28-9.el9.s390x.rpm"],
)

rpm(
    name = "kmod-libs-0__28-9.el9.x86_64",
    sha256 = "319957f8f3abe9b05b4aca442a3c633b36c8974e2dbd87f31ec66885f66e1b88",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/kmod-libs-28-9.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/319957f8f3abe9b05b4aca442a3c633b36c8974e2dbd87f31ec66885f66e1b88",
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/krb5-libs-1.21.1-3.el9.aarch64.rpm"],
)

rpm(
    name = "krb5-libs-0__1.21.1-3.el9.s390x",
    sha256 = "546f00d4eab306662ce8ce93a43a00f7f2c3e46dfe84a57df4402fadf14b3d9e",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/krb5-libs-1.21.1-3.el9.s390x.rpm"],
)

rpm(
    name = "krb5-libs-0__1.21.1-3.el9.x86_64",
    sha256 = "76f4a5d0de611ad58a320c11dee853a7944b48f8adc4922a0e7ac7ee476d9716",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/krb5-libs-1.21.1-3.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libacl-2.3.1-4.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libaio-0.3.111-13.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libarchive-3.5.3-4.el9.aarch64.rpm"],
)

rpm(
    name = "libarchive-0__3.5.3-4.el9.s390x",
    sha256 = "f95a05acd33d6f63a43ac2b065c45a3d2c9ef1923ec80d3a33946501dde0e751",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libarchive-3.5.3-4.el9.s390x.rpm"],
)

rpm(
    name = "libarchive-0__3.5.3-4.el9.x86_64",
    sha256 = "4c53176eafd8c449aef704b8fbc2d5401bb7d2ea0a67961956f318f2e9a2c7a4",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libarchive-3.5.3-4.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libassuan-2.5.5-3.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libattr-2.5.1-3.el9.s390x.rpm"],
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
    name = "libblkid-0__2.37.4-18.el9.aarch64",
    sha256 = "bb4cd8f1748f2ecf837017dada4c52ef60dc896dc504aef3378e016d5cab57b4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libblkid-2.37.4-18.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/bb4cd8f1748f2ecf837017dada4c52ef60dc896dc504aef3378e016d5cab57b4",
    ],
)

rpm(
    name = "libblkid-0__2.37.4-18.el9.s390x",
    sha256 = "eac8c0829e2a7b496f368891840a71b38ddd175e870b2d1d8b056e3d0381b21a",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libblkid-2.37.4-18.el9.s390x.rpm"],
)

rpm(
    name = "libblkid-0__2.37.4-18.el9.x86_64",
    sha256 = "f6dcef2625cb6910451ba7c7a8034ea0f73d06a9d7741e0373fe088fe6cf72dd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libblkid-2.37.4-18.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f6dcef2625cb6910451ba7c7a8034ea0f73d06a9d7741e0373fe088fe6cf72dd",
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libcap-2.48-9.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libcap-ng-0.8.2-7.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libcom_err-1.46.5-5.el9.s390x.rpm"],
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
    name = "libcurl-minimal-0__7.76.1-29.el9.aarch64",
    sha256 = "ab067d463a4f534d6180ec03dd597da0eeda8cd6a1a3882c2d8c3fb31be10533",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libcurl-minimal-7.76.1-29.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/ab067d463a4f534d6180ec03dd597da0eeda8cd6a1a3882c2d8c3fb31be10533",
    ],
)

rpm(
    name = "libcurl-minimal-0__7.76.1-29.el9.s390x",
    sha256 = "de8e5a00fed0d7ac020cf3c0ec2a6ea8e61f147ddf6b75824e8cfb8673aba250",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libcurl-minimal-7.76.1-29.el9.s390x.rpm"],
)

rpm(
    name = "libcurl-minimal-0__7.76.1-29.el9.x86_64",
    sha256 = "c304626e349ed7953b4962705bf2903cd96bf5d3feb5eb5d14a68c4b7e2ff093",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcurl-minimal-7.76.1-29.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c304626e349ed7953b4962705bf2903cd96bf5d3feb5eb5d14a68c4b7e2ff093",
    ],
)

rpm(
    name = "libdb-0__5.3.28-54.el9.aarch64",
    sha256 = "fdba5b07c422da16412014be63b29407423024a8c0aa367d0577c73600c65c93",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libdb-5.3.28-54.el9.aarch64.rpm"],
)

rpm(
    name = "libdb-0__5.3.28-54.el9.s390x",
    sha256 = "2dcb7dc38dff10884a00276da793b80f177327c49b4ebb81132575ebd09ed686",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libdb-5.3.28-54.el9.s390x.rpm"],
)

rpm(
    name = "libdb-0__5.3.28-54.el9.x86_64",
    sha256 = "3a54bfa30eab6a76d11ce543db49f697273aad0dbb54c20668a62e36a68aa32b",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libdb-5.3.28-54.el9.x86_64.rpm"],
)

rpm(
    name = "libeconf-0__0.4.1-4.el9.aarch64",
    sha256 = "c221c71bfd8f6692e305a4e0c0025c4789ab04661c11a1a18c34c3f873f1276f",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libeconf-0.4.1-4.el9.aarch64.rpm"],
)

rpm(
    name = "libeconf-0__0.4.1-4.el9.s390x",
    sha256 = "1ee2d8e7b48a5e9616c1f7a5b019e0aa054a80b5962d972104d78d095b2e926d",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libeconf-0.4.1-4.el9.s390x.rpm"],
)

rpm(
    name = "libeconf-0__0.4.1-4.el9.x86_64",
    sha256 = "ed519cc2e9031e2bf03275b28c7cca6520ae916d0a7edbbc69f327c1b70ed6cc",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libeconf-0.4.1-4.el9.x86_64.rpm"],
)

rpm(
    name = "libevent-0__2.1.12-6.el9.aarch64",
    sha256 = "5ff00c047204190e3b2ee19f81d644c8f82ea7e8d1f36fdaaf6483f0fa3b3339",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libevent-2.1.12-6.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/5ff00c047204190e3b2ee19f81d644c8f82ea7e8d1f36fdaaf6483f0fa3b3339",
    ],
)

rpm(
    name = "libevent-0__2.1.12-6.el9.s390x",
    sha256 = "3a93804c6662e4ed7a4f8fffe66c1f1a0148e8f2269713affd710e78ba4ba680",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libevent-2.1.12-6.el9.s390x.rpm"],
)

rpm(
    name = "libevent-0__2.1.12-6.el9.x86_64",
    sha256 = "82179f6f214ddf523e143c16c3474ccf8832551c6305faf89edfbd83b3424d48",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libevent-2.1.12-6.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/82179f6f214ddf523e143c16c3474ccf8832551c6305faf89edfbd83b3424d48",
    ],
)

rpm(
    name = "libfdisk-0__2.37.4-18.el9.aarch64",
    sha256 = "691c82ee8a6fcebd52ed5ae3538cf997bfa3002260ef076c3bef3ff0289d72cf",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libfdisk-2.37.4-18.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/691c82ee8a6fcebd52ed5ae3538cf997bfa3002260ef076c3bef3ff0289d72cf",
    ],
)

rpm(
    name = "libfdisk-0__2.37.4-18.el9.s390x",
    sha256 = "c121b308138c0ca6667f56d65eea86bdfdf917f496ee6ad57cedb8653abd1662",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libfdisk-2.37.4-18.el9.s390x.rpm"],
)

rpm(
    name = "libfdisk-0__2.37.4-18.el9.x86_64",
    sha256 = "051951603ec09ab292198cb96d3377fb6353534d6572f3a21053de0f48ab9430",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libfdisk-2.37.4-18.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/051951603ec09ab292198cb96d3377fb6353534d6572f3a21053de0f48ab9430",
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libffi-3.4.2-8.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libgcc-11.5.0-2.el9.aarch64.rpm"],
)

rpm(
    name = "libgcc-0__11.5.0-2.el9.s390x",
    sha256 = "ac6c003d9fe74072a7b3b34fc33fe649b5ec98cb0d8f08efb5239002cbd578c8",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libgcc-11.5.0-2.el9.s390x.rpm"],
)

rpm(
    name = "libgcc-0__11.5.0-2.el9.x86_64",
    sha256 = "ff344c9aaf0ef773230411b64e58d35d372314641b69113229afa6c539aa270a",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libgcc-11.5.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "libgcrypt-0__1.10.0-11.el9.aarch64",
    sha256 = "932bfe51b207e2ad8a0bd2b89e2fb33df73f3993586aaa4cc60576f57795e4db",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libgcrypt-1.10.0-11.el9.aarch64.rpm"],
)

rpm(
    name = "libgcrypt-0__1.10.0-11.el9.s390x",
    sha256 = "cf30c86fc1a18f504d639d3cbcf9e431af1ea639e6a5e7db1f6d30b763dd51a8",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libgcrypt-1.10.0-11.el9.s390x.rpm"],
)

rpm(
    name = "libgcrypt-0__1.10.0-11.el9.x86_64",
    sha256 = "0323a74a5ad27bc3dc4ac4e9565825f37dc58b2a4800adbf33f767fa7a267c35",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libgcrypt-1.10.0-11.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libgpg-error-1.42-5.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libidn2-2.3.0-7.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libksba-1.5.1-7.el9.aarch64.rpm"],
)

rpm(
    name = "libksba-0__1.5.1-7.el9.s390x",
    sha256 = "10e17f1f886f90259f915e855389f3e3852fddd52be35110ebe0d0f4b9b4f51a",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libksba-1.5.1-7.el9.s390x.rpm"],
)

rpm(
    name = "libksba-0__1.5.1-7.el9.x86_64",
    sha256 = "8c2a4312f0a700286e1c3630f62dba6d06e7a4c07a17182ca97f2d40d0b4c6a0",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libksba-1.5.1-7.el9.x86_64.rpm"],
)

rpm(
    name = "libmnl-0__1.0.4-16.el9.aarch64",
    sha256 = "c4d87c6439aa762891b024c0213df47af50e5b0683ffd827013bd02882d7d9b3",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libmnl-1.0.4-16.el9.aarch64.rpm"],
)

rpm(
    name = "libmnl-0__1.0.4-16.el9.s390x",
    sha256 = "344f21dedaaad1ddc5279e31a4dafd9354662a61f010249d86a424c903c4415a",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libmnl-1.0.4-16.el9.s390x.rpm"],
)

rpm(
    name = "libmnl-0__1.0.4-16.el9.x86_64",
    sha256 = "e60f3be453b44ea04bb596594963be1e1b3f4377f87b4ff923d612eae15740ce",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libmnl-1.0.4-16.el9.x86_64.rpm"],
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
    name = "libmount-0__2.37.4-18.el9.aarch64",
    sha256 = "47ff34986c31df37b782c62c0621d35954be2334bfd92a90376467b4376119fd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libmount-2.37.4-18.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/47ff34986c31df37b782c62c0621d35954be2334bfd92a90376467b4376119fd",
    ],
)

rpm(
    name = "libmount-0__2.37.4-18.el9.s390x",
    sha256 = "1e87191e999ea6ef99a7cf8eb65c6ddf6657c66a68eab46e0cd26a2cf463c28b",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libmount-2.37.4-18.el9.s390x.rpm"],
)

rpm(
    name = "libmount-0__2.37.4-18.el9.x86_64",
    sha256 = "54237ed9c05e3e307f0eb94a4ccd27c6790e8e05e39d3eafd76fd42a344aaa1d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libmount-2.37.4-18.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/54237ed9c05e3e307f0eb94a4ccd27c6790e8e05e39d3eafd76fd42a344aaa1d",
    ],
)

rpm(
    name = "libnbd-0__1.20.2-2.el9.aarch64",
    sha256 = "77761819fff8b8e9a71542484d0117ffae7057151dd04b73f06c4908477249aa",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/libnbd-1.20.2-2.el9.aarch64.rpm"],
)

rpm(
    name = "libnbd-0__1.20.2-2.el9.s390x",
    sha256 = "9c45112cbfa18bfd2b92588a592c5bb3d39c83744168b1dfa3b6997a7ae9a754",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/libnbd-1.20.2-2.el9.s390x.rpm"],
)

rpm(
    name = "libnbd-0__1.20.2-2.el9.x86_64",
    sha256 = "cda5a0f85ad33681d86b3fd2f33642769def489e8970f9a78c8dfe46163dd805",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libnbd-1.20.2-2.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libnetfilter_conntrack-1.0.9-1.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libnfnetlink-1.0.1-21.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libnftnl-1.2.6-4.el9.aarch64.rpm"],
)

rpm(
    name = "libnftnl-0__1.2.6-4.el9.s390x",
    sha256 = "1a717d2a04f257e452753ba29cc6c0848cd51a226bf5d000b89863fa7aad5250",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libnftnl-1.2.6-4.el9.s390x.rpm"],
)

rpm(
    name = "libnftnl-0__1.2.6-4.el9.x86_64",
    sha256 = "45d7325859bdfbddd9f24235695fc55138549fdccbe509484e9f905c5f1b466b",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnftnl-1.2.6-4.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libnghttp2-1.43.0-6.el9.aarch64.rpm"],
)

rpm(
    name = "libnghttp2-0__1.43.0-6.el9.s390x",
    sha256 = "6d9ea7820d952bb492ff575b87fd46c606acf12bd368a5b4c8df3efc6a054c57",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libnghttp2-1.43.0-6.el9.s390x.rpm"],
)

rpm(
    name = "libnghttp2-0__1.43.0-6.el9.x86_64",
    sha256 = "fc1cadbc6cf37cbea60112b7ae6f92fabfd5a7f76fa526bb5a1ea82746455ec7",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnghttp2-1.43.0-6.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libpwquality-1.4.4-8.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libseccomp-2.5.2-2.el9.s390x.rpm"],
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
    name = "libselinux-0__3.6-1.el9.aarch64",
    sha256 = "c7de4c5829488c9d1f0bb815f9b29dc3919533fb914f3408c7a276db2ee6c297",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libselinux-3.6-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c7de4c5829488c9d1f0bb815f9b29dc3919533fb914f3408c7a276db2ee6c297",
    ],
)

rpm(
    name = "libselinux-0__3.6-1.el9.s390x",
    sha256 = "820f7c6a2321cc162df878504de505e26cdcd1be854bebbbedbbeb4b3cf95fc1",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libselinux-3.6-1.el9.s390x.rpm"],
)

rpm(
    name = "libselinux-0__3.6-1.el9.x86_64",
    sha256 = "274be34f74f9a5adab8428cd4761df8c50ff85b2c1bad832d90a3b3bf3efd174",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libselinux-3.6-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/274be34f74f9a5adab8428cd4761df8c50ff85b2c1bad832d90a3b3bf3efd174",
    ],
)

rpm(
    name = "libselinux-utils-0__3.6-1.el9.aarch64",
    sha256 = "da8f16409e08c5d959002f37c84bde486160de078869bc95a4f373b6100c1636",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libselinux-utils-3.6-1.el9.aarch64.rpm"],
)

rpm(
    name = "libselinux-utils-0__3.6-1.el9.s390x",
    sha256 = "b3af5a9ac657dde5261e50c7bc77ebff2f66f77f0bbc8313e7b977c57a76d335",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libselinux-utils-3.6-1.el9.s390x.rpm"],
)

rpm(
    name = "libselinux-utils-0__3.6-1.el9.x86_64",
    sha256 = "5ccd4affb097e4cb8f88917f8997f21d548272192ba8ab86b592a0de3b39305f",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libselinux-utils-3.6-1.el9.x86_64.rpm"],
)

rpm(
    name = "libsemanage-0__3.6-1.el9.aarch64",
    sha256 = "f82d438c8af25926a67f75e9f758a9d39419b6f540118d41a1c57aa2c902d1e8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libsemanage-3.6-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f82d438c8af25926a67f75e9f758a9d39419b6f540118d41a1c57aa2c902d1e8",
    ],
)

rpm(
    name = "libsemanage-0__3.6-1.el9.s390x",
    sha256 = "15eb6e6dfc06fa5351eca930a611803783991464209994d9a1402dfc76884f5f",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libsemanage-3.6-1.el9.s390x.rpm"],
)

rpm(
    name = "libsemanage-0__3.6-1.el9.x86_64",
    sha256 = "7f5bce18f75287a1911028cb38a5e8103787038026f6d1fda4ef6afa1cb00efd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsemanage-3.6-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/7f5bce18f75287a1911028cb38a5e8103787038026f6d1fda4ef6afa1cb00efd",
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libsepol-3.6-1.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libsigsegv-2.13-4.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/libslirp-4.4.0-8.el9.aarch64.rpm"],
)

rpm(
    name = "libslirp-0__4.4.0-8.el9.s390x",
    sha256 = "d47be3b8520589ff857b0264075f98b0483863762a0d3b0ebb1fba7c870edba6",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/libslirp-4.4.0-8.el9.s390x.rpm"],
)

rpm(
    name = "libslirp-0__4.4.0-8.el9.x86_64",
    sha256 = "aa5c4568ef12b3324e28e2353a97e5d531892e9e0682a035a5669819c7fd6dc3",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libslirp-4.4.0-8.el9.x86_64.rpm"],
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
    name = "libsmartcols-0__2.37.4-18.el9.aarch64",
    sha256 = "c2126c4ef442a5b76e12a7e26fe5d2c9c342aadeef567ff7ee6c445e9a50bd48",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libsmartcols-2.37.4-18.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/c2126c4ef442a5b76e12a7e26fe5d2c9c342aadeef567ff7ee6c445e9a50bd48",
    ],
)

rpm(
    name = "libsmartcols-0__2.37.4-18.el9.s390x",
    sha256 = "0e5adad760d3cc133a48f426a79fcba0fefd6d5e67aa0c9a64e15ed52afa806b",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libsmartcols-2.37.4-18.el9.s390x.rpm"],
)

rpm(
    name = "libsmartcols-0__2.37.4-18.el9.x86_64",
    sha256 = "9bfa25bb3cc5308b29c9eb466a40a9c35e9c7e3c4f31d71487a91f9dacaa1870",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsmartcols-2.37.4-18.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/9bfa25bb3cc5308b29c9eb466a40a9c35e9c7e3c4f31d71487a91f9dacaa1870",
    ],
)

rpm(
    name = "libstdc__plus____plus__-0__11.5.0-2.el9.aarch64",
    sha256 = "fff7f00d26008ab09b566c3b14d446b4b0b3df08bedbeee29142d62278568c82",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libstdc++-11.5.0-2.el9.aarch64.rpm"],
)

rpm(
    name = "libstdc__plus____plus__-0__11.5.0-2.el9.s390x",
    sha256 = "3644bcebe706602976b4d1596eedefcb0af0cfdb74141ca9084e0b34e8d22890",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libstdc++-11.5.0-2.el9.s390x.rpm"],
)

rpm(
    name = "libstdc__plus____plus__-0__11.5.0-2.el9.x86_64",
    sha256 = "dcd7090c2a37f13b2d4a1a2bc2d1fedc514c745efc4f2619783bbd1979b5e82f",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libstdc++-11.5.0-2.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libtasn1-4.16.0-8.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libunistring-0.9.10-15.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/liburing-2.5-1.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libutempter-1.2.1-6.el9.s390x.rpm"],
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
    name = "libuuid-0__2.37.4-18.el9.aarch64",
    sha256 = "39003e00883594e490723b13754537b0b53ebc320b91ed58fe4715f852dee8ee",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libuuid-2.37.4-18.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/39003e00883594e490723b13754537b0b53ebc320b91ed58fe4715f852dee8ee",
    ],
)

rpm(
    name = "libuuid-0__2.37.4-18.el9.s390x",
    sha256 = "7fc04c6e2b4159bbb2b3c5aa761359205d44447aab3ba5799de69c167cf234fb",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libuuid-2.37.4-18.el9.s390x.rpm"],
)

rpm(
    name = "libuuid-0__2.37.4-18.el9.x86_64",
    sha256 = "226514aadd153d4065cdb1d008f1dbab1d8ee8653e319331c9000e52ebbefe67",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libuuid-2.37.4-18.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/226514aadd153d4065cdb1d008f1dbab1d8ee8653e319331c9000e52ebbefe67",
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libverto-0.3.2-3.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libxcrypt-4.4.18-3.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libxml2-2.9.13-6.el9.aarch64.rpm"],
)

rpm(
    name = "libxml2-0__2.9.13-6.el9.s390x",
    sha256 = "2ba167d1c5fe690868d32c2f09645a080297ca7f731c9793c9ac89ff8043455d",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libxml2-2.9.13-6.el9.s390x.rpm"],
)

rpm(
    name = "libxml2-0__2.9.13-6.el9.x86_64",
    sha256 = "7b23a9ca73db2ec13ee983594d4d0f4a85160ef8d05484f65c247801cb808a29",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libxml2-2.9.13-6.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/libzstd-1.5.1-2.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/lua-libs-5.4.4-4.el9.aarch64.rpm"],
)

rpm(
    name = "lua-libs-0__5.4.4-4.el9.s390x",
    sha256 = "616111e91869993d6db2fec066d5b5b29b2c17bfbce87748a51ed772dbc4d4ca",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/lua-libs-5.4.4-4.el9.s390x.rpm"],
)

rpm(
    name = "lua-libs-0__5.4.4-4.el9.x86_64",
    sha256 = "a24f7e08163b012cdbbdaba70788331050c2b7bdb9bc2fdc261c5c1f3cd3960d",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/lua-libs-5.4.4-4.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/lz4-libs-1.9.3-5.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/mpfr-4.1.0-7.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-basic-filters-1.38.3-1.el9.aarch64.rpm"],
)

rpm(
    name = "nbdkit-basic-filters-0__1.38.3-1.el9.s390x",
    sha256 = "d034cf81f3bfc29ce43e4dd6485ae174ac1ec165f5995336481c91d689d91f4b",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nbdkit-basic-filters-1.38.3-1.el9.s390x.rpm"],
)

rpm(
    name = "nbdkit-basic-filters-0__1.38.3-1.el9.x86_64",
    sha256 = "2ad146adcb3e97799c753b77ec62a053fff431c2729293641f8802ba3345aee3",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-basic-filters-1.38.3-1.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.38.3-1.el9.aarch64",
    sha256 = "48e087e299e117bc40a0e537d050a90dd856d189919146199660397b427bc672",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-curl-plugin-1.38.3-1.el9.aarch64.rpm"],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.38.3-1.el9.s390x",
    sha256 = "be9620c68f9a2f3ddb060b737ff94fcfc9b9d8b7bdd75b590a09b40160aedfb5",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nbdkit-curl-plugin-1.38.3-1.el9.s390x.rpm"],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.38.3-1.el9.x86_64",
    sha256 = "50399bc6593ef1ebedba72d336e7321c2f1b4fbaacbad1665c6975995dd13089",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-curl-plugin-1.38.3-1.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-gzip-filter-0__1.38.3-1.el9.aarch64",
    sha256 = "69155a4aacac2311e8c53c94e9e18c6db9f226e4099a4417c959ded7a0b0989b",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-gzip-filter-1.38.3-1.el9.aarch64.rpm"],
)

rpm(
    name = "nbdkit-gzip-filter-0__1.38.3-1.el9.s390x",
    sha256 = "f245e3a35de1839d7c0f0c9a4fbfa03672ac8820739a4631b8babc90e7e2f033",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nbdkit-gzip-filter-1.38.3-1.el9.s390x.rpm"],
)

rpm(
    name = "nbdkit-gzip-filter-0__1.38.3-1.el9.x86_64",
    sha256 = "b74c8b6f273f55909d498cba30f02740785dae1be3e081dd4635c69b1bf63e4a",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-gzip-filter-1.38.3-1.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-server-0__1.38.3-1.el9.aarch64",
    sha256 = "747ad87c46fd8b73e0f94408debcce76025cc9b8ffa1dc0bccd0db1f16e64a6f",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-server-1.38.3-1.el9.aarch64.rpm"],
)

rpm(
    name = "nbdkit-server-0__1.38.3-1.el9.s390x",
    sha256 = "d5f685ba9144586833b00db95571abf5d5e1f6e98307fd00ef35c6ef3ca5dc54",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nbdkit-server-1.38.3-1.el9.s390x.rpm"],
)

rpm(
    name = "nbdkit-server-0__1.38.3-1.el9.x86_64",
    sha256 = "c7d5c86229a21c1deb0df0826cbbce8599e5fc802d44e30c1fe9ead1ab668ced",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-server-1.38.3-1.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-vddk-plugin-0__1.38.3-1.el9.x86_64",
    sha256 = "9edc882953871a0aafbab48f4466d2b7c41fc59d393a7c04c7fad00bd6acc542",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-vddk-plugin-1.38.3-1.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-xz-filter-0__1.38.3-1.el9.aarch64",
    sha256 = "cc9178311ff0da758736d72d8a25e6ea866a1bf13ae01c5ab81f637f9cc4edc1",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-xz-filter-1.38.3-1.el9.aarch64.rpm"],
)

rpm(
    name = "nbdkit-xz-filter-0__1.38.3-1.el9.s390x",
    sha256 = "9d7cfd87165cf04892c92e65eff48dc5c1f906b72900b8124e03c67ec2b8522b",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nbdkit-xz-filter-1.38.3-1.el9.s390x.rpm"],
)

rpm(
    name = "nbdkit-xz-filter-0__1.38.3-1.el9.x86_64",
    sha256 = "61091c8765e22a06c0d6a8d89956794bc47afb3f2081ed8787641117b8190975",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-xz-filter-1.38.3-1.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/ncurses-base-6.2-10.20210508.el9.noarch.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/ncurses-libs-6.2-10.20210508.el9.s390x.rpm"],
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
    name = "netavark-2__1.12.1-1.el9.aarch64",
    sha256 = "0b65418f9c44f0a4b8e3c2ef03fcaba97b6fdbdde277f33211b41821305b2b3f",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/netavark-1.12.1-1.el9.aarch64.rpm"],
)

rpm(
    name = "netavark-2__1.12.1-1.el9.s390x",
    sha256 = "24f7bbabdc827c4ce6850147abaf103746ee609a8b012d734b8bb32b6e5caf24",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/netavark-1.12.1-1.el9.s390x.rpm"],
)

rpm(
    name = "netavark-2__1.12.1-1.el9.x86_64",
    sha256 = "bf8f36a76417919074f77f5ed0d9de128e208b809926e18724946b4e7d36f944",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/netavark-1.12.1-1.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/nettle-3.9.1-1.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/nftables-1.0.9-3.el9.aarch64.rpm"],
)

rpm(
    name = "nftables-1__1.0.9-3.el9.s390x",
    sha256 = "a8d9bd2a045a06a50756af71d41a3d4d15677d120bb1cf833907db2e990adad0",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/nftables-1.0.9-3.el9.s390x.rpm"],
)

rpm(
    name = "nftables-1__1.0.9-3.el9.x86_64",
    sha256 = "3f72eee1c40da5fa1f2eb59a77723f781ff27c53411b2aca1aee8bd6a577915b",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/nftables-1.0.9-3.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nginx-1.22.1-4.module_el9+666+132dc76f.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nginx-core-1.22.1-4.module_el9+666+132dc76f.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/nginx-filesystem-1.22.1-4.module_el9+666+132dc76f.noarch.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/npth-1.6-8.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openldap-2.6.6-3.el9.s390x.rpm"],
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
    name = "openssl-1__3.2.2-2.el9.aarch64",
    sha256 = "01b335a1c3380c7f458404bf036bed73ffeefb179a38b2eae3f432454fdb3797",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openssl-3.2.2-2.el9.aarch64.rpm"],
)

rpm(
    name = "openssl-1__3.2.2-2.el9.s390x",
    sha256 = "34c6413378eb8ac27a108daa8c4ab575e9b49b89ef971fbf942e75f297192bb2",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openssl-3.2.2-2.el9.s390x.rpm"],
)

rpm(
    name = "openssl-1__3.2.2-2.el9.x86_64",
    sha256 = "220bc122a55b081c8f05094b773875cc47de713a2425edf284d9048373431428",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-3.2.2-2.el9.x86_64.rpm"],
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
    name = "openssl-libs-1__3.2.2-2.el9.aarch64",
    sha256 = "29de503dcbd1c3bfdacb137c6798d76e68abcd76d9f164d819c1bc1f1f78c05b",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openssl-libs-3.2.2-2.el9.aarch64.rpm"],
)

rpm(
    name = "openssl-libs-1__3.2.2-2.el9.s390x",
    sha256 = "9cd15f4aa6968428b952f2feac5b965864086825686fbd9f90a0141d19d17440",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/openssl-libs-3.2.2-2.el9.s390x.rpm"],
)

rpm(
    name = "openssl-libs-1__3.2.2-2.el9.x86_64",
    sha256 = "b060c100f9920714d6923427ed9426ae742c715bc0badfa041c64972ccc5ca45",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-libs-3.2.2-2.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/p11-kit-0.25.3-2.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/p11-kit-trust-0.25.3-2.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/pam-1.5.1-20.el9.aarch64.rpm"],
)

rpm(
    name = "pam-0__1.5.1-20.el9.s390x",
    sha256 = "c56873f126b79e8f9585a7b84999fdab1e4119d1463cc837bb881a1dab9b58bd",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/pam-1.5.1-20.el9.s390x.rpm"],
)

rpm(
    name = "pam-0__1.5.1-20.el9.x86_64",
    sha256 = "090321330a9e9bf608158c22ad2aaaa35b6841d02692159a4e6e0126db52c73f",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pam-1.5.1-20.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/pcre-8.44-4.el9.aarch64.rpm"],
)

rpm(
    name = "pcre-0__8.44-4.el9.s390x",
    sha256 = "e42ebd2b71ed4d5ee34a5fbba116396c22ed4deb7d7ab6189f048a3f603e5dbb",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/pcre-8.44-4.el9.s390x.rpm"],
)

rpm(
    name = "pcre-0__8.44-4.el9.x86_64",
    sha256 = "7d6be1d41cb4d0b159a764bfc7c8efecc0353224b46e5286cbbea7092b700690",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pcre-8.44-4.el9.x86_64.rpm"],
)

rpm(
    name = "pcre2-0__10.37-3.el9.1.aarch64",
    sha256 = "82de22426c96c26e987befb1056e2a6ecd71ba6966736cd3810522e7da77a0f2",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/pcre2-10.37-3.el9.1.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/82de22426c96c26e987befb1056e2a6ecd71ba6966736cd3810522e7da77a0f2",
    ],
)

rpm(
    name = "pcre2-0__10.37-3.el9.1.x86_64",
    sha256 = "441e71f24e95b7c319f02264db53f88aa49778b2214f7dd5c75f1a3838e72dea",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pcre2-10.37-3.el9.1.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/441e71f24e95b7c319f02264db53f88aa49778b2214f7dd5c75f1a3838e72dea",
    ],
)

rpm(
    name = "pcre2-0__10.40-6.el9.aarch64",
    sha256 = "c13e323c383bd5bbe3415701aa21a56b3fefc32d96e081e91c012ef692c78599",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/pcre2-10.40-6.el9.aarch64.rpm"],
)

rpm(
    name = "pcre2-0__10.40-6.el9.s390x",
    sha256 = "f7c2df461b8fe6a9617a1c1089fc88576e4df16f6ff9aea83b05413d2e15b4d5",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/pcre2-10.40-6.el9.s390x.rpm"],
)

rpm(
    name = "pcre2-0__10.40-6.el9.x86_64",
    sha256 = "bc1012f5417aab8393836d78ac8c5472b1a2d84a2f9fa2b00fff5f8ad3a5ec26",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pcre2-10.40-6.el9.x86_64.rpm"],
)

rpm(
    name = "pcre2-syntax-0__10.37-3.el9.1.aarch64",
    sha256 = "55d7d2bc962334c236418b78199a496b05dea4efdc89e52453154bd1a5ad0e2e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/pcre2-syntax-10.37-3.el9.1.noarch.rpm",
        "https://storage.googleapis.com/builddeps/55d7d2bc962334c236418b78199a496b05dea4efdc89e52453154bd1a5ad0e2e",
    ],
)

rpm(
    name = "pcre2-syntax-0__10.37-3.el9.1.x86_64",
    sha256 = "55d7d2bc962334c236418b78199a496b05dea4efdc89e52453154bd1a5ad0e2e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pcre2-syntax-10.37-3.el9.1.noarch.rpm",
        "https://storage.googleapis.com/builddeps/55d7d2bc962334c236418b78199a496b05dea4efdc89e52453154bd1a5ad0e2e",
    ],
)

rpm(
    name = "pcre2-syntax-0__10.40-6.el9.aarch64",
    sha256 = "be36a84f6e311a59190664d61a466471391ab01fb77bd1d2348e9a76414aded4",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/pcre2-syntax-10.40-6.el9.noarch.rpm"],
)

rpm(
    name = "pcre2-syntax-0__10.40-6.el9.s390x",
    sha256 = "be36a84f6e311a59190664d61a466471391ab01fb77bd1d2348e9a76414aded4",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/pcre2-syntax-10.40-6.el9.noarch.rpm"],
)

rpm(
    name = "pcre2-syntax-0__10.40-6.el9.x86_64",
    sha256 = "be36a84f6e311a59190664d61a466471391ab01fb77bd1d2348e9a76414aded4",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pcre2-syntax-10.40-6.el9.noarch.rpm"],
)

rpm(
    name = "policycoreutils-0__3.6-2.1.el9.aarch64",
    sha256 = "93270211cc317bdd44706c3a216ebc8155942e349510a3906f26df0d10328d78",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/policycoreutils-3.6-2.1.el9.aarch64.rpm"],
)

rpm(
    name = "policycoreutils-0__3.6-2.1.el9.s390x",
    sha256 = "7ccadb5f8c3ecea0e24447211179c90abbb56cb8d52b97e811137a4588d9ce79",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/policycoreutils-3.6-2.1.el9.s390x.rpm"],
)

rpm(
    name = "policycoreutils-0__3.6-2.1.el9.x86_64",
    sha256 = "a87874363af6432b1c96b40f8b79b90616df22bff3bd4f9aa39da24f5bddd3e9",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/policycoreutils-3.6-2.1.el9.x86_64.rpm"],
)

rpm(
    name = "popt-0__1.18-8.el9.aarch64",
    sha256 = "032427adaa37d2a1c6d2f3cab42ccbdce2c6d9b3c1f3cd91c05a92c99198babb",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/popt-1.18-8.el9.aarch64.rpm"],
)

rpm(
    name = "popt-0__1.18-8.el9.s390x",
    sha256 = "b2bc4dbd78a6c3b9458cbc022e80d860fb2c6022fa308604f553289b62cb9511",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/popt-1.18-8.el9.s390x.rpm"],
)

rpm(
    name = "popt-0__1.18-8.el9.x86_64",
    sha256 = "d864419035e99f8bb06f5d1c767608ed81f942cb128a98b590c1dbc4afbd54d4",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/popt-1.18-8.el9.x86_64.rpm"],
)

rpm(
    name = "python3-0__3.9.19-7.el9.aarch64",
    sha256 = "b94cda69d019e6be68a9462cae16e5fc88f6d4f0582f36d57eb4c1bde7bf9d44",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-3.9.19-7.el9.aarch64.rpm"],
)

rpm(
    name = "python3-0__3.9.19-7.el9.s390x",
    sha256 = "8a0413615d5d3959877dc76f9744922f07ddf7a7008a25fafd3e24fc2379dbb4",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-3.9.19-7.el9.s390x.rpm"],
)

rpm(
    name = "python3-0__3.9.19-7.el9.x86_64",
    sha256 = "ca7edddb7491de30ee8f150111261047898e075f617e24a962b2ec49f20ab05c",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-3.9.19-7.el9.x86_64.rpm"],
)

rpm(
    name = "python3-libs-0__3.9.19-7.el9.aarch64",
    sha256 = "6ec67b2823d1329b663b44647370fbe6a8942a23bc65621f505e626329bbcab8",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-libs-3.9.19-7.el9.aarch64.rpm"],
)

rpm(
    name = "python3-libs-0__3.9.19-7.el9.s390x",
    sha256 = "f4dac16516d879662d2827086e59d3c388be3074d33eee223057fc74963c176e",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-libs-3.9.19-7.el9.s390x.rpm"],
)

rpm(
    name = "python3-libs-0__3.9.19-7.el9.x86_64",
    sha256 = "596cfbbf11ecae78eb077f9799f1b6c756cf7a8ae5fe745d5e901877a96d9c3c",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-libs-3.9.19-7.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-pip-wheel-21.3.1-1.el9.noarch.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/python3-pycurl-7.43.0.6-8.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-setuptools-wheel-53.0.0-13.el9.noarch.rpm"],
)

rpm(
    name = "python3-setuptools-wheel-0__53.0.0-13.el9.s390x",
    sha256 = "a4dfbc2c514f58839d7704acc046eb0fc54cfb670413decebd9641b4d76439e8",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-setuptools-wheel-53.0.0-13.el9.noarch.rpm"],
)

rpm(
    name = "python3-setuptools-wheel-0__53.0.0-13.el9.x86_64",
    sha256 = "a4dfbc2c514f58839d7704acc046eb0fc54cfb670413decebd9641b4d76439e8",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-setuptools-wheel-53.0.0-13.el9.noarch.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-six-1.15.0-9.el9.noarch.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-systemd-234-19.el9.aarch64.rpm"],
)

rpm(
    name = "python3-systemd-0__234-19.el9.s390x",
    sha256 = "fb74b14bf11fa996a2ef540126119febcb8aa2eeb3a855dffb1eba481b07c878",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/python3-systemd-234-19.el9.s390x.rpm"],
)

rpm(
    name = "python3-systemd-0__234-19.el9.x86_64",
    sha256 = "10ce18f02053671942ae5dc165c95cb195a50c309b90159e006214da2c953ea0",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-systemd-234-19.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/qemu-img-9.0.0-8.el9.aarch64.rpm"],
)

rpm(
    name = "qemu-img-17__9.0.0-8.el9.s390x",
    sha256 = "715a07b59920c5d1b5f2b48c4dcd05152f7c39982e2949d5d1136bcceb431347",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/qemu-img-9.0.0-8.el9.s390x.rpm"],
)

rpm(
    name = "qemu-img-17__9.0.0-8.el9.x86_64",
    sha256 = "15c6a6264bab7d87a38b3e74dc86b6c545e045369224445d04460e95c245f510",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-img-9.0.0-8.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/readline-8.1-4.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/rpm-4.16.1.3-34.el9.aarch64.rpm"],
)

rpm(
    name = "rpm-0__4.16.1.3-34.el9.s390x",
    sha256 = "2f94616fb6c12f708fce7b61d224462530eebdbf549b86aaf2b6efe3be85c511",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/rpm-4.16.1.3-34.el9.s390x.rpm"],
)

rpm(
    name = "rpm-0__4.16.1.3-34.el9.x86_64",
    sha256 = "be44e845149e2d1585d14e0330a62b54bf6ffcdcac5aa4443b7701b5de6ed199",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/rpm-4.16.1.3-34.el9.x86_64.rpm"],
)

rpm(
    name = "rpm-libs-0__4.16.1.3-34.el9.aarch64",
    sha256 = "770978997ed19345fb383beed22b50d27d11d5fdee1e55abb294cf4c73f3a871",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/rpm-libs-4.16.1.3-34.el9.aarch64.rpm"],
)

rpm(
    name = "rpm-libs-0__4.16.1.3-34.el9.s390x",
    sha256 = "51263dbdab8a002394bb41a2a86938548b8a26a02989b58f1fd1cec5373d2ac1",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/rpm-libs-4.16.1.3-34.el9.s390x.rpm"],
)

rpm(
    name = "rpm-libs-0__4.16.1.3-34.el9.x86_64",
    sha256 = "6a7a1900c4ce0e2e285719cb61da9f6d90aea1141f9b7e17cda7eaddb11cf943",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/rpm-libs-4.16.1.3-34.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/sed-4.8-9.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/setup-2.13.7-10.el9.noarch.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/shadow-utils-4.9-9.el9.aarch64.rpm"],
)

rpm(
    name = "shadow-utils-2__4.9-9.el9.s390x",
    sha256 = "aeee106d85e7550d08344bffdf2f833a87349722dbc45e777462bea8d9ea2cf2",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/shadow-utils-4.9-9.el9.s390x.rpm"],
)

rpm(
    name = "shadow-utils-2__4.9-9.el9.x86_64",
    sha256 = "e3c73fa1856efa23068675638e535a414e97b8927358f4578569f3cdb9ce3ee9",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/shadow-utils-4.9-9.el9.x86_64.rpm"],
)

rpm(
    name = "shadow-utils-subid-2__4.9-9.el9.aarch64",
    sha256 = "738bb7e9466b30cca673099e75eb1f332c44c30742e007b54700cce4db955802",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/shadow-utils-subid-4.9-9.el9.aarch64.rpm"],
)

rpm(
    name = "shadow-utils-subid-2__4.9-9.el9.s390x",
    sha256 = "2da0e548767449d98b12c904558be0ce1ce48f240598c90cdf876bcad57f8922",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/shadow-utils-subid-4.9-9.el9.s390x.rpm"],
)

rpm(
    name = "shadow-utils-subid-2__4.9-9.el9.x86_64",
    sha256 = "f3889cf52bb1432583338863ecbf9bf0a2a49df664555a6dda328aed18fa55eb",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/shadow-utils-subid-4.9-9.el9.x86_64.rpm"],
)

rpm(
    name = "slirp4netns-0__1.3.1-1.el9.aarch64",
    sha256 = "b33da013f63e8d75fdb29f1383139b55021f912b3a414ed8b729821328eae5eb",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/slirp4netns-1.3.1-1.el9.aarch64.rpm"],
)

rpm(
    name = "slirp4netns-0__1.3.1-1.el9.s390x",
    sha256 = "9a6933cc841405daeb547dd6200d0b719f73c9cd655e1a39f89b9f42608c478c",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/slirp4netns-1.3.1-1.el9.s390x.rpm"],
)

rpm(
    name = "slirp4netns-0__1.3.1-1.el9.x86_64",
    sha256 = "822eea36e2a390042bd22831ce6e6d9e6f596878395df31245354476e6f56f02",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/slirp4netns-1.3.1-1.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/sqlite-libs-3.34.1-7.el9.s390x.rpm"],
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
    name = "systemd-0__252-38.el9.aarch64",
    sha256 = "3c5fb0d3d2b14aa40f08c120605b397472b72b65464374d58971feed94440a6e",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-252-38.el9.aarch64.rpm"],
)

rpm(
    name = "systemd-0__252-38.el9.s390x",
    sha256 = "423307bc54700b7104280022f34f0d8ec45cb3f5aa6149744caffd66f013a24f",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-252-38.el9.s390x.rpm"],
)

rpm(
    name = "systemd-0__252-38.el9.x86_64",
    sha256 = "26d28d48ad55de5ecd0dc6915443945261742e9194882fe7b7136c29f3fc8f0b",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-252-38.el9.x86_64.rpm"],
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
    name = "systemd-libs-0__252-38.el9.aarch64",
    sha256 = "7280efca1c6ee6260f4c22ba93aa7376553ff526d0caae68f3da3030c583e531",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-libs-252-38.el9.aarch64.rpm"],
)

rpm(
    name = "systemd-libs-0__252-38.el9.s390x",
    sha256 = "a3ad07eec599b4d094a00af3c1a28e2b4659e2ae0207818a410314a967cd5deb",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-libs-252-38.el9.s390x.rpm"],
)

rpm(
    name = "systemd-libs-0__252-38.el9.x86_64",
    sha256 = "93841995451858c7948fa6d594ea5011d7e2014b0426fd3303568627b7202db2",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-libs-252-38.el9.x86_64.rpm"],
)

rpm(
    name = "systemd-pam-0__252-38.el9.aarch64",
    sha256 = "1701a4a0cfa8d9188505c476ff54eaec8740ed0660246ab74b62aabb270ae7e9",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-pam-252-38.el9.aarch64.rpm"],
)

rpm(
    name = "systemd-pam-0__252-38.el9.s390x",
    sha256 = "0302cbb6f98253d4c6785a1b92273a423803b9764caccba9ad6fbd8ba557e496",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-pam-252-38.el9.s390x.rpm"],
)

rpm(
    name = "systemd-pam-0__252-38.el9.x86_64",
    sha256 = "50d8f2929dd004c7f5401702a4f719873d3cf747bde7f5ac2904550047eb42b6",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-pam-252-38.el9.x86_64.rpm"],
)

rpm(
    name = "systemd-rpm-macros-0__252-38.el9.aarch64",
    sha256 = "2e30d3ea1f427ade9eda3bc53b275a5e1c72ba56cba0bb9fb750a9494de2a369",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-rpm-macros-252-38.el9.noarch.rpm"],
)

rpm(
    name = "systemd-rpm-macros-0__252-38.el9.s390x",
    sha256 = "2e30d3ea1f427ade9eda3bc53b275a5e1c72ba56cba0bb9fb750a9494de2a369",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/systemd-rpm-macros-252-38.el9.noarch.rpm"],
)

rpm(
    name = "systemd-rpm-macros-0__252-38.el9.x86_64",
    sha256 = "2e30d3ea1f427ade9eda3bc53b275a5e1c72ba56cba0bb9fb750a9494de2a369",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-rpm-macros-252-38.el9.noarch.rpm"],
)

rpm(
    name = "tar-2__1.34-6.el9.aarch64",
    sha256 = "98a9ca5a25c6aa73b5183b3333abad062a8f82d8b9390d2b2fbdc1eea5b4fb9b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/tar-1.34-6.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/98a9ca5a25c6aa73b5183b3333abad062a8f82d8b9390d2b2fbdc1eea5b4fb9b",
    ],
)

rpm(
    name = "tar-2__1.34-6.el9.s390x",
    sha256 = "584142eceda9fb5685310dab712603636c0f66efe7c24c9b8150d8fc07c0fef2",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/tar-1.34-6.el9.s390x.rpm"],
)

rpm(
    name = "tar-2__1.34-6.el9.x86_64",
    sha256 = "9f6adb2da035d5123587a2bb401487521bd6543497003ffc6e66386d898133f3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/tar-1.34-6.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/9f6adb2da035d5123587a2bb401487521bd6543497003ffc6e66386d898133f3",
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/tzdata-2024a-2.el9.noarch.rpm"],
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
    name = "util-linux-0__2.37.4-18.el9.aarch64",
    sha256 = "b3d4b1559f81d72e2112e9f5acf1c9e0bdadaa366cbcb8df7d2e603db0e689e4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/util-linux-2.37.4-18.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/b3d4b1559f81d72e2112e9f5acf1c9e0bdadaa366cbcb8df7d2e603db0e689e4",
    ],
)

rpm(
    name = "util-linux-0__2.37.4-18.el9.s390x",
    sha256 = "ab31c060229398e6e3e33034505b5e4254bab1e56d0e426efa521292628ac695",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/util-linux-2.37.4-18.el9.s390x.rpm"],
)

rpm(
    name = "util-linux-0__2.37.4-18.el9.x86_64",
    sha256 = "9f9b09b5d78cbeb3df6b9d6d5901b6a0b2b9668bf541d93e1a18e68cb33a8fb5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/util-linux-2.37.4-18.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/9f9b09b5d78cbeb3df6b9d6d5901b6a0b2b9668bf541d93e1a18e68cb33a8fb5",
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
    name = "util-linux-core-0__2.37.4-18.el9.aarch64",
    sha256 = "d0145bfa0348ccc4e23e2414533b9517ae02911645e312468d27cddb32575790",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/util-linux-core-2.37.4-18.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/d0145bfa0348ccc4e23e2414533b9517ae02911645e312468d27cddb32575790",
    ],
)

rpm(
    name = "util-linux-core-0__2.37.4-18.el9.s390x",
    sha256 = "a181dbfe6385d7564f766f07785490542edce4bf086b6669f09cffb9d9871c35",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/util-linux-core-2.37.4-18.el9.s390x.rpm"],
)

rpm(
    name = "util-linux-core-0__2.37.4-18.el9.x86_64",
    sha256 = "85a2802812f1ae05fa54e713183c9577afe767f47928cf66be746b12fa81edea",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/util-linux-core-2.37.4-18.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/85a2802812f1ae05fa54e713183c9577afe767f47928cf66be746b12fa81edea",
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/vim-minimal-8.2.2637-21.el9.aarch64.rpm"],
)

rpm(
    name = "vim-minimal-2__8.2.2637-21.el9.s390x",
    sha256 = "a04988c53eea9735bb2eb5106e7e2215f5a355af2c33dfbe20c643c811b9176f",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/vim-minimal-8.2.2637-21.el9.s390x.rpm"],
)

rpm(
    name = "vim-minimal-2__8.2.2637-21.el9.x86_64",
    sha256 = "1b15304790e4b2e7d4ff378b7bf0363b6ecb1c852fc42f984267296538de0c16",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/vim-minimal-8.2.2637-21.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/xz-libs-5.2.5-8.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/s390x/os/Packages/yajl-2.1.0-22.el9.s390x.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/s390x/os/Packages/zlib-1.2.11-41.el9.s390x.rpm"],
)

rpm(
    name = "zlib-0__1.2.11-41.el9.x86_64",
    sha256 = "370951ea635bc16313f21ac2823ec815147ed1124b74865a34c54e94e4db9602",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/zlib-1.2.11-41.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/370951ea635bc16313f21ac2823ec815147ed1124b74865a34c54e94e4db9602",
    ],
)
