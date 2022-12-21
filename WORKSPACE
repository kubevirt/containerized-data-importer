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
    go_version = "1.18.8",
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
    name = "audit-libs-0__3.0.7-102.el9.aarch64",
    sha256 = "7ac1d3cc6477e12e9ec05fc5a156e65f6e60105da8d35ea2b5a95692346734be",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/audit-libs-3.0.7-102.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/7ac1d3cc6477e12e9ec05fc5a156e65f6e60105da8d35ea2b5a95692346734be",
    ],
)

rpm(
    name = "audit-libs-0__3.0.7-102.el9.x86_64",
    sha256 = "e35ad310964ada32010e18dbc0a8eaddf8efaf062b8e546ff63ba4c4930b34c0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/audit-libs-3.0.7-102.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e35ad310964ada32010e18dbc0a8eaddf8efaf062b8e546ff63ba4c4930b34c0",
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
    name = "buildah-1__1.24.1-1.el9.aarch64",
    sha256 = "0033a716501502a295629fda62804ef060fc19e24434c8750966c72ec32666f2",
    urls = ["https://storage.googleapis.com/builddeps/0033a716501502a295629fda62804ef060fc19e24434c8750966c72ec32666f2"],
)

rpm(
    name = "buildah-1__1.24.1-1.el9.x86_64",
    sha256 = "660e27249712ce08bc153d4db8877fb34a9174d02ff5e31ab0cac59850bbb7cc",
    urls = ["https://storage.googleapis.com/builddeps/660e27249712ce08bc153d4db8877fb34a9174d02ff5e31ab0cac59850bbb7cc"],
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
    name = "containers-common-2__1-33.el9.aarch64",
    sha256 = "7f98dfd5bf3462c54c838f0884d5faa9890b1852acd654f34691384dbf3c77d0",
    urls = ["https://storage.googleapis.com/builddeps/7f98dfd5bf3462c54c838f0884d5faa9890b1852acd654f34691384dbf3c77d0"],
)

rpm(
    name = "containers-common-2__1-33.el9.x86_64",
    sha256 = "5f0590338462b02526f080f23300f011dc447a367e3c95d6cb07c85214661a86",
    urls = ["https://storage.googleapis.com/builddeps/5f0590338462b02526f080f23300f011dc447a367e3c95d6cb07c85214661a86"],
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
    name = "criu-0__3.15-13.el9.aarch64",
    sha256 = "7636b0438be1a1bac09ae1088e4356aec9540b3eb03149a9dbbfb591d0933479",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/criu-3.15-13.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/7636b0438be1a1bac09ae1088e4356aec9540b3eb03149a9dbbfb591d0933479",
    ],
)

rpm(
    name = "criu-0__3.15-13.el9.x86_64",
    sha256 = "c539e8ceaa3e05262a70c8b3e0fd5e168d645ad9a8370af8877bf2411b362efd",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/criu-3.15-13.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c539e8ceaa3e05262a70c8b3e0fd5e168d645ad9a8370af8877bf2411b362efd",
    ],
)

rpm(
    name = "criu-libs-0__3.15-13.el9.aarch64",
    sha256 = "7454ac5ae41b0d7a8126348cf5bb1a244291f47682edc425a44d18b884830c03",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/criu-libs-3.15-13.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/7454ac5ae41b0d7a8126348cf5bb1a244291f47682edc425a44d18b884830c03",
    ],
)

rpm(
    name = "criu-libs-0__3.15-13.el9.x86_64",
    sha256 = "e3ee56af0c754deec13e57898a928ef974f73b617b7aa39b08b55072b520b1ec",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/criu-libs-3.15-13.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e3ee56af0c754deec13e57898a928ef974f73b617b7aa39b08b55072b520b1ec",
    ],
)

rpm(
    name = "crun-0__1.4.4-1.el9.aarch64",
    sha256 = "bc18ec016fef1cb91b9daebf5ff90c5efb4d41c3c806ac5ba3a28944d075f737",
    urls = ["https://storage.googleapis.com/builddeps/bc18ec016fef1cb91b9daebf5ff90c5efb4d41c3c806ac5ba3a28944d075f737"],
)

rpm(
    name = "crun-0__1.4.4-1.el9.x86_64",
    sha256 = "52c2bfcbae02dfed2f1308747ecc044fba95495855aa92ebee266761af03cbb8",
    urls = ["https://storage.googleapis.com/builddeps/52c2bfcbae02dfed2f1308747ecc044fba95495855aa92ebee266761af03cbb8"],
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
    name = "crypto-policies-scripts-0__20220223-1.git5203b41.el9.aarch64",
    sha256 = "e90443cfe36a2765b66508941ed97247deae73a4c39f030be7e3d1d6a1a3b99e",
    urls = ["https://storage.googleapis.com/builddeps/e90443cfe36a2765b66508941ed97247deae73a4c39f030be7e3d1d6a1a3b99e"],
)

rpm(
    name = "crypto-policies-scripts-0__20220223-1.git5203b41.el9.x86_64",
    sha256 = "e90443cfe36a2765b66508941ed97247deae73a4c39f030be7e3d1d6a1a3b99e",
    urls = ["https://storage.googleapis.com/builddeps/e90443cfe36a2765b66508941ed97247deae73a4c39f030be7e3d1d6a1a3b99e"],
)

rpm(
    name = "cryptsetup-libs-0__2.4.3-4.el9.aarch64",
    sha256 = "acec9971baa75b27a33e10ed9e61b9cfcaf12d0c6adb7bc0a6d7af260db72d59",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/cryptsetup-libs-2.4.3-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/acec9971baa75b27a33e10ed9e61b9cfcaf12d0c6adb7bc0a6d7af260db72d59",
    ],
)

rpm(
    name = "cryptsetup-libs-0__2.4.3-4.el9.x86_64",
    sha256 = "7faf8194688eec41c36a9e120e1e3554d326fdb03a1a2d1c6b3a97d3b3aa0ad1",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/cryptsetup-libs-2.4.3-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/7faf8194688eec41c36a9e120e1e3554d326fdb03a1a2d1c6b3a97d3b3aa0ad1",
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
    name = "cyrus-sasl-lib-0__2.1.27-20.el9.aarch64",
    sha256 = "6dc9aa13255624297b9b3176696a2c50c9955e52cbcead574441468aab0e6351",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/cyrus-sasl-lib-2.1.27-20.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/6dc9aa13255624297b9b3176696a2c50c9955e52cbcead574441468aab0e6351",
    ],
)

rpm(
    name = "cyrus-sasl-lib-0__2.1.27-20.el9.x86_64",
    sha256 = "f3b7aa6db0de083df5c11526c621ed1265f591b1c52bc28941df0e90cd48eb6c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/cyrus-sasl-lib-2.1.27-20.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f3b7aa6db0de083df5c11526c621ed1265f591b1c52bc28941df0e90cd48eb6c",
    ],
)

rpm(
    name = "dbus-1__1.12.20-5.el9.aarch64",
    sha256 = "3b3056c21b5d5d2db4dc76c7bfa4f00573989cfe199349792e8366b7080d1296",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/dbus-1.12.20-5.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/3b3056c21b5d5d2db4dc76c7bfa4f00573989cfe199349792e8366b7080d1296",
    ],
)

rpm(
    name = "dbus-1__1.12.20-5.el9.x86_64",
    sha256 = "bb85bd28cc162e98da53b756b988ffd9350f4dbcc186f4c6962ae047e27f83d3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-1.12.20-5.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/bb85bd28cc162e98da53b756b988ffd9350f4dbcc186f4c6962ae047e27f83d3",
    ],
)

rpm(
    name = "dbus-broker-0__28-5.el9.aarch64",
    sha256 = "21ac73d4bcba7138a0aa5f02ae612719975a44d0ea2fd1ae230629c7e9c37501",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/dbus-broker-28-5.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/21ac73d4bcba7138a0aa5f02ae612719975a44d0ea2fd1ae230629c7e9c37501",
    ],
)

rpm(
    name = "dbus-broker-0__28-5.el9.x86_64",
    sha256 = "e9efdcdcfe430e474e3a7f09596a0a5a4314692d9ae846bb1ca86ff88ef81038",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-broker-28-5.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e9efdcdcfe430e474e3a7f09596a0a5a4314692d9ae846bb1ca86ff88ef81038",
    ],
)

rpm(
    name = "dbus-common-1__1.12.20-5.el9.aarch64",
    sha256 = "150048b6fdafd4271bd6badab3f8a2e56b86967266f890770eab7578289cc773",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/dbus-common-1.12.20-5.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/150048b6fdafd4271bd6badab3f8a2e56b86967266f890770eab7578289cc773",
    ],
)

rpm(
    name = "dbus-common-1__1.12.20-5.el9.x86_64",
    sha256 = "150048b6fdafd4271bd6badab3f8a2e56b86967266f890770eab7578289cc773",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-common-1.12.20-5.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/150048b6fdafd4271bd6badab3f8a2e56b86967266f890770eab7578289cc773",
    ],
)

rpm(
    name = "device-mapper-9__1.02.183-4.el9.aarch64",
    sha256 = "b080a59493ad1fa71bdffdd1ade971feca815d0ba7fd3da627c5f710f91c1470",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/device-mapper-1.02.183-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/b080a59493ad1fa71bdffdd1ade971feca815d0ba7fd3da627c5f710f91c1470",
    ],
)

rpm(
    name = "device-mapper-9__1.02.183-4.el9.x86_64",
    sha256 = "eb8323d62250dfabb5d3b1f832e785acbb30bbb82bb5e3f28e74ce7b0bef63ae",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/device-mapper-1.02.183-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/eb8323d62250dfabb5d3b1f832e785acbb30bbb82bb5e3f28e74ce7b0bef63ae",
    ],
)

rpm(
    name = "device-mapper-libs-9__1.02.183-4.el9.aarch64",
    sha256 = "04954340e373279c3451b2854f13644a49c1242704dd85b7b6866cae90bdca51",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/device-mapper-libs-1.02.183-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/04954340e373279c3451b2854f13644a49c1242704dd85b7b6866cae90bdca51",
    ],
)

rpm(
    name = "device-mapper-libs-9__1.02.183-4.el9.x86_64",
    sha256 = "c0e3b8af40f1b996ab06e0bc7ae77c7007b993415540c0f927e3c4957d73c698",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/device-mapper-libs-1.02.183-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c0e3b8af40f1b996ab06e0bc7ae77c7007b993415540c0f927e3c4957d73c698",
    ],
)

rpm(
    name = "expat-0__2.2.10-10.el9.aarch64",
    sha256 = "60f93d53f80b071c433a94229f84e74cdd17e658004ccd49e8cb3e9ef23a9ba5",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/expat-2.2.10-10.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/60f93d53f80b071c433a94229f84e74cdd17e658004ccd49e8cb3e9ef23a9ba5",
    ],
)

rpm(
    name = "expat-0__2.2.10-10.el9.x86_64",
    sha256 = "c9a68d875495500bee342ba6d34def2a56c1c21f573568a7aa5169587bff5cc7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/expat-2.2.10-10.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/c9a68d875495500bee342ba6d34def2a56c1c21f573568a7aa5169587bff5cc7",
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
    name = "gnupg2-0__2.3.3-1.el9.aarch64",
    sha256 = "2bd0fb2786d27bf88d605dbdce8643b3f83e6f16da412f95a6adcad464507edb",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gnupg2-2.3.3-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/2bd0fb2786d27bf88d605dbdce8643b3f83e6f16da412f95a6adcad464507edb",
    ],
)

rpm(
    name = "gnupg2-0__2.3.3-1.el9.x86_64",
    sha256 = "2eea9a666e80db3023f3453dc470b212b74fb638f15d73abaf296210ecc22787",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gnupg2-2.3.3-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/2eea9a666e80db3023f3453dc470b212b74fb638f15d73abaf296210ecc22787",
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
    name = "gzip-0__1.10-8.el9.aarch64",
    sha256 = "5adcdffa636a91110da1a97e6b8d984c7b6c5ef6171947ea1d2057c8622dbba9",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/gzip-1.10-8.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/5adcdffa636a91110da1a97e6b8d984c7b6c5ef6171947ea1d2057c8622dbba9",
    ],
)

rpm(
    name = "gzip-0__1.10-8.el9.x86_64",
    sha256 = "3b5ce98a03a3336a3f32ac7a0867fbc23da702e8618bfd20d49d882d42a460f4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gzip-1.10-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/3b5ce98a03a3336a3f32ac7a0867fbc23da702e8618bfd20d49d882d42a460f4",
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
    name = "json-c-0__0.14-11.el9.x86_64",
    sha256 = "1a75404c6bc8c1369914077dc99480e73bf13a40f15fd1cd8afc792b8600adf8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/json-c-0.14-11.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/1a75404c6bc8c1369914077dc99480e73bf13a40f15fd1cd8afc792b8600adf8",
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
    name = "kmod-libs-0__28-7.el9.aarch64",
    sha256 = "e7be371f66f08c54bc464cf376fce0f09ba07d1e10843c7f9bc35abbb3c38085",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/kmod-libs-28-7.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/e7be371f66f08c54bc464cf376fce0f09ba07d1e10843c7f9bc35abbb3c38085",
    ],
)

rpm(
    name = "kmod-libs-0__28-7.el9.x86_64",
    sha256 = "0727ff3131223446158aaec88cbf8f894a9e3592e73f231a1802629518eeb64b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/kmod-libs-28-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/0727ff3131223446158aaec88cbf8f894a9e3592e73f231a1802629518eeb64b",
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
    name = "libeconf-0__0.4.1-2.el9.aarch64",
    sha256 = "082dff130121fcdb7cb3fd432de482075b5003e0d95ff4ab6d8ba02404b69d6b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libeconf-0.4.1-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/082dff130121fcdb7cb3fd432de482075b5003e0d95ff4ab6d8ba02404b69d6b",
    ],
)

rpm(
    name = "libeconf-0__0.4.1-2.el9.x86_64",
    sha256 = "1d6fe169e74daff38ad5b0d6424c4d1b14545d5974c39e4421d20838a68f5892",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libeconf-0.4.1-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/1d6fe169e74daff38ad5b0d6424c4d1b14545d5974c39e4421d20838a68f5892",
    ],
)

rpm(
    name = "libfdisk-0__2.37.2-1.el9.aarch64",
    sha256 = "dd8a5068d6b69ebdcb974e48c421b98ef763a3f1dd348aaf1cd3f7ee6b539295",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libfdisk-2.37.2-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/dd8a5068d6b69ebdcb974e48c421b98ef763a3f1dd348aaf1cd3f7ee6b539295",
    ],
)

rpm(
    name = "libfdisk-0__2.37.2-1.el9.x86_64",
    sha256 = "a41bad6e261c719224abfd6745ccb1d2a0cac9d024ca9656904001a38d7cd8c7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libfdisk-2.37.2-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/a41bad6e261c719224abfd6745ccb1d2a0cac9d024ca9656904001a38d7cd8c7",
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
    name = "libgcc-0__11.2.1-9.4.el9.aarch64",
    sha256 = "83553d747fbbe61a2e5a22604f15d38e366bf4b453c99947bc1253ddec6b5049",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libgcc-11.2.1-9.4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/83553d747fbbe61a2e5a22604f15d38e366bf4b453c99947bc1253ddec6b5049",
    ],
)

rpm(
    name = "libgcc-0__11.2.1-9.4.el9.x86_64",
    sha256 = "34443f5befca73364cc7db887c4a95a254ba662cd45d80765a77a84e3a5da59f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libgcc-11.2.1-9.4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/34443f5befca73364cc7db887c4a95a254ba662cd45d80765a77a84e3a5da59f",
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
    name = "libidn2-0__2.3.0-7.el9.x86_64",
    sha256 = "f7fa1ad2fcd86beea5d4d965994c21dc98f47871faff14f73940190c754ab244",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libidn2-2.3.0-7.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f7fa1ad2fcd86beea5d4d965994c21dc98f47871faff14f73940190c754ab244",
    ],
)

rpm(
    name = "libksba-0__1.5.1-4.el9.aarch64",
    sha256 = "40b77edb4081ffc0bd8edfd5e89efc4d22557061d33d0d980ba1e5f993c7a798",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libksba-1.5.1-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/40b77edb4081ffc0bd8edfd5e89efc4d22557061d33d0d980ba1e5f993c7a798",
    ],
)

rpm(
    name = "libksba-0__1.5.1-4.el9.x86_64",
    sha256 = "d8aab5c91906bbccb3f45eb873004bbc7dc58d5ca8d338778c444fb298fff30a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libksba-1.5.1-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/d8aab5c91906bbccb3f45eb873004bbc7dc58d5ca8d338778c444fb298fff30a",
    ],
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
    name = "libnbd-0__1.12.2-1.el9.aarch64",
    sha256 = "9c15ca579e08d8df7a6950a8d63ceafcdd724bbef54ca8c4bc299eb7a4d9b608",
    urls = ["https://storage.googleapis.com/builddeps/9c15ca579e08d8df7a6950a8d63ceafcdd724bbef54ca8c4bc299eb7a4d9b608"],
)

rpm(
    name = "libnbd-0__1.12.2-1.el9.x86_64",
    sha256 = "8516de690d958c8cab488a5961f5eecaa641e4f4af2d989b411c760853d18827",
    urls = ["https://storage.googleapis.com/builddeps/8516de690d958c8cab488a5961f5eecaa641e4f4af2d989b411c760853d18827"],
)

rpm(
    name = "libnet-0__1.2-6.el9.aarch64",
    sha256 = "7733fb01282823a98622c80962ba090c10f10dec131b0fc9d2a40dd7a8971646",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/libnet-1.2-6.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/7733fb01282823a98622c80962ba090c10f10dec131b0fc9d2a40dd7a8971646",
    ],
)

rpm(
    name = "libnet-0__1.2-6.el9.x86_64",
    sha256 = "a24ff4f7a2f722126af7541fa812f83b4524652cca2a22342f860831e4b1f2fc",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libnet-1.2-6.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/a24ff4f7a2f722126af7541fa812f83b4524652cca2a22342f860831e4b1f2fc",
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
    name = "libnl3-0__3.5.0-10.el9.aarch64",
    sha256 = "f6c6562586998f34a8be13b83d5cf72be5c309f7a9f3bcd8c614cf9b2d8075ae",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libnl3-3.5.0-10.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f6c6562586998f34a8be13b83d5cf72be5c309f7a9f3bcd8c614cf9b2d8075ae",
    ],
)

rpm(
    name = "libnl3-0__3.5.0-10.el9.x86_64",
    sha256 = "78251bdd72afa836fa1d7578fcba3f9e36d2fc5beff257e5cb95810b21f64ba0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnl3-3.5.0-10.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/78251bdd72afa836fa1d7578fcba3f9e36d2fc5beff257e5cb95810b21f64ba0",
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
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libselinux-3.3-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f14cadbedd18e37a5ecb11d112095aa3e539de58bea77fb6f2aca5f165bf788b",
    ],
)

rpm(
    name = "libselinux-0__3.3-2.el9.x86_64",
    sha256 = "8e589b8408b04cbc19564620b229b6768edbaeb9090885d2273d84b8fc2f172b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libselinux-3.3-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/8e589b8408b04cbc19564620b229b6768edbaeb9090885d2273d84b8fc2f172b",
    ],
)

rpm(
    name = "libsemanage-0__3.3-2.el9.aarch64",
    sha256 = "d17a576a02f8dc920f4922573094212f3a6d325b572052d13f8eb2282e72fc33",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libsemanage-3.3-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/d17a576a02f8dc920f4922573094212f3a6d325b572052d13f8eb2282e72fc33",
    ],
)

rpm(
    name = "libsemanage-0__3.3-2.el9.x86_64",
    sha256 = "3cf458196a076b6cd29db495dcc0c518dd0e2f25c5ad47a9843a05b0d2eccc5e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsemanage-3.3-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/3cf458196a076b6cd29db495dcc0c518dd0e2f25c5ad47a9843a05b0d2eccc5e",
    ],
)

rpm(
    name = "libsepol-0__3.3-2.el9.aarch64",
    sha256 = "74dcae8d3dceb2aac2cbb3440015419aa4fec51e485eb92ef82df057f574e0ca",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libsepol-3.3-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/74dcae8d3dceb2aac2cbb3440015419aa4fec51e485eb92ef82df057f574e0ca",
    ],
)

rpm(
    name = "libsepol-0__3.3-2.el9.x86_64",
    sha256 = "fc508147fe876706b61941a6ce554d7f7786f1ec3d097c4411fd6c7511acd289",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsepol-3.3-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/fc508147fe876706b61941a6ce554d7f7786f1ec3d097c4411fd6c7511acd289",
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
    name = "libsigsegv-0__2.13-4.el9.x86_64",
    sha256 = "931bd0ec7050e8c3b37a9bfb489e30af32486a3c77203f1e9113eeceaa3b0a3a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsigsegv-2.13-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/931bd0ec7050e8c3b37a9bfb489e30af32486a3c77203f1e9113eeceaa3b0a3a",
    ],
)

rpm(
    name = "libslirp-0__4.4.0-4.el9.aarch64",
    sha256 = "b3d35a0f47c1666bbed175469443617a6b2366bd198d8fe2e7b9fb047e058cff",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/libslirp-4.4.0-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/b3d35a0f47c1666bbed175469443617a6b2366bd198d8fe2e7b9fb047e058cff",
    ],
)

rpm(
    name = "libslirp-0__4.4.0-4.el9.x86_64",
    sha256 = "06a12c4b78f60bd866ea91e648b86f1d52369f1981b5f18b6d2880ab8a951f81",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libslirp-4.4.0-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/06a12c4b78f60bd866ea91e648b86f1d52369f1981b5f18b6d2880ab8a951f81",
    ],
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
    name = "libstdc__plus____plus__-0__11.2.1-9.4.el9.aarch64",
    sha256 = "2617e76cc2048676224a4ec834925165c7ce3a93320e3cb8bdaa698a3fe0265b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libstdc++-11.2.1-9.4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/2617e76cc2048676224a4ec834925165c7ce3a93320e3cb8bdaa698a3fe0265b",
    ],
)

rpm(
    name = "libstdc__plus____plus__-0__11.2.1-9.4.el9.x86_64",
    sha256 = "212fb5fff9f8d7539f89f9a794baa8f5aa5dd9628f3abf8bbfe1193af669f449",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libstdc++-11.2.1-9.4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/212fb5fff9f8d7539f89f9a794baa8f5aa5dd9628f3abf8bbfe1193af669f449",
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
    name = "libxml2-0__2.9.13-1.el9.aarch64",
    sha256 = "f05dcfa270bb67326cdcf8cf962aa1f309af44efe980d16b1c49eae5794dc94f",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/libxml2-2.9.13-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f05dcfa270bb67326cdcf8cf962aa1f309af44efe980d16b1c49eae5794dc94f",
    ],
)

rpm(
    name = "libxml2-0__2.9.13-1.el9.x86_64",
    sha256 = "672e19296244d3bbc0804075f6df02925d4242a0b549482fa9dbbcef03ebd430",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libxml2-2.9.13-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/672e19296244d3bbc0804075f6df02925d4242a0b549482fa9dbbcef03ebd430",
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
    name = "nbdkit-vddk-plugin-0__1.30.1-2.el9.x86_64",
    sha256 = "04a2e9f046d8f63628cfdd42ab7cc16a92e0f4610172b9c73ec9d19c768cc95e",
    urls = ["https://storage.googleapis.com/builddeps/04a2e9f046d8f63628cfdd42ab7cc16a92e0f4610172b9c73ec9d19c768cc95e"],
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
    name = "nginx-1__1.20.1-10.el9.aarch64",
    sha256 = "81917c701f3e595fa8c40736b6a958fa9df335289696f1b75ba6a6d766357671",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nginx-1.20.1-10.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/81917c701f3e595fa8c40736b6a958fa9df335289696f1b75ba6a6d766357671",
    ],
)

rpm(
    name = "nginx-1__1.20.1-10.el9.x86_64",
    sha256 = "8dd157d2198e737dee4315ddfa1785bf26007c1d2a07704fcb48299fccde9629",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nginx-1.20.1-10.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/8dd157d2198e737dee4315ddfa1785bf26007c1d2a07704fcb48299fccde9629",
    ],
)

rpm(
    name = "nginx-filesystem-1__1.20.1-10.el9.aarch64",
    sha256 = "e1641951224b4e8073b91723a784636b6d809d153833f0d04830092be6de6c0a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/nginx-filesystem-1.20.1-10.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/e1641951224b4e8073b91723a784636b6d809d153833f0d04830092be6de6c0a",
    ],
)

rpm(
    name = "nginx-filesystem-1__1.20.1-10.el9.x86_64",
    sha256 = "e1641951224b4e8073b91723a784636b6d809d153833f0d04830092be6de6c0a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nginx-filesystem-1.20.1-10.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/e1641951224b4e8073b91723a784636b6d809d153833f0d04830092be6de6c0a",
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
    name = "npth-0__1.6-8.el9.x86_64",
    sha256 = "a7da4ef003bc60045bc60dae299b703e7f1db326f25208fb922ce1b79e2882da",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/npth-1.6-8.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/a7da4ef003bc60045bc60dae299b703e7f1db326f25208fb922ce1b79e2882da",
    ],
)

rpm(
    name = "openldap-0__2.4.59-4.el9.aarch64",
    sha256 = "fedc0f9ae16bc8f36e23b3b4feaf7afa57081b01279a4aeeead2a52c11b96b56",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/openldap-2.4.59-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/fedc0f9ae16bc8f36e23b3b4feaf7afa57081b01279a4aeeead2a52c11b96b56",
    ],
)

rpm(
    name = "openldap-0__2.4.59-4.el9.x86_64",
    sha256 = "794dbcd241347d373693886981c479c74108087552b7f6d4e88a422d688eebed",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openldap-2.4.59-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/794dbcd241347d373693886981c479c74108087552b7f6d4e88a422d688eebed",
    ],
)

rpm(
    name = "openssl-1__3.0.1-18.el9.aarch64",
    sha256 = "43245613d72d721a23f5ff4825596afb93ec81484f9ea8229a5f99b583e50f55",
    urls = ["https://storage.googleapis.com/builddeps/43245613d72d721a23f5ff4825596afb93ec81484f9ea8229a5f99b583e50f55"],
)

rpm(
    name = "openssl-1__3.0.1-18.el9.x86_64",
    sha256 = "19d7e2bfbff078bdb3781bee6bf2895aa263fe5ada3d2b0f39f9e4dffe6f5fd2",
    urls = ["https://storage.googleapis.com/builddeps/19d7e2bfbff078bdb3781bee6bf2895aa263fe5ada3d2b0f39f9e4dffe6f5fd2"],
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
    name = "pam-0__1.5.1-9.el9.aarch64",
    sha256 = "fcfbc00651e9ffb6defe10da28f9826becafb20029b812b25b32608f879d28cb",
    urls = ["https://storage.googleapis.com/builddeps/fcfbc00651e9ffb6defe10da28f9826becafb20029b812b25b32608f879d28cb"],
)

rpm(
    name = "pam-0__1.5.1-9.el9.x86_64",
    sha256 = "e64caedce811645ecdd78e7b4ae83c189aa884ff1ba6445374f39186c588c52c",
    urls = ["https://storage.googleapis.com/builddeps/e64caedce811645ecdd78e7b4ae83c189aa884ff1ba6445374f39186c588c52c"],
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
    name = "protobuf-c-0__1.3.3-12.el9.aarch64",
    sha256 = "f6096a23837dcf6755968c54d9a16875f3e9a86388f936ab1e2ff28e381f3cfa",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/protobuf-c-1.3.3-12.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/f6096a23837dcf6755968c54d9a16875f3e9a86388f936ab1e2ff28e381f3cfa",
    ],
)

rpm(
    name = "protobuf-c-0__1.3.3-12.el9.x86_64",
    sha256 = "5d1091426fc81321e00c805fff53b2da159de91d6d219d20f3defdfde41bf1d4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/protobuf-c-1.3.3-12.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/5d1091426fc81321e00c805fff53b2da159de91d6d219d20f3defdfde41bf1d4",
    ],
)

rpm(
    name = "python3-0__3.9.10-2.el9.aarch64",
    sha256 = "59b8a8f1473acf95f448370d587e5672b83bcfd5693f20421d060229e5565235",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-3.9.10-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/59b8a8f1473acf95f448370d587e5672b83bcfd5693f20421d060229e5565235",
    ],
)

rpm(
    name = "python3-0__3.9.10-2.el9.x86_64",
    sha256 = "e4f0741a08e047f06370791e8b33f952ccac106cdaae43d84db950a551fa7536",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-3.9.10-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e4f0741a08e047f06370791e8b33f952ccac106cdaae43d84db950a551fa7536",
    ],
)

rpm(
    name = "python3-libs-0__3.9.10-2.el9.aarch64",
    sha256 = "e2425e2081e428200ab730890402f076f94b7d1befed2f72295dd074b3c7be4b",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-libs-3.9.10-2.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/e2425e2081e428200ab730890402f076f94b7d1befed2f72295dd074b3c7be4b",
    ],
)

rpm(
    name = "python3-libs-0__3.9.10-2.el9.x86_64",
    sha256 = "1c2203b76476a223996695dd4e2fca6743101d049b276d8a523a200dc60244f4",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-libs-3.9.10-2.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/1c2203b76476a223996695dd4e2fca6743101d049b276d8a523a200dc60244f4",
    ],
)

rpm(
    name = "python3-pip-wheel-0__21.2.3-6.el9.aarch64",
    sha256 = "8e9e72535944204b48dbcb9cb34007b4991bdb4b5223e4c5874b07c6c122c1ff",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-pip-wheel-21.2.3-6.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/8e9e72535944204b48dbcb9cb34007b4991bdb4b5223e4c5874b07c6c122c1ff",
    ],
)

rpm(
    name = "python3-pip-wheel-0__21.2.3-6.el9.x86_64",
    sha256 = "8e9e72535944204b48dbcb9cb34007b4991bdb4b5223e4c5874b07c6c122c1ff",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-pip-wheel-21.2.3-6.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/8e9e72535944204b48dbcb9cb34007b4991bdb4b5223e4c5874b07c6c122c1ff",
    ],
)

rpm(
    name = "python3-setuptools-wheel-0__53.0.0-10.el9.aarch64",
    sha256 = "e5f90b8d3c7d45172b7e54e2fe290b27bff5646be8638ddfb5086856bee56368",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/python3-setuptools-wheel-53.0.0-10.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/e5f90b8d3c7d45172b7e54e2fe290b27bff5646be8638ddfb5086856bee56368",
    ],
)

rpm(
    name = "python3-setuptools-wheel-0__53.0.0-10.el9.x86_64",
    sha256 = "e5f90b8d3c7d45172b7e54e2fe290b27bff5646be8638ddfb5086856bee56368",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-setuptools-wheel-53.0.0-10.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/e5f90b8d3c7d45172b7e54e2fe290b27bff5646be8638ddfb5086856bee56368",
    ],
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
    name = "runc-2__1.0.0-332.dev.gitbd013b6.el9.aarch64",
    sha256 = "2924679bb55a9c384c08bb87fcc59c5677b456c425fe45d897f2c6131d133d8f",
    urls = ["https://storage.googleapis.com/builddeps/2924679bb55a9c384c08bb87fcc59c5677b456c425fe45d897f2c6131d133d8f"],
)

rpm(
    name = "runc-2__1.0.0-332.dev.gitbd013b6.el9.x86_64",
    sha256 = "7f273a39912aa38de533fffb2d97124d9cb28640d9cc26eb6cc25a3384bc6eda",
    urls = ["https://storage.googleapis.com/builddeps/7f273a39912aa38de533fffb2d97124d9cb28640d9cc26eb6cc25a3384bc6eda"],
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
    name = "shadow-utils-2__4.9-3.el9.aarch64",
    sha256 = "860932f0a6faf95659c6f017ced07caf6c5c1fedae6ce63713bea4196072c7fc",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/shadow-utils-4.9-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/860932f0a6faf95659c6f017ced07caf6c5c1fedae6ce63713bea4196072c7fc",
    ],
)

rpm(
    name = "shadow-utils-2__4.9-3.el9.x86_64",
    sha256 = "46fca2ed21478e5143434da4fbd47ca4599a885fab9f8636f9c7ba54942dd27e",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/shadow-utils-4.9-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/46fca2ed21478e5143434da4fbd47ca4599a885fab9f8636f9c7ba54942dd27e",
    ],
)

rpm(
    name = "slirp4netns-0__1.1.12-4.el9.aarch64",
    sha256 = "1855b8ee835ccd26ba36786a2341cd9c939b8dcf239ab919346457edbd5ba67c",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/slirp4netns-1.1.12-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/1855b8ee835ccd26ba36786a2341cd9c939b8dcf239ab919346457edbd5ba67c",
    ],
)

rpm(
    name = "slirp4netns-0__1.1.12-4.el9.x86_64",
    sha256 = "09d80decf0824bd02e3f85a5caff9d248ce0a249abf5d78308c15be086b05485",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/slirp4netns-1.1.12-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/09d80decf0824bd02e3f85a5caff9d248ce0a249abf5d78308c15be086b05485",
    ],
)

rpm(
    name = "sqlite-libs-0__3.34.1-5.el9.aarch64",
    sha256 = "7ebb88f2eb86ae915c1fc83f6983b8dca6348a6264c68166ee14541b99bfbcdc",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/sqlite-libs-3.34.1-5.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/7ebb88f2eb86ae915c1fc83f6983b8dca6348a6264c68166ee14541b99bfbcdc",
    ],
)

rpm(
    name = "sqlite-libs-0__3.34.1-5.el9.x86_64",
    sha256 = "420bf785149b4a852ae1b3259a4bc1fd22055998af26b042ad2f06deeb345ba3",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/sqlite-libs-3.34.1-5.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/420bf785149b4a852ae1b3259a4bc1fd22055998af26b042ad2f06deeb345ba3",
    ],
)

rpm(
    name = "systemd-0__250-4.el9.aarch64",
    sha256 = "51ed04aaf861814c8deeea92db6235267f4a90ca8b3aaa49a6ed028d8d44e634",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-250-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/51ed04aaf861814c8deeea92db6235267f4a90ca8b3aaa49a6ed028d8d44e634",
    ],
)

rpm(
    name = "systemd-0__250-4.el9.x86_64",
    sha256 = "6cd93803f7bd9478e70e57264fe7f08112c9803f02def06e55c2ac0b7ffd47ad",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-250-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/6cd93803f7bd9478e70e57264fe7f08112c9803f02def06e55c2ac0b7ffd47ad",
    ],
)

rpm(
    name = "systemd-libs-0__250-4.el9.aarch64",
    sha256 = "0afc6fc8e96fb76f2183774bf309efb5bef2c0f85b68f351bece3e0385f08106",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-libs-250-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/0afc6fc8e96fb76f2183774bf309efb5bef2c0f85b68f351bece3e0385f08106",
    ],
)

rpm(
    name = "systemd-libs-0__250-4.el9.x86_64",
    sha256 = "f0a57df3dcea7a138470ffb9a4e5201edf807ce4082730dd9f0e886435df7ced",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-libs-250-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f0a57df3dcea7a138470ffb9a4e5201edf807ce4082730dd9f0e886435df7ced",
    ],
)

rpm(
    name = "systemd-pam-0__250-4.el9.aarch64",
    sha256 = "2e4fd7198e3933e070d42f469d24c93a30cd76f7a87f320c2d24031b140909ee",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-pam-250-4.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/2e4fd7198e3933e070d42f469d24c93a30cd76f7a87f320c2d24031b140909ee",
    ],
)

rpm(
    name = "systemd-pam-0__250-4.el9.x86_64",
    sha256 = "e0813882893b81892ae871cbe8524e201d820c369a0abb45a63f30508671ca7a",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-pam-250-4.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/e0813882893b81892ae871cbe8524e201d820c369a0abb45a63f30508671ca7a",
    ],
)

rpm(
    name = "systemd-rpm-macros-0__250-4.el9.aarch64",
    sha256 = "9680e77174be51be8e12c19fd6c62dc60d8ddacb135181fcb8f7b0c8618e84ec",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/systemd-rpm-macros-250-4.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/9680e77174be51be8e12c19fd6c62dc60d8ddacb135181fcb8f7b0c8618e84ec",
    ],
)

rpm(
    name = "systemd-rpm-macros-0__250-4.el9.x86_64",
    sha256 = "9680e77174be51be8e12c19fd6c62dc60d8ddacb135181fcb8f7b0c8618e84ec",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-rpm-macros-250-4.el9.noarch.rpm",
        "https://storage.googleapis.com/builddeps/9680e77174be51be8e12c19fd6c62dc60d8ddacb135181fcb8f7b0c8618e84ec",
    ],
)

rpm(
    name = "tar-2__1.34-3.el9.aarch64",
    sha256 = "6fc1c8575a94aa6a60bf2fdf40c91ec0dc355de18ccb81298af10b123b706247",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/tar-1.34-3.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/6fc1c8575a94aa6a60bf2fdf40c91ec0dc355de18ccb81298af10b123b706247",
    ],
)

rpm(
    name = "tar-2__1.34-3.el9.x86_64",
    sha256 = "4cb3bcc3e5d2e9d8820960bd9aa8fdc5bfc0d53a5f5da0a5fd9a8090b3e86150",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/tar-1.34-3.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/4cb3bcc3e5d2e9d8820960bd9aa8fdc5bfc0d53a5f5da0a5fd9a8090b3e86150",
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
    name = "util-linux-0__2.37.2-1.el9.aarch64",
    sha256 = "157c71ec3e351dcb72aa36e37e325da134d50a52a34a61a0c1b435845f3b60ca",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/Packages/util-linux-2.37.2-1.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/157c71ec3e351dcb72aa36e37e325da134d50a52a34a61a0c1b435845f3b60ca",
    ],
)

rpm(
    name = "util-linux-0__2.37.2-1.el9.x86_64",
    sha256 = "4ca41a925461daa936db284a59bf325ea061cdb39d8738e288cc19afe30a8ae8",
    urls = [
        "http://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/util-linux-2.37.2-1.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/4ca41a925461daa936db284a59bf325ea061cdb39d8738e288cc19afe30a8ae8",
    ],
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
    name = "yajl-0__2.1.0-20.el9.aarch64",
    sha256 = "baa4d36c8b7a367bc11c17082d29505df7f017dcb3e58dcb0d60d599114ff2a7",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/aarch64/os/Packages/yajl-2.1.0-20.el9.aarch64.rpm",
        "https://storage.googleapis.com/builddeps/baa4d36c8b7a367bc11c17082d29505df7f017dcb3e58dcb0d60d599114ff2a7",
    ],
)

rpm(
    name = "yajl-0__2.1.0-20.el9.x86_64",
    sha256 = "0868a3ded06f09b4e72b102a609a60b6345afc7944e7783dab5bdade4084b8d0",
    urls = [
        "http://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/yajl-2.1.0-20.el9.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/0868a3ded06f09b4e72b102a609a60b6345afc7944e7783dab5bdade4084b8d0",
    ],
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
