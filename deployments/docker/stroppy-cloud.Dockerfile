# Stage 1: Build SPA
FROM node:22-alpine AS spa

WORKDIR /app/web
COPY web/package.json web/yarn.lock* ./
RUN yarn install --frozen-lockfile 2>/dev/null || yarn install
COPY web/ .
RUN npx vite build

# Stage 2: Build Go binary (with embedded SPA)
FROM golang:1.25.5-alpine AS build

WORKDIR /app

ARG VERSION=0.0.0

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Copy built SPA into web/dist/ so go:embed picks it up
COPY --from=spa /app/web/dist ./web/dist

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
	-ldflags="-w -s -X github.com/stroppy-io/stroppy-cloud/internal/core/build.Version=$VERSION -X github.com/stroppy-io/stroppy-cloud/internal/core/build.ServiceName=stroppy-cloud" \
	-trimpath \
	-v -o /app/bin/stroppy-cloud "./cmd/cli"

# Stage 3: Runtime
FROM ubuntu:22.04

RUN apt-get update && apt-get install -y --no-install-recommends \
	bash curl ca-certificates wget sudo gnupg lsb-release \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=build /app/bin/stroppy-cloud /usr/local/bin/stroppy-cloud

EXPOSE 8080 9090

ENTRYPOINT ["/usr/local/bin/stroppy-cloud"]
