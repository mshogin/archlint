use crate::service::UserService;
use std::io;

mod service;
mod model;

fn main() {
    let svc = UserService::new();
    println!("Hello, world!");
}

pub fn run_app() -> Result<(), io::Error> {
    Ok(())
}
