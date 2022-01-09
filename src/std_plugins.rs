// All standard plugins available in the RR
pub(crate) enum PluginsList {
    AMQP(String),
    Beanstalk(String),
    BoltDB(String),
    Broadcast(String),
    Config(String),
    FileServer(String),
    GRPC(String),
    HTTP(String),
    Informer(String),
    Jobs(String),
    KV(String),
    Logger(String),
    Memcached(String),
    Memory(String),
    Metrics(String),
    NATS(String),
    Redis(String),
    Reload(String),
    Resetter(String),
    RPC(String),
    Server(String),
    Service(String),
    SQS(String),
    Status(String),
    TCP(String),

    // middleware
    Cache(String),
    Gzip(String),
    Headers(String),
    NewRelic(String),
    Prometheus(String),
    Send(String),
    Static(String),
    WebSockets(String),
}

impl PluginsList {
    pub fn GetPlugin(&self) {

    }
}