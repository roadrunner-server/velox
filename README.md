# velox

Replacement for the roadrunner-binary. Automated build system for the RR and roadrunner-plugins.

### 1. Installation:

- Docker:

```dockerfile
# https://docs.docker.com/buildx/working-with-buildx/
# TARGETPLATFORM if not empty OR linux/amd64 by default
FROM --platform=${TARGETPLATFORM:-linux/amd64} spiralscout/velox:latest as velox

# app version and build date must be passed during image building (version without any prefix).
# e.g.: `docker build --build-arg "APP_VERSION=1.2.3" --build-arg "BUILD_TIME=$(date +%FT%T%z)" .`
ARG APP_VERSION="undefined"
ARG BUILD_TIME="undefined"

# copy your configuration into the docker
COPY velox.toml .

# we don't need CGO
ENV CGO_ENABLED=0

# RUN build
RUN vx build -c velox.toml -o /usr/bin/

FROM --platform=${TARGETPLATFORM:-linux/amd64} php:8.1-cli

# copy required files from builder image
COPY --from=velox /usr/bin/rr /usr/bin/rr

# use roadrunner binary as image entrypoint
CMD ["/usr/bin/rr"]
```

- `go install` command:
```shell
go install github.com/roadrunner-server/velox/vx@latest
```

- Or download velox binary from the [releases page](https://github.com/roadrunner-server/velox/releases) and unpack to your `PATH`.

## Configuration:

```toml
# filename - `plugins.toml`

[velox]
build_args = ['-trimpath', '-ldflags', '-s -X github.com/roadrunner-server/roadrunner/v2/internal/meta.version=v2.10.1 -X github.com/roadrunner-server/roadrunner/v2/internal/meta.buildTime=10:00:00']

[roadrunner]
ref = "v2.10.1"

[github]
    [github.token]
    token = "token"

    [github.plugins]
    # ref -> master, commit or tag
    logger = { ref = "master", owner = "roadrunner-server", repository = "logger" }
    temporal = { ref = "master", owner = "temporalio", repository = "roadrunner-temporal" }
    metrics = { ref = "master", owner = "roadrunner-server", repository = "metrics" }
    cache = { ref = "master", owner = "roadrunner-server", repository = "cache" }
    reload = { ref = "master", owner = "roadrunner-server", repository = "reload" }
    otel = { ref = "master", owner = "roadrunner-server", repository = "otel" }
    server = { ref = "master", owner = "roadrunner-server", repository = "server" }
    service = { ref = "master", owner = "roadrunner-server", repository = "service" }
    amqp = { ref = "master", owner = "roadrunner-server", repository = "amqp" }
    beanstalk = { ref = "master", owner = "roadrunner-server", repository = "beanstalk" }
    boltdb = { ref = "master", owner = "roadrunner-server", repository = "boltdb" }
    broadcast = { ref = "master", owner = "roadrunner-server", repository = "broadcast" }
    fileserver = { ref = "master", owner = "roadrunner-server", repository = "fileserver" }
    grpc = { ref = "master", owner = "roadrunner-server", repository = "grpc" }
    gzip = { ref = "master", owner = "roadrunner-server", repository = "gzip" }
    headers = { ref = "master", owner = "roadrunner-server", repository = "headers" }
    http = { ref = "master", owner = "roadrunner-server", repository = "http" }
    jobs = { ref = "master", owner = "roadrunner-server", repository = "jobs" }
    memory = { ref = "master", owner = "roadrunner-server", repository = "memory" }
    nats = { ref = "master", owner = "roadrunner-server", repository = "nats" }
    new_relic = { ref = "master", owner = "roadrunner-server", repository = "new_relic" }
    prometheus = { ref = "master", owner = "roadrunner-server", repository = "prometheus" }
    redis = { ref = "master", owner = "roadrunner-server", repository = "redis" }
    sqs = { ref = "master", owner = "roadrunner-server", repository = "sqs" }
    static = { ref = "master", owner = "roadrunner-server", repository = "static" }
    status = { ref = "master", owner = "roadrunner-server", repository = "status" }
    kv = { ref = "master", owner = "roadrunner-server", repository = "kv" }
    memcached = { ref = "master", owner = "roadrunner-server", repository = "memcached" }
    tcp = { ref = "master", owner = "roadrunner-server", repository = "tcp" }

[gitlab]
    [gitlab.token]
    # api, read-api, read-repo
    token = "token"
    
    [gitlab.endpoint]
    endpoint = "https://gitlab.com"

    [gitlab.plugins]
    # ref -> master, commit or tag
    test_plugin_1 = { ref = "main", owner = "rustatian", repository = "36405203" }
    test_plugin_2 = { ref = "main", owner = "rustatian", repository = "36405235" }

[log]
level = "debug"
mode = "development"
```

### 3. Usage:

```shell
vx build -c plugins.toml -o ~/Downloads
```
Where:
- `-c` - path to the configuration
- `-o` - path where to put the RR binary
