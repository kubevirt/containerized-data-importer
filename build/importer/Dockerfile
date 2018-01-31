FROM busybox
COPY ./importer /importer
RUN mkdir /image
ENTRYPOINT ./importer -alsologtostderr
