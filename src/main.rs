use std::process::exit;
use log::{info, trace, warn, error};

use clap::{App, ArgEnum, Args, Parser};
mod std_plugins;
mod template;

#[derive(Parser, Debug)]
#[clap(about = "RoadRunnerV2 build system", author = "SpiralScout", version)]
struct CliArgs {
    // repositories to add
    #[clap(long, name = "with_remote")]
    with_remote: Vec<String>,

    #[clap(long, name = "with_std")]
    with_std: Vec<String>,
}

fn main() {
    env_logger::init();
    info!("STARTING");

    let args = CliArgs::parse();
    let version = env!("CARGO_PKG_VERSION");

    for v in &args.with_remote {
        println!("{}", v);
    }

    if args.with_remote.is_empty() && args.with_std.is_empty() {
        error!("can't build RoadRunner with no plugins specified, exiting");
        exit(1);
    }

    let matcher = App::new("rrbuild").version(version).subcommand(App::new("foo")).get_matches();

    match matcher.subcommand() {
        _ => panic!("foo")
    }

    std_plugins::PluginsList::Beanstalk("dir".into()).GetPlugin()
}
