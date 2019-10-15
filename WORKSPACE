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

# Pull base image fedora29
container_pull(
    name = "fedora",
    digest = "sha256:1123bb2879415bd8694ceca7af2eeb149fcd22a44adb7d4bd371e919594db74e",
    registry = "registry.fedoraproject.org",
    repository = "fedora-minimal",
    tag = "29",
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
    sha256 = "280b5a5e60727d5b3dc32e993497b76d9cb6c0fc9bfeb531c623615ff1049460",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/29/Everything/x86_64/Packages/q/qemu-img-3.0.1-4.fc29.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/280b5a5e60727d5b3dc32e993497b76d9cb6c0fc9bfeb531c623615ff1049460",
    ],
)

http_file(
    name = "qemu-block-curl",
    sha256 = "3921fabebf993d267bb63724c3ab913d7fcde3f236f994d6593f2d7b286fb319",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/29/Everything/x86_64/Packages/q/qemu-block-curl-3.0.1-4.fc29.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/3921fabebf993d267bb63724c3ab913d7fcde3f236f994d6593f2d7b286fb319",
    ],
)

http_file(
    name = "nginx",
    sha256 = "5a623df8f5c548055f05958aacdf9eda85ba0ca5f91134c2aab7f03c1636bd36",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/29/Everything/x86_64/Packages/n/nginx-1.16.1-1.fc29.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/5a623df8f5c548055f05958aacdf9eda85ba0ca5f91134c2aab7f03c1636bd36",
    ],
)

http_file(
    name = "xen-libs",
    sha256 = "ad7d8d52f5fdd4909c0ee985fee80ad9653e9cd05ecc6e924eee237b9f4960b6",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/29/Everything/x86_64/Packages/x/xen-libs-4.11.2-1.fc29.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/ad7d8d52f5fdd4909c0ee985fee80ad9653e9cd05ecc6e924eee237b9f4960b6",
    ],
)

http_file(
    name = "libaio",
    sha256 = "9a48510e729f4795cf89b895aae220903bc9576eac19b448ca6839f1dce299d1",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/29/Everything/x86_64/os/Packages/l/libaio-0.3.111-3.fc29.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/9a48510e729f4795cf89b895aae220903bc9576eac19b448ca6839f1dce299d1",
    ],
)

http_file(
    name = "capstone",
    sha256 = "3bc3142f32859569c3468d98b8ad5c9057eefaf9b58725b10a3fc46326812e9a",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/29/Everything/x86_64/Packages/c/capstone-3.0.5-2.fc29.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/3bc3142f32859569c3468d98b8ad5c9057eefaf9b58725b10a3fc46326812e9a",
    ],
)

http_file(
    name = "gperftools-lib",
    sha256 = "f2c3f315a6d2e63b0fa8d81e52af34a9ceef60a5155e109cb209c54f0a418c9f",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/29/Everything/x86_64/os/Packages/g/gperftools-libs-2.7-3.fc29.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/f2c3f315a6d2e63b0fa8d81e52af34a9ceef60a5155e109cb209c54f0a418c9f",
    ],
)

http_file(
    name = "libunwind",
    sha256 = "9b2642ab84e17cb834ddad8d89355126b1a9395f99fa52562cff7f3506b0a545",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/29/Everything/x86_64/os/Packages/l/libunwind-1.2.1-6.fc29.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/9b2642ab84e17cb834ddad8d89355126b1a9395f99fa52562cff7f3506b0a545",
    ],
)

http_file(
    name = "nginx-mimetypes",
    sha256 = "17afff16a381063ab6b80568d8bc1ea1b8c2e52c42cdb0b298a85023c7823737",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/29/Everything/x86_64/os/Packages/n/nginx-mimetypes-2.1.48-4.fc29.noarch.rpm",
        "https://storage.googleapis.com/builddeps/17afff16a381063ab6b80568d8bc1ea1b8c2e52c42cdb0b298a85023c7823737",
    ],
)

http_file(
    name = "nginx-filesystem",
    sha256 = "15e6d05d7bd74bcaf923dcb6e6df01082f97e29ac0de5c0d225cc899f3250dad",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/29/Everything/x86_64/Packages/n/nginx-filesystem-1.16.1-1.fc29.noarch.rpm",
        "https://storage.googleapis.com/builddeps/15e6d05d7bd74bcaf923dcb6e6df01082f97e29ac0de5c0d225cc899f3250dad",
    ],
)

http_file(
    name = "buildah",
    sha256 = "8b17997fee2cced970c816d69ce7e3f9584ddf0b6576740c139a4146092055a4",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/29/Everything/x86_64/Packages/b/buildah-1.11.3-2.fc29.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/8b17997fee2cced970c816d69ce7e3f9584ddf0b6576740c139a4146092055a4",
    ],
)

http_file(
    name = "libseccomp",
    sha256 = "5d2f08ad8fa40b23bfdd60479b4b75218f3c33d44e7e793e68df644a1e311967",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/29/Everything/x86_64/Packages/l/libseccomp-2.4.1-0.fc29.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/5d2f08ad8fa40b23bfdd60479b4b75218f3c33d44e7e793e68df644a1e311967",
    ],
)

http_file(
    name = "device-mapper-libs",
    sha256 = "0696965252b154567957ffb06faf0bb44965ede2277c0eabe62077c2db5d10fb",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/29/Everything/x86_64/Packages/d/device-mapper-libs-1.02.154-1.fc29.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/0696965252b154567957ffb06faf0bb44965ede2277c0eabe62077c2db5d10fb",
    ],
)

http_file(
    name = "containers-common",
    sha256 = "b982d62d2b1fbf93609061ba627b71d91d5c185f9b9dc23534e16f4cb7dfddb7",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/29/Everything/x86_64/Packages/c/containers-common-0.1.37-2.fc29.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/b982d62d2b1fbf93609061ba627b71d91d5c185f9b9dc23534e16f4cb7dfddb7",
    ],
)

http_file(
    name = "tar",
    sha256 = "a9c15e3c3ffaf06077c9ad513e8b92dcc22317fc073a3c325fe0ea94e7599668",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/releases/29/Everything/x86_64/os//Packages/t/tar-1.30-6.fc29.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/a9c15e3c3ffaf06077c9ad513e8b92dcc22317fc073a3c325fe0ea94e7599668",
    ],
)

http_file(
    name = "skopeo",
    sha256 = "7f50955623687eb4d2eca707f25f622c55db604f326294052851ec21bab1e7bd",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/29/Everything/x86_64/Packages/s/skopeo-0.1.37-2.fc29.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/7f50955623687eb4d2eca707f25f622c55db604f326294052851ec21bab1e7bd",
    ],
)

http_file(
    name = "ostree-libs",
    sha256 = "4e01ceb34cfbf153f0e2e15220007217d6b99521812adfbed01e983c852ded00",
    urls = [
        "http://download.fedoraproject.org/pub/fedora/linux/updates/29/Everything/x86_64/Packages/o/ostree-libs-2019.1-3.fc29.x86_64.rpm",
        "https://storage.googleapis.com/builddeps/4e01ceb34cfbf153f0e2e15220007217d6b99521812adfbed01e983c852ded00",
    ],
)
