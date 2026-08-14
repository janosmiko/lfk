# Build stage
FROM golang:1.26.6-alpine@sha256:af8d6740070b8906d12eae1c3e3ea0957fb63f492051ea05e354c38ef9fe88df AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 go build -ldflags "\
    -s -w \
    -X github.com/janosmiko/lfk/internal/version.Version=${VERSION} \
    -X github.com/janosmiko/lfk/internal/version.GitCommit=${GIT_COMMIT} \
    -X github.com/janosmiko/lfk/internal/version.BuildDate=${BUILD_DATE}" \
    -o /lfk .

# Runtime stage
FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

RUN apk add --no-cache \
    ca-certificates \
    helm \
    kubectl \
    && addgroup -S lfk \
    && adduser -S lfk -G lfk

COPY --from=builder /lfk /usr/local/bin/lfk
ENV TERM=xterm-256color
ENV COLORTERM=truecolor

USER lfk

# Default kubeconfig mount point
VOLUME ["/home/lfk/.kube"]

ENTRYPOINT ["lfk"]
