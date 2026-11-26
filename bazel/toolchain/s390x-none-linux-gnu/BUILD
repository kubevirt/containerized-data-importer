package(default_visibility = ["//visibility:public"])

load(":cc_toolchain_config.bzl", "cc_toolchain_config")

filegroup(name = "empty")

cc_toolchain_config(name = "s390x_toolchain_config")

cc_toolchain_config(
    name = "s390x_toolchain_cs10_config",
    gcc_major = "14",
    identifier = "s390x-toolchain-cs10",
    sysroot_id = "centos-stream-10",
)

cc_toolchain(
    name = "s390x_toolchain",
    all_files = ":empty",
    compiler_files = ":empty",
    dwp_files = ":empty",
    linker_files = ":empty",
    objcopy_files = ":empty",
    strip_files = ":empty",
    toolchain_config = ":s390x_toolchain_config",
    toolchain_identifier = "s390x-toolchain",
)

cc_toolchain(
    name = "s390x_toolchain_cs10",
    all_files = ":empty",
    compiler_files = ":empty",
    dwp_files = ":empty",
    linker_files = ":empty",
    objcopy_files = ":empty",
    strip_files = ":empty",
    toolchain_config = ":s390x_toolchain_cs10_config",
    toolchain_identifier = "s390x-toolchain-cs10",
)

cc_toolchain_suite(
    name = "gcc_toolchain",
    tags = ["manual"],
    toolchains = {
        "s390x": ":s390x_toolchain",
    },
)

toolchain(
    name = "s390x_linux_toolchain",
    exec_compatible_with = [
        "@platforms//os:linux",
        "@platforms//cpu:x86_64",
    ],
    target_compatible_with = [
        "@platforms//os:linux",
        "@platforms//cpu:s390x",
    ],
    toolchain = ":s390x_toolchain",
    toolchain_type = "@bazel_tools//tools/cpp:toolchain_type",
)

# Selected with --config=cs10; registered before s390x_linux_toolchain (see toolchain.bzl).
toolchain(
    name = "s390x_linux_toolchain_cs10",
    exec_compatible_with = [
        "@platforms//os:linux",
        "@platforms//cpu:x86_64",
    ],
    target_compatible_with = [
        "@platforms//os:linux",
        "@platforms//cpu:s390x",
    ],
    target_settings = ["//bazel/centos_stream:is_cs10"],
    toolchain = ":s390x_toolchain_cs10",
    toolchain_type = "@bazel_tools//tools/cpp:toolchain_type",
)
