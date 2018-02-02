FROM alpine
COPY ./importer /importer
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
ENTRYPOINT ./importer -alsologtostderr
