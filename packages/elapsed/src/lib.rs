pub mod prelude {
    pub use elapsed_macro::elapsed;
    pub use serde_json;
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct SerializableTime {
    #[serde(with = "approx_instant")]
    time: std::time::Instant,
}

impl SerializableTime {
    pub fn new(time: std::time::Instant) -> Self {
        Self { time }
    }

    pub fn elapsed(&self) -> std::time::Duration {
        self.time.elapsed()
    }
}

mod approx_instant {
    use serde::{Deserialize, Serialize};

    pub fn serialize<S>(instant: &std::time::Instant, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let system_now = std::time::SystemTime::now();
        let approx = system_now
            .checked_sub(instant.elapsed())
            .ok_or(serde::ser::Error::custom(
                "error occured while calculating elapsed time",
            ))?;
        approx.serialize(serializer)
    }

    pub fn deserialize<'de, D>(deserializer: D) -> Result<std::time::Instant, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        let de = std::time::SystemTime::deserialize(deserializer)?;
        let system_now = std::time::SystemTime::now();
        let instant_now = std::time::Instant::now();
        let duration = system_now
            .duration_since(de)
            .map_err(serde::de::Error::custom)?;
        let approx = instant_now
            .checked_sub(duration)
            .ok_or(serde::de::Error::custom(
                "error occured while calculating elapsed time",
            ))?;
        Ok(approx)
    }
}
