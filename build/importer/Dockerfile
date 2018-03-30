FROM alpine:3.3
RUN mkdir /data
ARG entrypoint
ARG runArgs
ENV RUN_ARGS=$runArgs
COPY ./$entrypoint ./run-binary
RUN apk update && apk add qemu-img \
  xz \
  qemu-system-or32 \
  qemu-system-mips64el \
  qemu-system-ppc64 \
  qemu-system-arm \
  qemu-system-sh4eb \
  qemu-system-xtensa \
  qemu-system-sparc64 \
  qemu-system-lm32 \
  qemu-system-mipsel \
  qemu-system-cris \
  qemu-system-alpha \
  qemu-system-ppc \
  qemu-system-unicore32 \
  qemu-system-microblaze \
  qemu-system-m68k \
  qemu-system-x86_64 \
  qemu-system-aarch64 \
  qemu-system-ppcemb \
  qemu-system-i386 \
  qemu-system-xtensaeb \
  qemu-system-mips64 \
  qemu-system-moxie \
  qemu-system-s390x \
  qemu-system-tricore \
  qemu-system-mips \
  qemu-system-microblazeel \
  qemu-system-sh4 \
  qemu-system-sparc
RUN apk add ca-certificates && rm -rf /var/cache/apk/*
ENTRYPOINT ./run-binary $RUN_ARGS
