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
    sha256 = "685052b498b6ddfe562ca7a97736741d87916fe536623afb7da2824c0211c369",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.33.0/rules_go-v0.33.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.33.0/rules_go-v0.33.0.zip",
        "https://storage.googleapis.com/builddeps/685052b498b6ddfe562ca7a97736741d87916fe536623afb7da2824c0211c369",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(
    go_version = "1.18.10",
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/alternatives-1.24-1.el9.aarch64.rpm"],
)

rpm(
    name = "alternatives-0__1.24-1.el9.x86_64",
    sha256 = "b58e7ea30c27ecb321d9a279b95b62aef59d92173714fce859bfb359ee231ff3",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/alternatives-1.24-1.el9.x86_64.rpm"],
)

rpm(
    name = "audit-libs-0__3.0.7-104.el9.aarch64",
    sha256 = "959ba1d41b5d28d7121c2f8b64f931d886f2d41f77fbd1034d55fbb33328d9f8",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/audit-libs-3.0.7-104.el9.aarch64.rpm"],
)

rpm(
    name = "audit-libs-0__3.0.7-104.el9.x86_64",
    sha256 = "22d335e369ffb90f4b9ab496aa5f47c1a0913679db7ff4e706397be3bba4e3ca",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/audit-libs-3.0.7-104.el9.x86_64.rpm"],
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
    name = "bash-0__5.1.8-6.el9.aarch64",
    sha256 = "adbea9afe78b2f67de854fdf5440326dda5383763797eb9ac486969edeecaef0",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/bash-5.1.8-6.el9.aarch64.rpm"],
)

rpm(
    name = "bash-0__5.1.8-6.el9.x86_64",
    sha256 = "09f700a94e187a74f6f4a5f750082732e193d41392a85f042bdeb0bcbabe0a1f",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/bash-5.1.8-6.el9.x86_64.rpm"],
)

rpm(
    name = "buildah-1__1.31.3-1.el9.aarch64",
    sha256 = "75cee8238006cd2ad42054a66cb93a36d429c86d661d293a632c19acf87b24a9",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/buildah-1.31.3-1.el9.aarch64.rpm"],
)

rpm(
    name = "buildah-1__1.31.3-1.el9.x86_64",
    sha256 = "468a96b262c7368f340dfbd3f3d02a6ba1c67eda39bd5f8ebe47f379b7c1ad43",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/buildah-1.31.3-1.el9.x86_64.rpm"],
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
    name = "ca-certificates-0__2022.2.54-90.2.el9.aarch64",
    sha256 = "24978e8dd3e054583da86036657ab16e93da97a0bafc148ec28d871d8c15257c",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/ca-certificates-2022.2.54-90.2.el9.noarch.rpm"],
)

rpm(
    name = "ca-certificates-0__2022.2.54-90.2.el9.x86_64",
    sha256 = "24978e8dd3e054583da86036657ab16e93da97a0bafc148ec28d871d8c15257c",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ca-certificates-2022.2.54-90.2.el9.noarch.rpm"],
)

rpm(
    name = "centos-gpg-keys-0__9.0-12.el9.aarch64",
    sha256 = "3af698b9f4dbf5368d1454df4e06cb8ffb75247b7b8385cfb0f7698f3db7d3ab",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-gpg-keys-9.0-12.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/3af698b9f4dbf5368d1454df4e06cb8ffb75247b7b8385cfb0f7698f3db7d3ab",
    ],
)

rpm(
    name = "centos-gpg-keys-0__9.0-12.el9.x86_64",
    sha256 = "3af698b9f4dbf5368d1454df4e06cb8ffb75247b7b8385cfb0f7698f3db7d3ab",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-gpg-keys-9.0-12.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/3af698b9f4dbf5368d1454df4e06cb8ffb75247b7b8385cfb0f7698f3db7d3ab",
    ],
)

rpm(
    name = "centos-gpg-keys-0__9.0-22.el9.aarch64",
    sha256 = "108f1f4f4aee8ccdf6535833bbab14bd3808c395ef7d1d425473aee4467e2156",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-gpg-keys-9.0-22.el9.noarch.rpm"],
)

rpm(
    name = "centos-gpg-keys-0__9.0-22.el9.x86_64",
    sha256 = "108f1f4f4aee8ccdf6535833bbab14bd3808c395ef7d1d425473aee4467e2156",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-gpg-keys-9.0-22.el9.noarch.rpm"],
)

rpm(
    name = "centos-logos-httpd-0__90.4-1.el9.aarch64",
    sha256 = "b74cfa3743b6fcc4290ceea6da3ed5cead7ba4bf7c18b8f301d37e1a9e62d20e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/centos-logos-httpd-90.4-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/b74cfa3743b6fcc4290ceea6da3ed5cead7ba4bf7c18b8f301d37e1a9e62d20e",
    ],
)

rpm(
    name = "centos-logos-httpd-0__90.4-1.el9.x86_64",
    sha256 = "b74cfa3743b6fcc4290ceea6da3ed5cead7ba4bf7c18b8f301d37e1a9e62d20e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/centos-logos-httpd-90.4-1.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/b74cfa3743b6fcc4290ceea6da3ed5cead7ba4bf7c18b8f301d37e1a9e62d20e",
    ],
)

rpm(
    name = "centos-stream-release-0__9.0-12.el9.aarch64",
    sha256 = "400b77fb28443d36a6fa3c25c95e84b843ac9ae17b205651f1e2bea32c7289cc",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-stream-release-9.0-12.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/400b77fb28443d36a6fa3c25c95e84b843ac9ae17b205651f1e2bea32c7289cc",
    ],
)

rpm(
    name = "centos-stream-release-0__9.0-12.el9.x86_64",
    sha256 = "400b77fb28443d36a6fa3c25c95e84b843ac9ae17b205651f1e2bea32c7289cc",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-stream-release-9.0-12.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/400b77fb28443d36a6fa3c25c95e84b843ac9ae17b205651f1e2bea32c7289cc",
    ],
)

rpm(
    name = "centos-stream-release-0__9.0-22.el9.aarch64",
    sha256 = "78cc2608d720cb0c4a1528365036a64eb64b622a76e40d0625e805428db0d66a",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-stream-release-9.0-22.el9.noarch.rpm"],
)

rpm(
    name = "centos-stream-release-0__9.0-22.el9.x86_64",
    sha256 = "78cc2608d720cb0c4a1528365036a64eb64b622a76e40d0625e805428db0d66a",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-stream-release-9.0-22.el9.noarch.rpm"],
)

rpm(
    name = "centos-stream-repos-0__9.0-12.el9.aarch64",
    sha256 = "d093d9f9021a8edc28843f61059a94bd8aa0109f6a9a865c2a1560cf6602a2ab",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-stream-repos-9.0-12.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/d093d9f9021a8edc28843f61059a94bd8aa0109f6a9a865c2a1560cf6602a2ab",
    ],
)

rpm(
    name = "centos-stream-repos-0__9.0-12.el9.x86_64",
    sha256 = "d093d9f9021a8edc28843f61059a94bd8aa0109f6a9a865c2a1560cf6602a2ab",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-stream-repos-9.0-12.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/d093d9f9021a8edc28843f61059a94bd8aa0109f6a9a865c2a1560cf6602a2ab",
    ],
)

rpm(
    name = "centos-stream-repos-0__9.0-22.el9.aarch64",
    sha256 = "41eee9bd192d069e855f6b8ff43e08e350d02eeab8f47a23aa2fc517936b17cc",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/centos-stream-repos-9.0-22.el9.noarch.rpm"],
)

rpm(
    name = "centos-stream-repos-0__9.0-22.el9.x86_64",
    sha256 = "41eee9bd192d069e855f6b8ff43e08e350d02eeab8f47a23aa2fc517936b17cc",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-stream-repos-9.0-22.el9.noarch.rpm"],
)

rpm(
    name = "containers-common-2__1-55.el9.aarch64",
    sha256 = "49527f5b1007eb1954f954692f85b6e2d22a109027418ac685e5d25337f86665",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/containers-common-1-55.el9.aarch64.rpm"],
)

rpm(
    name = "containers-common-2__1-55.el9.x86_64",
    sha256 = "c44ecd693e242ba7dffc21b58cc9ced8d8be549324c50854c2509a5f7b6306ac",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/containers-common-1-55.el9.x86_64.rpm"],
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
    name = "coreutils-single-0__8.32-34.el9.aarch64",
    sha256 = "9ab931a79d42f2cf38ef98283603792abbef8c99d7cc112e04c69d0a66fb074c",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/coreutils-single-8.32-34.el9.aarch64.rpm"],
)

rpm(
    name = "coreutils-single-0__8.32-34.el9.x86_64",
    sha256 = "fd6001340bdba2e7b49b6dee004dc7e54e5b2393bdb0c9de9ca2e8801e39e671",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/coreutils-single-8.32-34.el9.x86_64.rpm"],
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
    name = "cracklib-dicts-0__2.9.6-27.el9.x86_64",
    sha256 = "01df2a72fcdf988132e82764ce1a22a5a9513fa253b54e17d23058bdb53c2d85",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/cracklib-dicts-2.9.6-27.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/01df2a72fcdf988132e82764ce1a22a5a9513fa253b54e17d23058bdb53c2d85",
    ],
)

rpm(
    name = "crun-0__1.8.7-1.el9.aarch64",
    sha256 = "256f9ab7db504e1ea952f9bcc4f7273c0d9d4c6ba399800b9ac0ce9381a9ea68",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/crun-1.8.7-1.el9.aarch64.rpm"],
)

rpm(
    name = "crun-0__1.8.7-1.el9.x86_64",
    sha256 = "73c1e2587880bc188e737023ab638bfc76e706d5001ddf2af40d3fc91fbc8bdc",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/crun-1.8.7-1.el9.x86_64.rpm"],
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
    name = "crypto-policies-0__20230731-1.git94f0e2c.el9.aarch64",
    sha256 = "c210b20f244bb840cb182e2e4c2887ac5b2a54a95a6e628537cde96f90e5326b",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/crypto-policies-20230731-1.git94f0e2c.el9.noarch.rpm"],
)

rpm(
    name = "crypto-policies-0__20230731-1.git94f0e2c.el9.x86_64",
    sha256 = "c210b20f244bb840cb182e2e4c2887ac5b2a54a95a6e628537cde96f90e5326b",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/crypto-policies-20230731-1.git94f0e2c.el9.noarch.rpm"],
)

rpm(
    name = "crypto-policies-scripts-0__20230731-1.git94f0e2c.el9.aarch64",
    sha256 = "0ed55fb0ada718bd09ac137fbe74ddfa8dc4dc6c5ad8798e61501ab87ed1d1a2",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/crypto-policies-scripts-20230731-1.git94f0e2c.el9.noarch.rpm"],
)

rpm(
    name = "crypto-policies-scripts-0__20230731-1.git94f0e2c.el9.x86_64",
    sha256 = "0ed55fb0ada718bd09ac137fbe74ddfa8dc4dc6c5ad8798e61501ab87ed1d1a2",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/crypto-policies-scripts-20230731-1.git94f0e2c.el9.noarch.rpm"],
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
    name = "curl-0__7.76.1-26.el9.aarch64",
    sha256 = "d5caffb79146129040cc3240749e38539ed21f0575891f91635a831e3979c403",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/curl-7.76.1-26.el9.aarch64.rpm"],
)

rpm(
    name = "curl-0__7.76.1-26.el9.x86_64",
    sha256 = "15014b23637f9291d298d34a5d4cdacc483364dc0751573b00c2a22f9b644764",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/curl-7.76.1-26.el9.x86_64.rpm"],
)

rpm(
    name = "cyrus-sasl-lib-0__2.1.27-21.el9.aarch64",
    sha256 = "898d7094964022ca527a6596550b8d46499b3274f8c6a1ee632a98961012d80c",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/cyrus-sasl-lib-2.1.27-21.el9.aarch64.rpm"],
)

rpm(
    name = "cyrus-sasl-lib-0__2.1.27-21.el9.x86_64",
    sha256 = "fd4292a29759f9531bbc876d1818e7a83ccac76907234002f598671d7b338469",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/cyrus-sasl-lib-2.1.27-21.el9.x86_64.rpm"],
)

rpm(
    name = "dbus-1__1.12.20-8.el9.aarch64",
    sha256 = "29c244f31d9f3ae910a6b95d4d5534cdf1ea4870fc277e29876a10cf3bd193ae",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/dbus-1.12.20-8.el9.aarch64.rpm"],
)

rpm(
    name = "dbus-1__1.12.20-8.el9.x86_64",
    sha256 = "d13d52df79bb9a0a1795530a5ce1134c9c92a2a7c401dfc3827ee8bf02f60018",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-1.12.20-8.el9.x86_64.rpm"],
)

rpm(
    name = "dbus-broker-0__28-7.el9.aarch64",
    sha256 = "28a7abe52040dcda6e5d941206ef6e5c47478fcc06a9f05c2ab7dacc2afa9f42",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/dbus-broker-28-7.el9.aarch64.rpm"],
)

rpm(
    name = "dbus-broker-0__28-7.el9.x86_64",
    sha256 = "dd65bddd728ed08dcdba5d06b5a5af9f958e5718e8cab938783241bd8f4d1131",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-broker-28-7.el9.x86_64.rpm"],
)

rpm(
    name = "dbus-common-1__1.12.20-8.el9.aarch64",
    sha256 = "ff91286d9413256c50886a0c96b3d5d0773bd25284b9a94b28b98a5215f09a56",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/dbus-common-1.12.20-8.el9.noarch.rpm"],
)

rpm(
    name = "dbus-common-1__1.12.20-8.el9.x86_64",
    sha256 = "ff91286d9413256c50886a0c96b3d5d0773bd25284b9a94b28b98a5215f09a56",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-common-1.12.20-8.el9.noarch.rpm"],
)

rpm(
    name = "device-mapper-9__1.02.195-3.el9.aarch64",
    sha256 = "e696b10edfe98a7c4e6f87358353035618e20a18e8e733c00bfe22078263fcd3",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/device-mapper-1.02.195-3.el9.aarch64.rpm"],
)

rpm(
    name = "device-mapper-9__1.02.195-3.el9.x86_64",
    sha256 = "441482e8244d2753fff19c2cfc0e17b2655fb7542bde27db540c9cfac2b76e3d",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/device-mapper-1.02.195-3.el9.x86_64.rpm"],
)

rpm(
    name = "device-mapper-libs-9__1.02.195-3.el9.aarch64",
    sha256 = "1409398162b3129e33044270537388112011d2b7a90dc2746354dfc177480599",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/device-mapper-libs-1.02.195-3.el9.aarch64.rpm"],
)

rpm(
    name = "device-mapper-libs-9__1.02.195-3.el9.x86_64",
    sha256 = "a5b4509bbd31c4cad65c8460fa2acf4c1505e6b27f38dc7b56fdab125a5fa0b8",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/device-mapper-libs-1.02.195-3.el9.x86_64.rpm"],
)

rpm(
    name = "expat-0__2.5.0-1.el9.aarch64",
    sha256 = "2163792c7a297e441d7c3c0cbef7a6da0695e44e0b16fbb796cd90ab91dfe0cb",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/expat-2.5.0-1.el9.aarch64.rpm"],
)

rpm(
    name = "expat-0__2.5.0-1.el9.x86_64",
    sha256 = "b5092845377c3505cd072a896c443abe5da21d3c6c6cb23d917db159905178a6",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/expat-2.5.0-1.el9.x86_64.rpm"],
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
    name = "gdbm-libs-1__1.19-4.el9.aarch64",
    sha256 = "4fc723b43287c971507ec7899a1517dcc91abab962707febc7fdd9c1d865ace8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gdbm-libs-1.19-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/4fc723b43287c971507ec7899a1517dcc91abab962707febc7fdd9c1d865ace8",
    ],
)

rpm(
    name = "gdbm-libs-1__1.19-4.el9.x86_64",
    sha256 = "8cd5a78cab8783dd241c52c4fcda28fb111c443887dd6d0fe38385e8383c98b3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gdbm-libs-1.19-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/8cd5a78cab8783dd241c52c4fcda28fb111c443887dd6d0fe38385e8383c98b3",
    ],
)

rpm(
    name = "glib2-0__2.68.4-11.el9.aarch64",
    sha256 = "37eaf1d4446bdb5f65c64f4bffb7e131cb55628151b47d859b9e5de909a32d9f",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glib2-2.68.4-11.el9.aarch64.rpm"],
)

rpm(
    name = "glib2-0__2.68.4-11.el9.x86_64",
    sha256 = "4f75096fde19d60137eccade0dc9451cb8baa9306c84d2aae1bd7faa8b8cff05",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glib2-2.68.4-11.el9.x86_64.rpm"],
)

rpm(
    name = "glib2-0__2.68.4-5.el9.aarch64",
    sha256 = "fa9e25b82015b5d2023d9f71582e2dc0ed13ce7fc70c29ee49797713a88b46db",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glib2-2.68.4-5.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/fa9e25b82015b5d2023d9f71582e2dc0ed13ce7fc70c29ee49797713a88b46db",
    ],
)

rpm(
    name = "glib2-0__2.68.4-5.el9.x86_64",
    sha256 = "34bc8c6f001daa8dba60aee15956d7ac124e71bd7c5c99039245a4bf6e61a8f5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glib2-2.68.4-5.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/34bc8c6f001daa8dba60aee15956d7ac124e71bd7c5c99039245a4bf6e61a8f5",
    ],
)

rpm(
    name = "glibc-0__2.34-82.el9.aarch64",
    sha256 = "f9093943ccc3df6653968441b0a116e02d0589b278266679f2c0bbe9fc0a4ea3",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-2.34-82.el9.aarch64.rpm"],
)

rpm(
    name = "glibc-0__2.34-82.el9.x86_64",
    sha256 = "fbc1d1f04ac348d5226d3ed4035fdb6e9f202a1412510d4b495de3f7f1b1a4d0",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-2.34-82.el9.x86_64.rpm"],
)

rpm(
    name = "glibc-common-0__2.34-82.el9.aarch64",
    sha256 = "85cf1b508c71169f25e5989186f05c492cd7208c5ea25e3ea87cc4b83ce6d91f",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-common-2.34-82.el9.aarch64.rpm"],
)

rpm(
    name = "glibc-common-0__2.34-82.el9.x86_64",
    sha256 = "63797813fb79c4b445bf6f9fe73368362fa1d859a9332d214d4504f14ba74b6d",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-common-2.34-82.el9.x86_64.rpm"],
)

rpm(
    name = "glibc-minimal-langpack-0__2.34-82.el9.aarch64",
    sha256 = "98536615a9719305128a19fe35e677fc44d343ec5e43afe89977cc783a1e11d2",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/glibc-minimal-langpack-2.34-82.el9.aarch64.rpm"],
)

rpm(
    name = "glibc-minimal-langpack-0__2.34-82.el9.x86_64",
    sha256 = "7a95d438bb5ed9fc4aa9a1a6772394a874bb94a599949ec8afea1261b09fc518",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-minimal-langpack-2.34-82.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gmp-6.2.0-13.el9.aarch64.rpm"],
)

rpm(
    name = "gmp-1__6.2.0-13.el9.x86_64",
    sha256 = "b6d592895ccc0fcad6106cd41800cd9d68e5384c418e53a2c3ff2ac8c8b15a33",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gmp-6.2.0-13.el9.x86_64.rpm"],
)

rpm(
    name = "gnupg2-0__2.3.3-4.el9.aarch64",
    sha256 = "ea254e4f615d3865263236e433e3fe674fd58b842134e72f07db80a50df0f0cb",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gnupg2-2.3.3-4.el9.aarch64.rpm"],
)

rpm(
    name = "gnupg2-0__2.3.3-4.el9.x86_64",
    sha256 = "03e7697ffc0ae9301c30adccfe28d3b100063e5d2c7c5f87dc21f1c56af4052f",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gnupg2-2.3.3-4.el9.x86_64.rpm"],
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
    name = "gnutls-0__3.7.6-23.el9.aarch64",
    sha256 = "0be14e945956d83deb956d47cc7684509b533b4c6f57961f404d005ebb42a113",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gnutls-3.7.6-23.el9.aarch64.rpm"],
)

rpm(
    name = "gnutls-0__3.7.6-23.el9.x86_64",
    sha256 = "6fd8a96f4632f2d08efc211d04ebf7591fecd213af1374f4ffe57b614b76552b",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gnutls-3.7.6-23.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gzip-1.12-1.el9.aarch64.rpm"],
)

rpm(
    name = "gzip-0__1.12-1.el9.x86_64",
    sha256 = "e8d7783c666a58ab870246b04eb0ea22965123fe284697d2c0e1e6dbf10ea861",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gzip-1.12-1.el9.x86_64.rpm"],
)

rpm(
    name = "iptables-libs-0__1.8.8-6.el9.aarch64",
    sha256 = "a0572f3b2eddcc18370801fd86bf6e5ed729702b63fadfc032c9855661090639",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/iptables-libs-1.8.8-6.el9.aarch64.rpm"],
)

rpm(
    name = "iptables-libs-0__1.8.8-6.el9.x86_64",
    sha256 = "c1e4ebce15d824604e777993f46b94706239044c81bc5240e9541b1ae93485a5",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/iptables-libs-1.8.8-6.el9.x86_64.rpm"],
)

rpm(
    name = "iptables-nft-0__1.8.8-6.el9.aarch64",
    sha256 = "b6779bea1dbc7b6923bf57d3656c3725fa615cdb7080561e59618f0f8a8d353e",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/iptables-nft-1.8.8-6.el9.aarch64.rpm"],
)

rpm(
    name = "iptables-nft-0__1.8.8-6.el9.x86_64",
    sha256 = "5c0a229d49b772dbe69ed646421eb402f2855baef7c767e733401a0f4fe426ca",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/iptables-nft-1.8.8-6.el9.x86_64.rpm"],
)

rpm(
    name = "jansson-0__2.14-1.el9.aarch64",
    sha256 = "23a8033dae909a6b87db199e04ecbc9798820b1b939e12d51733fed4554b9279",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/jansson-2.14-1.el9.aarch64.rpm"],
)

rpm(
    name = "jansson-0__2.14-1.el9.x86_64",
    sha256 = "c3fb9f8020f978f9b392709996e62e4ddb6cb19074635af3338487195b688f66",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/jansson-2.14-1.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/keyutils-libs-1.6.3-1.el9.aarch64.rpm"],
)

rpm(
    name = "keyutils-libs-0__1.6.3-1.el9.x86_64",
    sha256 = "aef982501694486a27411c68698886d76ec70c5cd10bfe619501e7e4c36f50a9",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/keyutils-libs-1.6.3-1.el9.x86_64.rpm"],
)

rpm(
    name = "kmod-libs-0__28-9.el9.aarch64",
    sha256 = "0e51fa74611d31585fb4e665fc4b24b0ff300821d109b3e0116ccdfc54c04789",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/kmod-libs-28-9.el9.aarch64.rpm"],
)

rpm(
    name = "kmod-libs-0__28-9.el9.x86_64",
    sha256 = "319957f8f3abe9b05b4aca442a3c633b36c8974e2dbd87f31ec66885f66e1b88",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/kmod-libs-28-9.el9.x86_64.rpm"],
)

rpm(
    name = "krb5-libs-0__1.21.1-1.el9.aarch64",
    sha256 = "348c8b97edf3ec258e3b5281af48ac22369bba8b747e0a52de1258578e91c36e",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/krb5-libs-1.21.1-1.el9.aarch64.rpm"],
)

rpm(
    name = "krb5-libs-0__1.21.1-1.el9.x86_64",
    sha256 = "3ef93138174dc618bbf4680b5df11d27cd6afb361cd02efad8bcbb5bf0769c2e",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/krb5-libs-1.21.1-1.el9.x86_64.rpm"],
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
    name = "libassuan-0__2.5.5-3.el9.aarch64",
    sha256 = "3efd507e48ef013bba5ca3c36a1c99923ded4f498827f927298d69f9fd06b1d0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libassuan-2.5.5-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/3efd507e48ef013bba5ca3c36a1c99923ded4f498827f927298d69f9fd06b1d0",
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
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libblkid-2.37.2-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/32dc0d2954245d958516ef05860485d2360e0eb697abada4968953b501dfcc7a",
    ],
)

rpm(
    name = "libblkid-0__2.37.2-1.el9.x86_64",
    sha256 = "f5cf36e8081c2d72e9dd64dd1614155857dd6e71ebb2237e5b0e11ace5481bac",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libblkid-2.37.2-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f5cf36e8081c2d72e9dd64dd1614155857dd6e71ebb2237e5b0e11ace5481bac",
    ],
)

rpm(
    name = "libblkid-0__2.37.4-15.el9.aarch64",
    sha256 = "e7f4f1a30f71feea534a79b3be03d26ae3001c6ececb7b2a5a54371e66deef78",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libblkid-2.37.4-15.el9.aarch64.rpm"],
)

rpm(
    name = "libblkid-0__2.37.4-15.el9.x86_64",
    sha256 = "519a372517cb0d466878808afceda7afd95be9278aa67b2dd311a8a886783b77",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libblkid-2.37.4-15.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libcap-2.48-9.el9.aarch64.rpm"],
)

rpm(
    name = "libcap-0__2.48-9.el9.x86_64",
    sha256 = "7d07ec8a6a0975d84c66adf21c885c41a5571ecb631055959265c60fda314111",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcap-2.48-9.el9.x86_64.rpm"],
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
    name = "libcom_err-0__1.46.5-3.el9.aarch64",
    sha256 = "a735b91094a13612830db66fd2021c9ec86c92697e526068e8b3919111cc2ba8",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libcom_err-1.46.5-3.el9.aarch64.rpm"],
)

rpm(
    name = "libcom_err-0__1.46.5-3.el9.x86_64",
    sha256 = "ef9db384c8fbfc0b8676aec1896070dc308cfc0c7b515ebbe556e0fea68318d0",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcom_err-1.46.5-3.el9.x86_64.rpm"],
)

rpm(
    name = "libcurl-minimal-0__7.76.1-26.el9.aarch64",
    sha256 = "30c63d2b75711225c578bd6e5beb442bbcb1c9de18391b260ea15895448cd876",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libcurl-minimal-7.76.1-26.el9.aarch64.rpm"],
)

rpm(
    name = "libcurl-minimal-0__7.76.1-26.el9.x86_64",
    sha256 = "b9c24d7a8e0c9e840b7d799e478870c75cb0101f8e3aaaca163d25d3f661b652",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcurl-minimal-7.76.1-26.el9.x86_64.rpm"],
)

rpm(
    name = "libdb-0__5.3.28-53.el9.aarch64",
    sha256 = "65a5743728c6c331dd8aadc9b51f261f90ffa47ffd0cfb448da8bdf28af6dd77",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libdb-5.3.28-53.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/65a5743728c6c331dd8aadc9b51f261f90ffa47ffd0cfb448da8bdf28af6dd77",
    ],
)

rpm(
    name = "libdb-0__5.3.28-53.el9.x86_64",
    sha256 = "3a44d15d695944bde4e7290800b815f98bfd9cd6f6f868cec3e8991606f556d5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libdb-5.3.28-53.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/3a44d15d695944bde4e7290800b815f98bfd9cd6f6f868cec3e8991606f556d5",
    ],
)

rpm(
    name = "libeconf-0__0.4.1-3.el9.aarch64",
    sha256 = "f2a26663f33189999b437c769bcd3069a3e919b4590c62edaac706fdb32654f5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libeconf-0.4.1-3.el9.aarch64.rpm",
    ],
)

rpm(
    name = "libeconf-0__0.4.1-3.el9.x86_64",
    sha256 = "841f2f5822dafc227f1eb70f4549fb382b326440fd22dc655dcbb37c843b1320",
    urls = [
        "https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libeconf-0.4.1-3.el9.x86_64.rpm",
    ],
)

rpm(
    name = "libevent-0__2.1.12-6.el9.aarch64",
    sha256 = "5ff00c047204190e3b2ee19f81d644c8f82ea7e8d1f36fdaaf6483f0fa3b3339",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libevent-2.1.12-6.el9.aarch64.rpm"],
)

rpm(
    name = "libevent-0__2.1.12-6.el9.x86_64",
    sha256 = "82179f6f214ddf523e143c16c3474ccf8832551c6305faf89edfbd83b3424d48",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libevent-2.1.12-6.el9.x86_64.rpm"],
)

rpm(
    name = "libfdisk-0__2.37.4-15.el9.aarch64",
    sha256 = "471af108df2956f60540f1b87f2f274906f564e9871b625940549a1b25d3adef",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libfdisk-2.37.4-15.el9.aarch64.rpm"],
)

rpm(
    name = "libfdisk-0__2.37.4-15.el9.x86_64",
    sha256 = "56ce4611076497f779467d631ec855a465a559c243635f9531eb365810e2eff5",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libfdisk-2.37.4-15.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libffi-3.4.2-8.el9.aarch64.rpm"],
)

rpm(
    name = "libffi-0__3.4.2-8.el9.x86_64",
    sha256 = "110d5008364a65b38b832949970886fdccb97762b0cdb257571cc0c84182d7d0",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libffi-3.4.2-8.el9.x86_64.rpm"],
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
    name = "libgcc-0__11.4.1-2.1.el9.aarch64",
    sha256 = "9887be82c9fdd442d58900beca4da1c1770eded459352a04c33a335a4592308e",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libgcc-11.4.1-2.1.el9.aarch64.rpm"],
)

rpm(
    name = "libgcc-0__11.4.1-2.1.el9.x86_64",
    sha256 = "41b5d12b9056caf345df988d0ed01b3d3abc6fb463789b495a0a6185b7160868",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libgcc-11.4.1-2.1.el9.x86_64.rpm"],
)

rpm(
    name = "libgcrypt-0__1.10.0-10.el9.aarch64",
    sha256 = "b5a90cb5a86ee956da8439362d8547342f240e71674e4703d87f27736dbede14",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libgcrypt-1.10.0-10.el9.aarch64.rpm"],
)

rpm(
    name = "libgcrypt-0__1.10.0-10.el9.x86_64",
    sha256 = "186ae69a1f72d3992f2f65a4cc91da856a54475f4762a69f3b5ca5d350e7edb3",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libgcrypt-1.10.0-10.el9.x86_64.rpm"],
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
    name = "libgpg-error-0__1.42-5.el9.x86_64",
    sha256 = "a1883804c376f737109f4dff06077d1912b90150a732d11be7bc5b3b67e512fe",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libgpg-error-1.42-5.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/a1883804c376f737109f4dff06077d1912b90150a732d11be7bc5b3b67e512fe",
    ],
)

rpm(
    name = "libibverbs-0__46.0-1.el9.aarch64",
    sha256 = "503d39f5db45aeaa5eb6a3d559af7a40cec54c424b03ed1653904160c858976a",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libibverbs-46.0-1.el9.aarch64.rpm"],
)

rpm(
    name = "libibverbs-0__46.0-1.el9.x86_64",
    sha256 = "ca7eae95bf6bf989574f10b0fb3cdd82e3d5c871faeb57f6271233cbdbac5cfe",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libibverbs-46.0-1.el9.x86_64.rpm"],
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
    name = "libidn2-0__2.3.0-7.el9.x86_64",
    sha256 = "f7fa1ad2fcd86beea5d4d965994c21dc98f47871faff14f73940190c754ab244",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libidn2-2.3.0-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f7fa1ad2fcd86beea5d4d965994c21dc98f47871faff14f73940190c754ab244",
    ],
)

rpm(
    name = "libksba-0__1.5.1-6.el9.aarch64",
    sha256 = "6411f65968b1347e261b6c4934c07c9ab0d9bd34a55ba61dbc6ca55388db2dd4",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libksba-1.5.1-6.el9.aarch64.rpm"],
)

rpm(
    name = "libksba-0__1.5.1-6.el9.x86_64",
    sha256 = "ff76d9798e2f040fed715968a9e67f6d5cfef59671e07575fc8d6510126b5340",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libksba-1.5.1-6.el9.x86_64.rpm"],
)

rpm(
    name = "libmnl-0__1.0.4-15.el9.aarch64",
    sha256 = "a3e80b22d57f0e2843e37eee0440a9bae92e4a0cbe75b13520be7616afd70e78",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libmnl-1.0.4-15.el9.aarch64.rpm"],
)

rpm(
    name = "libmnl-0__1.0.4-15.el9.x86_64",
    sha256 = "a70fdda85cd771ef5bf5b17c2996e4ff4d21c2e5b1eece1764a87f12e720ab68",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libmnl-1.0.4-15.el9.x86_64.rpm"],
)

rpm(
    name = "libmount-0__2.37.2-1.el9.aarch64",
    sha256 = "7ae3f2c10203d0fb0b76d3abd7f58197a62b8898572add7488de1a7570ea407d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libmount-2.37.2-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/7ae3f2c10203d0fb0b76d3abd7f58197a62b8898572add7488de1a7570ea407d",
    ],
)

rpm(
    name = "libmount-0__2.37.2-1.el9.x86_64",
    sha256 = "26191af0cc7acf9bb335ebd8b4ed357582165ee3be78fce9f4395f84ad2805ce",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libmount-2.37.2-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/26191af0cc7acf9bb335ebd8b4ed357582165ee3be78fce9f4395f84ad2805ce",
    ],
)

rpm(
    name = "libmount-0__2.37.4-15.el9.aarch64",
    sha256 = "09cb345eba67be22dd50a87b5232c41ed6da5b7a13bff57f8d951a69d9dc4196",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libmount-2.37.4-15.el9.aarch64.rpm"],
)

rpm(
    name = "libmount-0__2.37.4-15.el9.x86_64",
    sha256 = "b615d8c2fe67089c91025f48c700986c5cb62298e3ed58dbfc3c188af4e77a96",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libmount-2.37.4-15.el9.x86_64.rpm"],
)

rpm(
    name = "libnbd-0__1.16.0-1.el9.aarch64",
    sha256 = "1cb82e8ad06f5890c3047c97ce4b6a20499575bba7f70b52efa8d608e10f5f73",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/libnbd-1.16.0-1.el9.aarch64.rpm"],
)

rpm(
    name = "libnbd-0__1.16.0-1.el9.x86_64",
    sha256 = "8c52f6636324ab230d0441163405efbe3c7baf5803f3436190a3d27ac5d3836d",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libnbd-1.16.0-1.el9.x86_64.rpm"],
)

rpm(
    name = "libnetfilter_conntrack-0__1.0.9-1.el9.aarch64",
    sha256 = "6871a3371b5a9a8239606efd453b59b274040e9d8d8f0c18bdffa7264db64264",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libnetfilter_conntrack-1.0.9-1.el9.aarch64.rpm"],
)

rpm(
    name = "libnetfilter_conntrack-0__1.0.9-1.el9.x86_64",
    sha256 = "f81a0188964268ae9e1d53d99dba3ef96a65fe2fb00bc8fe6c39cedfdd364f44",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnetfilter_conntrack-1.0.9-1.el9.x86_64.rpm"],
)

rpm(
    name = "libnfnetlink-0__1.0.1-21.el9.aarch64",
    sha256 = "682c4cca565ce483ff0749dbb39b154bc080ac531c418d05890e454114c11821",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libnfnetlink-1.0.1-21.el9.aarch64.rpm"],
)

rpm(
    name = "libnfnetlink-0__1.0.1-21.el9.x86_64",
    sha256 = "64f54f412cc0ee6fe82be7557f471a06f6bf1f5bba1d6fe0ad1879e5a62d7c95",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnfnetlink-1.0.1-21.el9.x86_64.rpm"],
)

rpm(
    name = "libnftnl-0__1.2.2-1.el9.aarch64",
    sha256 = "6e2dac1414db86b13f0efbca18bd0128a122ba2b814faed1bce309200304cc86",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libnftnl-1.2.2-1.el9.aarch64.rpm"],
)

rpm(
    name = "libnftnl-0__1.2.2-1.el9.x86_64",
    sha256 = "fd75863a6dd1be0e7f1b7eed3e5f13a0efead33ba9bb05b0f8430574aa804783",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnftnl-1.2.2-1.el9.x86_64.rpm"],
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
    name = "libnl3-0__3.7.0-1.el9.aarch64",
    sha256 = "5f8ede2ff552132a369b43e7babfd5e08e0dc46b5c659a665f188dc497cb0415",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libnl3-3.7.0-1.el9.aarch64.rpm"],
)

rpm(
    name = "libnl3-0__3.7.0-1.el9.x86_64",
    sha256 = "8abf9bf3f62df66aeed157fc9f9494a2ea792eb11eb221caa17ce7f97330a2f3",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnl3-3.7.0-1.el9.x86_64.rpm"],
)

rpm(
    name = "libpcap-14__1.10.0-4.el9.aarch64",
    sha256 = "c1827185bde78c34817a75c79522963c76cd07585eeeb6961e58c6ddadc69333",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libpcap-1.10.0-4.el9.aarch64.rpm"],
)

rpm(
    name = "libpcap-14__1.10.0-4.el9.x86_64",
    sha256 = "c76c9887f6b9d218300b24f1adee1b0d9104d25152df3fcd005002d12e12399e",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libpcap-1.10.0-4.el9.x86_64.rpm"],
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
    name = "libselinux-0__3.5-1.el9.aarch64",
    sha256 = "1968d3199e772d0476df14b54b5f85a23329befc1ff7597f45d457b8dc9b0ddd",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libselinux-3.5-1.el9.aarch64.rpm"],
)

rpm(
    name = "libselinux-0__3.5-1.el9.x86_64",
    sha256 = "7e7309502af6056593e4c247f1829fd46cc7480ed46da020446ea6c2f1553bd1",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libselinux-3.5-1.el9.x86_64.rpm"],
)

rpm(
    name = "libsemanage-0__3.5-2.el9.aarch64",
    sha256 = "216f1393639dbd62fafb993478db286e7cd8ccf0a411afc46510b80e5cecba68",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libsemanage-3.5-2.el9.aarch64.rpm"],
)

rpm(
    name = "libsemanage-0__3.5-2.el9.x86_64",
    sha256 = "918842b65f93e5a4fe3582178777a3e73591acc58e127a5a0048b62eebc3e10d",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsemanage-3.5-2.el9.x86_64.rpm"],
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
    name = "libsepol-0__3.5-1.el9.aarch64",
    sha256 = "70e6cb0c9d177d512431a1a18ecb7f0bced1e08940df5463961de59fc243ab62",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libsepol-3.5-1.el9.aarch64.rpm"],
)

rpm(
    name = "libsepol-0__3.5-1.el9.x86_64",
    sha256 = "90428114387b69b45fcd7014b219a44ffd89cfecb3bb47c94ca29ab7dce5b940",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsepol-3.5-1.el9.x86_64.rpm"],
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
    name = "libsigsegv-0__2.13-4.el9.x86_64",
    sha256 = "931bd0ec7050e8c3b37a9bfb489e30af32486a3c77203f1e9113eeceaa3b0a3a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsigsegv-2.13-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/931bd0ec7050e8c3b37a9bfb489e30af32486a3c77203f1e9113eeceaa3b0a3a",
    ],
)

rpm(
    name = "libslirp-0__4.4.0-7.el9.aarch64",
    sha256 = "321ef98abb278174e60823b5f032ef8f5bee45d67a0e2b0a56e08e6ae8a7381b",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/libslirp-4.4.0-7.el9.aarch64.rpm"],
)

rpm(
    name = "libslirp-0__4.4.0-7.el9.x86_64",
    sha256 = "4d7383a18c393e909d037f64c35a8d5d01c559032a3bd760a77844986d57062a",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libslirp-4.4.0-7.el9.x86_64.rpm"],
)

rpm(
    name = "libsmartcols-0__2.37.2-1.el9.aarch64",
    sha256 = "5102aa25f42a101bbc41b9f9286300cdcc863811785e5a4da6ad90d6a1105067",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libsmartcols-2.37.2-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/5102aa25f42a101bbc41b9f9286300cdcc863811785e5a4da6ad90d6a1105067",
    ],
)

rpm(
    name = "libsmartcols-0__2.37.2-1.el9.x86_64",
    sha256 = "c62433784604a2e6571e0fcbdd4a2d60f059c5c15624207998c5f03b18d9d382",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsmartcols-2.37.2-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c62433784604a2e6571e0fcbdd4a2d60f059c5c15624207998c5f03b18d9d382",
    ],
)

rpm(
    name = "libsmartcols-0__2.37.4-15.el9.aarch64",
    sha256 = "461a1ed6f3ba43e2f2372eadf18148638b576eca97e6b913fbc283e08e765d4e",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libsmartcols-2.37.4-15.el9.aarch64.rpm"],
)

rpm(
    name = "libsmartcols-0__2.37.4-15.el9.x86_64",
    sha256 = "e538fc0ff411e62612f1430d4a97e112a0e77a7059db108c84b972aac6788462",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsmartcols-2.37.4-15.el9.x86_64.rpm"],
)

rpm(
    name = "libstdc__plus____plus__-0__11.4.1-2.1.el9.aarch64",
    sha256 = "540d33a05f6a8840423c3a863e30af8fb50cbb02026403c49199de338a266482",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libstdc++-11.4.1-2.1.el9.aarch64.rpm"],
)

rpm(
    name = "libstdc__plus____plus__-0__11.4.1-2.1.el9.x86_64",
    sha256 = "4831fea6e56920d6765b05e875485bf4d0f00ef2c4e60866782de3c6bc454ddf",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libstdc++-11.4.1-2.1.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libtasn1-4.16.0-8.el9.aarch64.rpm"],
)

rpm(
    name = "libtasn1-0__4.16.0-8.el9.x86_64",
    sha256 = "c8b13c9e1292de474e76ab80f230f86cce2e8f5f53592e168bdcaa604ed1b37d",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libtasn1-4.16.0-8.el9.x86_64.rpm"],
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
    name = "libunistring-0__0.9.10-15.el9.x86_64",
    sha256 = "11e736e44265d2d0ca0afa4c11cfe0856553c4124e534fb616e6ab61c9b59e46",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libunistring-0.9.10-15.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/11e736e44265d2d0ca0afa4c11cfe0856553c4124e534fb616e6ab61c9b59e46",
    ],
)

rpm(
    name = "liburing-0__2.3-2.el9.aarch64",
    sha256 = "433ecb131e763b7150b40f11f0a94a7252943dfb3827ca40bd0021884b2a5dc4",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/liburing-2.3-2.el9.aarch64.rpm"],
)

rpm(
    name = "liburing-0__2.3-2.el9.x86_64",
    sha256 = "7d04b63a1d183515a2471ad5813409586fc746ca7bcdc3f8b474543cfc325c9c",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/liburing-2.3-2.el9.x86_64.rpm"],
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
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libuuid-2.37.2-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/49e914c5f068ded96c050fd66c1110ec77f703369b9f0b08d85f80b822b1431d",
    ],
)

rpm(
    name = "libuuid-0__2.37.2-1.el9.x86_64",
    sha256 = "ffd8317ccc6f80524b7bf15a8157d82f36a2b9c7478bb04eb4a34c18d019e6fa",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libuuid-2.37.2-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/ffd8317ccc6f80524b7bf15a8157d82f36a2b9c7478bb04eb4a34c18d019e6fa",
    ],
)

rpm(
    name = "libuuid-0__2.37.4-15.el9.aarch64",
    sha256 = "6486b8a7e56ca99a40a080dea4f5b71ce4dfefe2692a53c49f565dc6aff9c474",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libuuid-2.37.4-15.el9.aarch64.rpm"],
)

rpm(
    name = "libuuid-0__2.37.4-15.el9.x86_64",
    sha256 = "383cde88a366f8d6bfb9fc29a33f8dc835cdaaf4302dc6fc90d87742a0ab0c00",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libuuid-2.37.4-15.el9.x86_64.rpm"],
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
    name = "libxml2-0__2.9.13-4.el9.aarch64",
    sha256 = "a007525b4b82ca2d62cec26e750ee546a4165635dbf2cb39a6e1b579bbf9c035",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libxml2-2.9.13-4.el9.aarch64.rpm"],
)

rpm(
    name = "libxml2-0__2.9.13-4.el9.x86_64",
    sha256 = "ee1a3c25255ad5821bd4a7bec9fdc45c77ae2a4671ea3ea96235305e19efec11",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libxml2-2.9.13-4.el9.x86_64.rpm"],
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
    name = "lz4-libs-0__1.9.3-5.el9.aarch64",
    sha256 = "9aa14d26393dd46c0a390cf04f939f7f759a33165bdb506f8bee0653f3b70f45",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/lz4-libs-1.9.3-5.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/9aa14d26393dd46c0a390cf04f939f7f759a33165bdb506f8bee0653f3b70f45",
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
    name = "mpfr-0__4.1.0-7.el9.x86_64",
    sha256 = "179760104aa5a31ca463c586d0f21f380ba4d0eed212eee91bd1ca513e5d7a8d",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/mpfr-4.1.0-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/179760104aa5a31ca463c586d0f21f380ba4d0eed212eee91bd1ca513e5d7a8d",
    ],
)

rpm(
    name = "nbdkit-basic-filters-0__1.30.1-2.el9.aarch64",
    sha256 = "38e603ca2272c2d4839b05555df85082aa9aee10bca02913a4d4906d72c48d49",
    urls = ["https://storage.googleapis.com/builddeps/38e603ca2272c2d4839b05555df85082aa9aee10bca02913a4d4906d72c48d49"],
)

rpm(
    name = "nbdkit-basic-filters-0__1.30.1-2.el9.x86_64",
    sha256 = "a48949f07b3b216da2ec315eea0371326af7065a6481b0015b1975ebf5617c08",
    urls = ["https://storage.googleapis.com/builddeps/a48949f07b3b216da2ec315eea0371326af7065a6481b0015b1975ebf5617c08"],
)

rpm(
    name = "nbdkit-basic-filters-0__1.34.2-1.el9.aarch64",
    sha256 = "401d9d1203c7f275cca58499b383fd326023d2353d729326dbc53be85b0d79e2",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-basic-filters-1.34.2-1.el9.aarch64.rpm"],
)

rpm(
    name = "nbdkit-basic-filters-0__1.34.2-1.el9.x86_64",
    sha256 = "58c3b29f75e1bade08a633e1c9a8a52638b7a8fd078b472a54de0dc747402df7",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-basic-filters-1.34.2-1.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.30.1-2.el9.aarch64",
    sha256 = "cab28966e1a42c9ad1e258ad8ca6f82ffd3d7f9d2797b36d55699daee2dc552b",
    urls = ["https://storage.googleapis.com/builddeps/cab28966e1a42c9ad1e258ad8ca6f82ffd3d7f9d2797b36d55699daee2dc552b"],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.30.1-2.el9.x86_64",
    sha256 = "87dbe46ebce06e20216ff7efd3207347ec3404e832094805c21e83f9d34039c2",
    urls = ["https://storage.googleapis.com/builddeps/87dbe46ebce06e20216ff7efd3207347ec3404e832094805c21e83f9d34039c2"],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.34.2-1.el9.aarch64",
    sha256 = "78256a58aa2afc1f058027f16398887765e381990fbd9053df6286e46b1b7fd9",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-curl-plugin-1.34.2-1.el9.aarch64.rpm"],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.34.2-1.el9.x86_64",
    sha256 = "2bf28e2a42386410c0efa489cc0b46a4cf2013ce3bf57b37f51e3b7d77333632",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-curl-plugin-1.34.2-1.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-gzip-filter-0__1.30.1-2.el9.aarch64",
    sha256 = "00e25f543a8f7dc26393a01a94f43b7dc688e063377bc9220ff4a48fa5a785b9",
    urls = ["https://storage.googleapis.com/builddeps/00e25f543a8f7dc26393a01a94f43b7dc688e063377bc9220ff4a48fa5a785b9"],
)

rpm(
    name = "nbdkit-gzip-filter-0__1.30.1-2.el9.x86_64",
    sha256 = "809d15266a1a243326b65425d5e1c3b1072fe5127e6131d1edde1be702c1d1c4",
    urls = ["https://storage.googleapis.com/builddeps/809d15266a1a243326b65425d5e1c3b1072fe5127e6131d1edde1be702c1d1c4"],
)

rpm(
    name = "nbdkit-gzip-filter-0__1.34.2-1.el9.aarch64",
    sha256 = "fc8f3ef9848b0a67c05f4d87d0f267de4ba28ddcde8b948d439d1dd9c9e8deda",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-gzip-filter-1.34.2-1.el9.aarch64.rpm"],
)

rpm(
    name = "nbdkit-gzip-filter-0__1.34.2-1.el9.x86_64",
    sha256 = "2c42378d92a12087d5c23339734368c4f7aad8b0f348cd887ffec446722fd639",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-gzip-filter-1.34.2-1.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-server-0__1.30.1-2.el9.aarch64",
    sha256 = "66a465ea8d131491fb581082ec7c5044c3918e6d2016db361796cd76cb4286ee",
    urls = ["https://storage.googleapis.com/builddeps/66a465ea8d131491fb581082ec7c5044c3918e6d2016db361796cd76cb4286ee"],
)

rpm(
    name = "nbdkit-server-0__1.30.1-2.el9.x86_64",
    sha256 = "3e677a5613e9fea1bc27503e282222b7131db4f177d9222948313c912c57b610",
    urls = ["https://storage.googleapis.com/builddeps/3e677a5613e9fea1bc27503e282222b7131db4f177d9222948313c912c57b610"],
)

rpm(
    name = "nbdkit-server-0__1.34.2-1.el9.aarch64",
    sha256 = "ab64e77a652532cef5d6341c8dbc87d95f0bdd1b0ccf8cdd5d4d5f05a9665870",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-server-1.34.2-1.el9.aarch64.rpm"],
)

rpm(
    name = "nbdkit-server-0__1.34.2-1.el9.x86_64",
    sha256 = "fc7d35f7a7e1102147dfce33a8fea6886d592781dd15a31adee345632c48d825",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-server-1.34.2-1.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-vddk-plugin-0__1.30.1-2.el9.x86_64",
    sha256 = "04a2e9f046d8f63628cfdd42ab7cc16a92e0f4610172b9c73ec9d19c768cc95e",
    urls = ["https://storage.googleapis.com/builddeps/04a2e9f046d8f63628cfdd42ab7cc16a92e0f4610172b9c73ec9d19c768cc95e"],
)

rpm(
    name = "nbdkit-vddk-plugin-0__1.34.2-1.el9.x86_64",
    sha256 = "27ed162e2353d4284193a7bc6b34455f31b2ed830d16a1b502f1002c19d38d12",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-vddk-plugin-1.34.2-1.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-xz-filter-0__1.30.1-2.el9.aarch64",
    sha256 = "59fe2df30e34d32b1083bbeeadb71a1112967e44d6d6529217aee0fa0084a955",
    urls = ["https://storage.googleapis.com/builddeps/59fe2df30e34d32b1083bbeeadb71a1112967e44d6d6529217aee0fa0084a955"],
)

rpm(
    name = "nbdkit-xz-filter-0__1.30.1-2.el9.x86_64",
    sha256 = "f4e7b9ea8bd7fd73e58c9ad24846a7467a9b36d9a98d851341cfded8e14ca59e",
    urls = ["https://storage.googleapis.com/builddeps/f4e7b9ea8bd7fd73e58c9ad24846a7467a9b36d9a98d851341cfded8e14ca59e"],
)

rpm(
    name = "nbdkit-xz-filter-0__1.34.2-1.el9.aarch64",
    sha256 = "badd1504e545d0b4b49d4001d9dbe6f7397e275949dcc2fa3efb3d2e85256b05",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nbdkit-xz-filter-1.34.2-1.el9.aarch64.rpm"],
)

rpm(
    name = "nbdkit-xz-filter-0__1.34.2-1.el9.x86_64",
    sha256 = "97634846b85f91152b9763e291305585da9ccc97e18017e00434056164fce9de",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-xz-filter-1.34.2-1.el9.x86_64.rpm"],
)

rpm(
    name = "ncurses-base-0__6.2-10.20210508.el9.aarch64",
    sha256 = "00ba56b28a3a85c3c03387bb7abeca92597c8a5fac7f53d48410ca2a20fd8065",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/ncurses-base-6.2-10.20210508.el9.noarch.rpm",
    ],
)

rpm(
    name = "ncurses-base-0__6.2-10.20210508.el9.x86_64",
    sha256 = "00ba56b28a3a85c3c03387bb7abeca92597c8a5fac7f53d48410ca2a20fd8065",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ncurses-base-6.2-10.20210508.el9.noarch.rpm",
    ],
)

rpm(
    name = "ncurses-libs-0__6.2-10.20210508.el9.aarch64",
    sha256 = "0ccfc9eeb99be404367bf6157db2d1a6fb9ed479247f578501594e08e8f7080c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/ncurses-libs-6.2-10.20210508.el9.aarch64.rpm",
    ],
)

rpm(
    name = "ncurses-libs-0__6.2-10.20210508.el9.x86_64",
    sha256 = "f4ead70a508051ed338499b35605b5b2b5bccde19c9e83f7e4b948f171b542ff",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ncurses-libs-6.2-10.20210508.el9.x86_64.rpm",
    ],
)

rpm(
    name = "netavark-2__1.7.0-1.el9.aarch64",
    sha256 = "875133282b0460f2a294a1d83296e56bfdca0382a7b65f184fd20a9b751d72ec",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/netavark-1.7.0-1.el9.aarch64.rpm"],
)

rpm(
    name = "netavark-2__1.7.0-1.el9.x86_64",
    sha256 = "3862a4f4ac8b51cbeb6dbbfd407bb72db29e68e4eecfea86ac914e9bfc1059ce",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/netavark-1.7.0-1.el9.x86_64.rpm"],
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
    name = "nettle-0__3.8-3.el9.aarch64",
    sha256 = "94386170c99bb195481806f20ae034f246e863fc02a1eeaddf88212ae545f826",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/nettle-3.8-3.el9.aarch64.rpm"],
)

rpm(
    name = "nettle-0__3.8-3.el9.x86_64",
    sha256 = "ed956f9e018ab00d6ddf567487dd6bbcdc634d27dd69b485b416c6cf40026b82",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/nettle-3.8-3.el9.x86_64.rpm"],
)

rpm(
    name = "nftables-1__1.0.4-10.el9.aarch64",
    sha256 = "03f20f6478763b1e4691d946929110da8571f6cc07c6977ff5234f382cc19698",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/nftables-1.0.4-10.el9.aarch64.rpm"],
)

rpm(
    name = "nftables-1__1.0.4-10.el9.x86_64",
    sha256 = "94a1828fabb047dacfc637f73831b7d2f2a2ff82984984c4af125f5efe0f2329",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/nftables-1.0.4-10.el9.x86_64.rpm"],
)

rpm(
    name = "nginx-1__1.22.1-3.module_el9__plus__259__plus__78986a65.aarch64",
    sha256 = "ac91d7c232028a86fd04e841cc800b4d70ddac6623ba102a59b951b11e9a2396",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nginx-1.22.1-3.module_el9+259+78986a65.aarch64.rpm"],
)

rpm(
    name = "nginx-1__1.22.1-3.module_el9__plus__259__plus__78986a65.x86_64",
    sha256 = "03462b42fdb807012a94a1e31ba995f70c17a03ae96ec160b546384b321c22d8",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nginx-1.22.1-3.module_el9+259+78986a65.x86_64.rpm"],
)

rpm(
    name = "nginx-core-1__1.22.1-3.module_el9__plus__259__plus__78986a65.aarch64",
    sha256 = "53c1e5c47b62f9e58ecc12562a8e6dac2f6de23ae0f2bc2f2614a953698a000a",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nginx-core-1.22.1-3.module_el9+259+78986a65.aarch64.rpm"],
)

rpm(
    name = "nginx-core-1__1.22.1-3.module_el9__plus__259__plus__78986a65.x86_64",
    sha256 = "58a0498713d288426d05be41ae5df87433b4052283077a5c49541bce1f0aaf95",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nginx-core-1.22.1-3.module_el9+259+78986a65.x86_64.rpm"],
)

rpm(
    name = "nginx-filesystem-1__1.22.1-3.module_el9__plus__259__plus__78986a65.aarch64",
    sha256 = "d751401a97a6c2a9fa4132063a3f358faf1e25427d9e91da60afa2ba49905342",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nginx-filesystem-1.22.1-3.module_el9+259+78986a65.noarch.rpm"],
)

rpm(
    name = "nginx-filesystem-1__1.22.1-3.module_el9__plus__259__plus__78986a65.x86_64",
    sha256 = "d751401a97a6c2a9fa4132063a3f358faf1e25427d9e91da60afa2ba49905342",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nginx-filesystem-1.22.1-3.module_el9+259+78986a65.noarch.rpm"],
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
    name = "npth-0__1.6-8.el9.x86_64",
    sha256 = "a7da4ef003bc60045bc60dae299b703e7f1db326f25208fb922ce1b79e2882da",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/npth-1.6-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/a7da4ef003bc60045bc60dae299b703e7f1db326f25208fb922ce1b79e2882da",
    ],
)

rpm(
    name = "numactl-libs-0__2.0.14-7.el9.aarch64",
    sha256 = "288250e514a6d1e4299656c1b68d49653cc92060a35024631c80fc0f206cf433",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/numactl-libs-2.0.14-7.el9.aarch64.rpm"],
)

rpm(
    name = "numactl-libs-0__2.0.14-7.el9.x86_64",
    sha256 = "7a3bc16b3fee48c53e0f54a7cb4cd3857eb1be3984d58da3bdf2c297d6b55af1",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/numactl-libs-2.0.14-7.el9.x86_64.rpm"],
)

rpm(
    name = "openldap-0__2.6.3-1.el9.aarch64",
    sha256 = "2e6e4097eb6b282c94511ee5b96d97523e0f06610570abad918836e8d784050c",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openldap-2.6.3-1.el9.aarch64.rpm"],
)

rpm(
    name = "openldap-0__2.6.3-1.el9.x86_64",
    sha256 = "847286e28d64a2e52ff858cf09fcc659f2d2d025313249da8d0f6cbd702e51cb",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openldap-2.6.3-1.el9.x86_64.rpm"],
)

rpm(
    name = "openssl-1__3.0.7-24.el9.aarch64",
    sha256 = "783a074fc19e611f191c680e3721dc66f2538d5bcf1b0dfbb0d2395c5cf9c521",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openssl-3.0.7-24.el9.aarch64.rpm"],
)

rpm(
    name = "openssl-1__3.0.7-24.el9.x86_64",
    sha256 = "a63f38ebf62f3ee60db2939ebf554797c4a67dde9e58c38d79ca3d6c3b946593",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-3.0.7-24.el9.x86_64.rpm"],
)

rpm(
    name = "openssl-libs-1__3.0.7-24.el9.aarch64",
    sha256 = "724ebdfe345be2457c4793cc354d11b926deb2bf27672f758fb6f1c71d1b68d2",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openssl-libs-3.0.7-24.el9.aarch64.rpm"],
)

rpm(
    name = "openssl-libs-1__3.0.7-24.el9.x86_64",
    sha256 = "c61b626a5dd42c1df532a51febc1fd24029bf89046be6a5755e47e244e5481d4",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-libs-3.0.7-24.el9.x86_64.rpm"],
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
    name = "pam-0__1.5.1-15.el9.aarch64",
    sha256 = "64e3412816678c0491d22f44adeb764b681b6cac4139480a20125754034f47cb",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/pam-1.5.1-15.el9.aarch64.rpm"],
)

rpm(
    name = "pam-0__1.5.1-15.el9.x86_64",
    sha256 = "d2215206d4f6a18e5d485fe312ea891eb8c6669e4079e36676b6e4c2dd7b8bc5",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pam-1.5.1-15.el9.x86_64.rpm"],
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
    name = "pcre2-0__10.40-2.el9.aarch64",
    sha256 = "8879da4bf6f8ec1a17105a3d54130d77afad48021c7280d8edb3f63fed80c4a5",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/pcre2-10.40-2.el9.aarch64.rpm"],
)

rpm(
    name = "pcre2-0__10.40-2.el9.x86_64",
    sha256 = "8cc83f9f130e6ef50d54d75eb4050ce879d8acaf5bb616b398ad92c1ad2b3d21",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pcre2-10.40-2.el9.x86_64.rpm"],
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
    name = "pcre2-syntax-0__10.40-2.el9.aarch64",
    sha256 = "4dad144194fe6794c7621c38b6a7f917a81ceaeb3f2be25833b9b0af1181ebe2",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/pcre2-syntax-10.40-2.el9.noarch.rpm"],
)

rpm(
    name = "pcre2-syntax-0__10.40-2.el9.x86_64",
    sha256 = "4dad144194fe6794c7621c38b6a7f917a81ceaeb3f2be25833b9b0af1181ebe2",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pcre2-syntax-10.40-2.el9.noarch.rpm"],
)

rpm(
    name = "python3-0__3.9.17-2.el9.aarch64",
    sha256 = "733c7d872287738bc4c412b4c1e4208e199ac8fdcfdde58bc9b25cfa12105f60",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-3.9.17-2.el9.aarch64.rpm"],
)

rpm(
    name = "python3-0__3.9.17-2.el9.x86_64",
    sha256 = "0c908646997be2a982c6f971230ca467c1039ad5a6cb961ee86d2c0ed1586983",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-3.9.17-2.el9.x86_64.rpm"],
)

rpm(
    name = "python3-libs-0__3.9.17-2.el9.aarch64",
    sha256 = "dd9cfbb8dc9714960ff65b0ba49102fd1b7b8132bc1917a2c32aa33702f913dc",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-libs-3.9.17-2.el9.aarch64.rpm"],
)

rpm(
    name = "python3-libs-0__3.9.17-2.el9.x86_64",
    sha256 = "ceeede42edd33b2ac964dd2fce66d77363be4530cce206185ae168186696bee4",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-libs-3.9.17-2.el9.x86_64.rpm"],
)

rpm(
    name = "python3-pip-wheel-0__21.2.3-7.el9.aarch64",
    sha256 = "629fa0b71150272bfc7d4f348b04d2588a70fcdd5c3b52e176ea3a7d3de71e89",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-pip-wheel-21.2.3-7.el9.noarch.rpm"],
)

rpm(
    name = "python3-pip-wheel-0__21.2.3-7.el9.x86_64",
    sha256 = "629fa0b71150272bfc7d4f348b04d2588a70fcdd5c3b52e176ea3a7d3de71e89",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-pip-wheel-21.2.3-7.el9.noarch.rpm"],
)

rpm(
    name = "python3-setuptools-wheel-0__53.0.0-12.el9.aarch64",
    sha256 = "de1a05afcb6087cf6fc6e38b952485239a72ae719538bd255e14789e606ab2ca",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-setuptools-wheel-53.0.0-12.el9.noarch.rpm"],
)

rpm(
    name = "python3-setuptools-wheel-0__53.0.0-12.el9.x86_64",
    sha256 = "de1a05afcb6087cf6fc6e38b952485239a72ae719538bd255e14789e606ab2ca",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-setuptools-wheel-53.0.0-12.el9.noarch.rpm"],
)

rpm(
    name = "python3-systemd-0__234-18.el9.aarch64",
    sha256 = "1f8ab1b8f5fa235bb75245eab6f5685b4afdfc73aa35b1a9f7df25a4b88a7f69",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/python3-systemd-234-18.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/1f8ab1b8f5fa235bb75245eab6f5685b4afdfc73aa35b1a9f7df25a4b88a7f69",
    ],
)

rpm(
    name = "python3-systemd-0__234-18.el9.x86_64",
    sha256 = "fafd41778cd2a1f26e3df7e9c395f9a66dc823d1c09a9f29fdf6e591977c318f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/python3-systemd-234-18.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/fafd41778cd2a1f26e3df7e9c395f9a66dc823d1c09a9f29fdf6e591977c318f",
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
    name = "qemu-img-17__8.0.0-13.el9.aarch64",
    sha256 = "4e5e088c57310147f64a0dc8b7096a0fe9daac8cee6149c42687a0679923fc39",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/qemu-img-8.0.0-13.el9.aarch64.rpm"],
)

rpm(
    name = "qemu-img-17__8.0.0-13.el9.x86_64",
    sha256 = "2f2b859728eaacf2034ec755f5b1b8a35650f46bb11ec8005d59400ceaefaefa",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-img-8.0.0-13.el9.x86_64.rpm"],
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
    name = "sed-0__4.8-9.el9.x86_64",
    sha256 = "a2c5d9a7f569abb5a592df1c3aaff0441bf827c9d0e2df0ab42b6c443dbc475f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/sed-4.8-9.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/a2c5d9a7f569abb5a592df1c3aaff0441bf827c9d0e2df0ab42b6c443dbc475f",
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
    name = "setup-0__2.13.7-9.el9.aarch64",
    sha256 = "e1b7458eff8a50015cdfaef129aeebf663ffd70a5b94f4e3318a7603023de8ae",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/setup-2.13.7-9.el9.noarch.rpm"],
)

rpm(
    name = "setup-0__2.13.7-9.el9.x86_64",
    sha256 = "e1b7458eff8a50015cdfaef129aeebf663ffd70a5b94f4e3318a7603023de8ae",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/setup-2.13.7-9.el9.noarch.rpm"],
)

rpm(
    name = "shadow-utils-2__4.9-8.el9.aarch64",
    sha256 = "e425a9b6b5ba059e0d633f9193b83db4e0bef7f9c4f5b8dbeef41bbb153d6162",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/shadow-utils-4.9-8.el9.aarch64.rpm"],
)

rpm(
    name = "shadow-utils-2__4.9-8.el9.x86_64",
    sha256 = "d656b38df69084201a459e9d7084e3653a58b238a7c947e465b8db6c31104261",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/shadow-utils-4.9-8.el9.x86_64.rpm"],
)

rpm(
    name = "slirp4netns-0__1.2.1-1.el9.aarch64",
    sha256 = "59ed1f9821adefa89cdf70d331d0c372943e5bf706c48adde8c82cbedb35251d",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/slirp4netns-1.2.1-1.el9.aarch64.rpm"],
)

rpm(
    name = "slirp4netns-0__1.2.1-1.el9.x86_64",
    sha256 = "6e639219342be732d0581fd8b2505a97762faaea044f532da3719f075df75c03",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/slirp4netns-1.2.1-1.el9.x86_64.rpm"],
)

rpm(
    name = "sqlite-libs-0__3.34.1-6.el9.aarch64",
    sha256 = "14ebed56d97af9a87504d2bf4c1c52f68e514cba6fb308ef559a0ed18e51d77f",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/sqlite-libs-3.34.1-6.el9.aarch64.rpm"],
)

rpm(
    name = "sqlite-libs-0__3.34.1-6.el9.x86_64",
    sha256 = "440da6dd7ad99e29e540626efe09650add959846d00a9759f0c4a417161d911e",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/sqlite-libs-3.34.1-6.el9.x86_64.rpm"],
)

rpm(
    name = "systemd-0__252-18.el9.aarch64",
    sha256 = "b227048150dea6866efbcdb67f3f1a4f6fc89531fc3827bf11263857f442546a",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-252-18.el9.aarch64.rpm"],
)

rpm(
    name = "systemd-0__252-18.el9.x86_64",
    sha256 = "9c0c1834dcc3db548f97cdfd6a6ac65a261fd4d8224edf46e30bb88b2b2c63f1",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-252-18.el9.x86_64.rpm"],
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
    name = "systemd-libs-0__252-18.el9.aarch64",
    sha256 = "f71ca1cfedf8f65a7b7212fb9dc9fd1a8b510bd740d5e79d46acece46f933d05",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-libs-252-18.el9.aarch64.rpm"],
)

rpm(
    name = "systemd-libs-0__252-18.el9.x86_64",
    sha256 = "a572727265b31de5d74c8065f452e785484158941da4c960148e1618ab0b64dd",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-libs-252-18.el9.x86_64.rpm"],
)

rpm(
    name = "systemd-pam-0__252-18.el9.aarch64",
    sha256 = "d246ba6dac02d263d2f220321dd672f2b21cfafc9983f744efd10eacaf100979",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-pam-252-18.el9.aarch64.rpm"],
)

rpm(
    name = "systemd-pam-0__252-18.el9.x86_64",
    sha256 = "fdafbf55660897e5bf33c9cf39c73b3c7a24e4e5677272490855427053a6799b",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-pam-252-18.el9.x86_64.rpm"],
)

rpm(
    name = "systemd-rpm-macros-0__252-18.el9.aarch64",
    sha256 = "8e3fef50d8c964af111bddd370675deabbc2b3e3c68a9b9c0492463e2b51ef08",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-rpm-macros-252-18.el9.noarch.rpm"],
)

rpm(
    name = "systemd-rpm-macros-0__252-18.el9.x86_64",
    sha256 = "8e3fef50d8c964af111bddd370675deabbc2b3e3c68a9b9c0492463e2b51ef08",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-rpm-macros-252-18.el9.noarch.rpm"],
)

rpm(
    name = "tar-2__1.34-6.el9.aarch64",
    sha256 = "98a9ca5a25c6aa73b5183b3333abad062a8f82d8b9390d2b2fbdc1eea5b4fb9b",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/tar-1.34-6.el9.aarch64.rpm"],
)

rpm(
    name = "tar-2__1.34-6.el9.x86_64",
    sha256 = "9f6adb2da035d5123587a2bb401487521bd6543497003ffc6e66386d898133f3",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/tar-1.34-6.el9.x86_64.rpm"],
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
    name = "tzdata-0__2023c-1.el9.aarch64",
    sha256 = "6990005a7665404476ca1a274a5e195ca3afbb5763b51720ce2c3127cc5e6114",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/tzdata-2023c-1.el9.noarch.rpm"],
)

rpm(
    name = "tzdata-0__2023c-1.el9.x86_64",
    sha256 = "6990005a7665404476ca1a274a5e195ca3afbb5763b51720ce2c3127cc5e6114",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/tzdata-2023c-1.el9.noarch.rpm"],
)

rpm(
    name = "util-linux-0__2.37.4-15.el9.aarch64",
    sha256 = "6b4a34b132ab405fcede4b2f027998640acc1285ab91ae3146e9a7dab0d6e271",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/util-linux-2.37.4-15.el9.aarch64.rpm"],
)

rpm(
    name = "util-linux-0__2.37.4-15.el9.x86_64",
    sha256 = "b7a2259bfb358029dadb493d5d746462f1c7c0244e450ccdc7a8681ac263b9ed",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/util-linux-2.37.4-15.el9.x86_64.rpm"],
)

rpm(
    name = "util-linux-core-0__2.37.2-1.el9.aarch64",
    sha256 = "5bd360c94d20a11bac665b634569fc2597eab88280d88cd5b71be853e8331e14",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/util-linux-core-2.37.2-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/5bd360c94d20a11bac665b634569fc2597eab88280d88cd5b71be853e8331e14",
    ],
)

rpm(
    name = "util-linux-core-0__2.37.2-1.el9.x86_64",
    sha256 = "0313682867c1d07785a6d02ff87e1899f484bd1ce6348fa5c673eca78c0da2bd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/util-linux-core-2.37.2-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/0313682867c1d07785a6d02ff87e1899f484bd1ce6348fa5c673eca78c0da2bd",
    ],
)

rpm(
    name = "util-linux-core-0__2.37.4-15.el9.aarch64",
    sha256 = "8a4e235a2ead88eb2b9e5b45333e60061b6c8062abcd5c3d5c388a745541046f",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/util-linux-core-2.37.4-15.el9.aarch64.rpm"],
)

rpm(
    name = "util-linux-core-0__2.37.4-15.el9.x86_64",
    sha256 = "027d0fa1342fb2118784f910c390d33a5e145237ca7e2bceec0342dd28598232",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/util-linux-core-2.37.4-15.el9.x86_64.rpm"],
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
    name = "vim-minimal-2__8.2.2637-20.el9.aarch64",
    sha256 = "b142f0b4f853c0560a17f118cbffadd89d16296cac85287cd14d35bf8b0847f2",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/vim-minimal-8.2.2637-20.el9.aarch64.rpm"],
)

rpm(
    name = "vim-minimal-2__8.2.2637-20.el9.x86_64",
    sha256 = "5bef7d6b66ece8820a758a6b1fb99a4512dd3bdcac0774723b630bfd5144ee62",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/vim-minimal-8.2.2637-20.el9.x86_64.rpm"],
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
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/xz-libs-5.2.5-8.el9.aarch64.rpm"],
)

rpm(
    name = "xz-libs-0__5.2.5-8.el9.x86_64",
    sha256 = "ff3c88297d75c51a5f8e9d2d69f8ad1eaf8347e20920b4335a3e0fc53269ad28",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/xz-libs-5.2.5-8.el9.x86_64.rpm"],
)

rpm(
    name = "yajl-0__2.1.0-22.el9.aarch64",
    sha256 = "5f099ce8836377f6aba662e5835cc500b2e8f29cd8c9b56b22df7c564f7d209c",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/yajl-2.1.0-22.el9.aarch64.rpm"],
)

rpm(
    name = "yajl-0__2.1.0-22.el9.x86_64",
    sha256 = "907156eb13e2120402287396f92b7589515ab0cba802b99c3835dd36f6a12cdf",
    urls = ["http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/yajl-2.1.0-22.el9.x86_64.rpm"],
)

rpm(
    name = "zlib-0__1.2.11-32.el9.aarch64",
    sha256 = "1b99ee6c18e92f2a727c39668941273c67f25eef18f7e9fe4febd484e9a80dbd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/zlib-1.2.11-32.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/1b99ee6c18e92f2a727c39668941273c67f25eef18f7e9fe4febd484e9a80dbd",
    ],
)

rpm(
    name = "zlib-0__1.2.11-32.el9.x86_64",
    sha256 = "59b0101c691ea180b992d338b372852c1d1607931c472c6ee22056e2fb099505",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/zlib-1.2.11-32.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/59b0101c691ea180b992d338b372852c1d1607931c472c6ee22056e2fb099505",
    ],
)

rpm(
    name = "zlib-0__1.2.11-40.el9.aarch64",
    sha256 = "dfba73a51e7d01bf239d6bc58270814da76081c9666a2ae0ce6d28d0a479e766",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/zlib-1.2.11-40.el9.aarch64.rpm"],
)

rpm(
    name = "zlib-0__1.2.11-40.el9.x86_64",
    sha256 = "8a9f51eac4658d4d05c883cbef15ae7b08acf274a46b4c4d9d28a3e2ae9f5b47",
    urls = ["http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/zlib-1.2.11-40.el9.x86_64.rpm"],
)
