FROM alpine:3.22
RUN apk add --no-cache ca-certificates
WORKDIR /workspace
COPY stroppy /usr/local/bin/stroppy
ENTRYPOINT ["/usr/local/bin/stroppy"]
