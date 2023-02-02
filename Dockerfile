# https://docs.docker.com/buildx/working-with-buildx/
# TARGETPLATFORM if not empty OR linux/amd64 by default
FROM --platform=${TARGETPLATFORM:-linux/amd64} golang:1.20-alpine as builder

# app version and build date must be passed during image building (version without any prefix).
# e.g.: `docker build --build-arg "APP_VERSION=1.2.3" --build-arg "BUILD_TIME=$(date +%FT%T%z)" .`
ARG APP_VERSION="undefined"
ARG BUILD_TIME="undefined"

COPY . /src
WORKDIR /src

# arguments to pass on each go tool link invocation
ENV LDFLAGS="-s \
-X github.com/roadrunner-server/velox/internal/version.version=$APP_VERSION \
-X github.com/roadrunner-server/velox/internal/version.buildTime=$BUILD_TIME"

# verbose
RUN set -x
RUN go mod download
RUN go mod tidy -go 1.19

RUN CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o ./velox ./cmd/vx

FROM --platform=${TARGETPLATFORM:-linux/amd64} golang:1.20-alpine

# use same build arguments for image labels
ARG APP_VERSION="undefined"
ARG BUILD_TIME="undefined"

LABEL \
    org.opencontainers.image.title="velox" \
    org.opencontainers.image.description="Automated build system for the RR and roadrunner-plugins" \
    org.opencontainers.image.url="https://roadrunner.dev" \
    org.opencontainers.image.source="https://github.com/roadrunner-server/velox" \
    org.opencontainers.image.vendor="SpiralScout" \
    org.opencontainers.image.version="$APP_VERSION" \
    org.opencontainers.image.created="$BUILD_TIME" \
    org.opencontainers.image.licenses="MIT"

# copy required files from builder image
COPY --from=builder /src/velox /usr/bin/vx
COPY --from=builder /src/velox.toml /etc/velox.toml

# use roadrunner binary as image entrypoint
ENTRYPOINT ["/usr/bin/vx"]