use clap::Parser;
mod template;

#[derive(Parser, Debug)]
#[clap(about, version, author)]
struct Args {
    // repositories to add
    #[clap(short, long)]
    with_remote: Vec<String>,
}

fn main() {
    let args = Args::parse();
    
    for v in args.with_remote {
        println!("{}", v);
    }
}
