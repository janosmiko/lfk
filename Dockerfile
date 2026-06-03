# Build stage
FROM golang:1.26.4-alpine@sha256:f23e8b227fb4493eabe03bede4d5a32d04092da71962f1fb79b5f7d1e6c2a17f AS builder

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
FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11

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
