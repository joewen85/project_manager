ARG GO_BUILDER_IMAGE=golang:1.23-alpine
ARG APP_RUNTIME_IMAGE=alpine:3.20
ARG GO_PROXY=https://proxy.golang.org,direct

FROM ${GO_BUILDER_IMAGE} AS builder
ARG GO_PROXY
WORKDIR /app
COPY go.mod go.sum ./
ENV GOPROXY=${GO_PROXY}
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

FROM ${APP_RUNTIME_IMAGE}
WORKDIR /app
COPY --from=builder /app/server /app/server
EXPOSE 8080
CMD ["/app/server"]
