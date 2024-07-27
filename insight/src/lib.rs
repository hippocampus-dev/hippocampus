#![feature(type_ascription)]

pub mod core;
pub mod cpu_usage;
pub mod http;
pub mod https;
pub mod ui;

#[derive(clap::Parser, Clone, Debug, Default)]
pub struct Args {
    #[clap(short, long)]
    pid: Option<u32>,
    #[clap(short, long)]
    debug: bool,
    #[clap(long, default_value = "/usr/lib/libssl.so")]
    libssl_path: String,
}

#[derive(Clone, Debug)]
pub enum ResultEvent {
    L7(String),
    TracePipe(String),
}

#[derive(Clone, Debug)]
pub enum HistogramEvent {
    CPUUsage(std::collections::HashMap<String, crate::core::types::Histogram>),
}

#[derive(Clone, Debug)]
pub enum GaugeEvent {
    CPUUtilization(f64),
}

#[derive(Clone, Debug)]
pub enum Event {
    Input(termion::event::Event),
    Histogram(HistogramEvent),
    Gauge(GaugeEvent),
    Result(ResultEvent),
}
