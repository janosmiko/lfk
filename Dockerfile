# Build stage
FROM golang:1.26.4-alpine@sha256:7a3e50096189ad57c9f9f865e7e4aa8585ed1585248513dc5cda498e2f41812c AS builder

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
FROM alpine:3.24@sha256:a2d49ea686c2adfe3c992e47dc3b5e7fa6e6b5055609400dc2acaeb241c829f4

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
