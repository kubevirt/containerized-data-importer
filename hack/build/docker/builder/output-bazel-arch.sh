PLATFORM=$(uname -m)
case ${PLATFORM} in
x86_64* | i?86_64* | amd64*)
    ARCH="x86_64"
    ;;
aarch64* | arm64*)
    ARCH="arm64"
    ;;
s390x)
    ARCH="s390x"
    ;;
ppc64le)
    ARCH="ppc64le"
    ;;
*)
    echo "invalid Arch, only support x86_64, aarch64, s390x"
    exit 1
    ;;
esac

echo $ARCH
