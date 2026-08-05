pub mod audio_speech;
pub mod chat_completions;
pub mod debug;
pub mod health;
pub mod metrics;

pub use audio_speech::audio_speech;
pub use chat_completions::chat_completions;
pub use health::healthz;
pub use metrics::metrics;
