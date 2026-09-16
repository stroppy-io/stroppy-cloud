FROM docker.stroppy.io/library/alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40 AS unpack
ARG TARGETARCH
RUN test "$TARGETARCH" = amd64
ADD --checksum=sha256:1bc328a8b484694aeba0cc4d988887acb6e58c6848addc6f242655358ca8d893 https://github.com/stroppy-io/pg-noop/releases/download/v0.1.2/pg-noop-x86_64-unknown-linux-musl.tar.xz /tmp/pg-noop.tar.xz
RUN tar -xJf /tmp/pg-noop.tar.xz -C /tmp

FROM docker.stroppy.io/library/alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40
RUN apk add --no-cache netcat-openbsd
COPY --from=unpack /tmp/pg-noop-x86_64-unknown-linux-musl/pgnoop /usr/local/bin/pgnoop
COPY --from=unpack /tmp/pg-noop-x86_64-unknown-linux-musl/LICENSE-* /usr/share/licenses/pg-noop/
USER 65532:65532
ENV PGNOOP_HOST=0.0.0.0 PGNOOP_PORT=5432
EXPOSE 5432
ENTRYPOINT ["/usr/local/bin/pgnoop"]
