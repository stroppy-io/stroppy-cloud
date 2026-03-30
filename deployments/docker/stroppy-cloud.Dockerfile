FROM golang:1.25.5-alpine AS build

WORKDIR /app

ARG VERSION=0.0.0

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
	-ldflags="-w -s -X github.com/stroppy-io/hatchet-workflow/internal/core/build.Version=$VERSION -X github.com/stroppy-io/hatchet-workflow/internal/core/build.ServiceName=stroppy-cloud" \
	-trimpath \
	-v -o /app/bin/stroppy-cloud "./cmd/cli"

FROM ubuntu:22.04

RUN apt-get update && apt-get install -y --no-install-recommends \
	bash curl ca-certificates wget sudo gnupg lsb-release \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=build /app/bin/stroppy-cloud /usr/local/bin/stroppy-cloud

EXPOSE 8080 9090

ENTRYPOINT ["/usr/local/bin/stroppy-cloud"]
