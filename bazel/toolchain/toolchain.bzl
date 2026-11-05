def register_all_toolchains():
    native.register_toolchains(
        "//bazel/toolchain/s390x-none-linux-gnu:s390x_linux_toolchain",
        "//bazel/toolchain/ppc64le-none-linux-gnu:ppc64le_linux_toolchain",
        "//bazel/toolchain/aarch64-none-linux-gnu:aarch64_linux_toolchain",
    )
