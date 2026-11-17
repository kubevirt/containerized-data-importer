def register_all_toolchains():
    native.register_toolchains(
        # The CentOS Stream 10 one first: it only matches with --config=cs10.
        "//bazel/toolchain/s390x-none-linux-gnu:s390x_linux_toolchain_cs10",
        "//bazel/toolchain/s390x-none-linux-gnu:s390x_linux_toolchain",
        "//bazel/toolchain/aarch64-none-linux-gnu:aarch64_linux_toolchain",
    )
