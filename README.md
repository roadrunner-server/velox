# velox

Replacement for the roadrunner-binary. Automated build system for the RR and roadrunner-plugins.

1. Installation:

```shell
go install github.com/roadrunner-server/velox/vx@master
```

2. Configuration sample: (filename - `plugins.toml`)

```toml
[velox]
build_args = ['-trimpath', '-ldflags', '-s -X github.com/roadrunner-server/roadrunner/v2/internal/meta.version=v2.8.0-alpha.1 -X github.com/roadrunner-server/roadrunner/v2/internal/meta.buildTime=foo']

[roadrunner]
ref = "master"

[github_token]
token = ""

[log]
level = "debug"
mode = "development"

[plugins]
# ref -> master, commit or tag
logger = { ref = "master", owner = "roadrunner-server", repository = "logger" }
temporal = { ref = "master", owner = "temporalio", repository = "roadrunner-temporal" }
metrics = { ref = "master", owner = "roadrunner-server", repository = "metrics" }
cache = { ref = "master", owner = "roadrunner-server", repository = "cache" }
reload = { ref = "master", owner = "roadrunner-server", repository = "reload" }
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
```

3. Usage:
```shell
vx build -c=plugins.toml -o=~/Downloads
```
