# --- Stage 1: build frontend ---
FROM node:22-alpine AS web

WORKDIR /web
COPY web/package.json web/yarn.lock ./
RUN yarn install --frozen-lockfile
COPY web/ .
RUN yarn build

# --- Stage 2: build Go binary ---
FROM golang:1.25.5-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Copy built frontend into embed location.
COPY --from=web /web/dist cmd/api/web/dist

ARG VERSION=0.0.0
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPRIVATE="github.com/stroppy-io" go build \
	-ldflags="-w -s -X github.com/stroppy-io/hatchet-workflow/internal/core/build.Version=$VERSION -X github.com/stroppy-io/hatchet-workflow/internal/core/build.ServiceName=api" \
	-trimpath \
	-v -o /app/bin/api ./cmd/api

# --- Stage 3: minimal runtime ---
FROM gcr.io/distroless/static-debian11

WORKDIR /root/
COPY --from=build /app/bin/api .

EXPOSE 8090
CMD ["./api"]
