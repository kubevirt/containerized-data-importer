FROM alpine:3.3
RUN mkdir /data
ARG entrypoint
ARG runArgs
ARG depFile1
ARG depFile2
ENV RUN_ARGS=$runArgs
COPY $depFile1 /
COPY $depFile2 /
COPY ./$entrypoint ./run-binary
RUN apk update && apk add qemu-img \
  xz \
  gzip
RUN apk add ca-certificates && rm -rf /var/cache/apk/*
ENTRYPOINT ["./run-binary", "-alsologtostderr"]
CMD $RUN_ARGS
