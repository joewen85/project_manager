ARG GO_BUILDER_IMAGE=registry.cn-guangzhou.aliyuncs.com/joe/golang:1.25-alpine
ARG APP_RUNTIME_IMAGE=registry.cn-guangzhou.aliyuncs.com/joe/alpine:latest
ARG GO_PROXY=https://proxy.golang.org,direct

FROM --platform=$BUILDPLATFORM ${GO_BUILDER_IMAGE} AS builder
ARG GO_PROXY
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG TARGETVARIANT
WORKDIR /app
COPY go.mod go.sum ./
ENV GOPROXY=${GO_PROXY}
RUN go mod download
COPY . .
RUN set -eux; \
    GOARM_VALUE=""; \
    if [ "${TARGETARCH}" = "arm" ]; then \
      GOARM_VALUE="${TARGETVARIANT#v}"; \
      if [ -z "${GOARM_VALUE}" ]; then GOARM_VALUE="7"; fi; \
    fi; \
    export CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}"; \
    if [ -n "${GOARM_VALUE}" ]; then export GOARM="${GOARM_VALUE}"; fi; \
    go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM ${APP_RUNTIME_IMAGE}
WORKDIR /app
COPY --from=builder /out/server /app/server
EXPOSE 8080
CMD ["/app/server"]
