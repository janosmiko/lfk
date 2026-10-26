# Build stage
FROM golang:1.27.3-alpine@sha256:b8c9b82affbbb82a020bbcf998dad2ba9516944ac7e883524642a481a9505e9c AS builder

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
FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a

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
