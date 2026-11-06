#[cfg(target_os = "linux")]
mod linux;
pub mod utils;
#[cfg(target_os = "windows")]
mod windows;

#[cfg(target_os = "linux")]
pub use linux::{capture_device, prepare_loopback};

#[cfg(target_os = "windows")]
pub use windows::{capture_device, prepare_loopback};

pub enum CaptureControl {
    Continue,
    Stop,
}

#[cfg(not(any(target_os = "linux", target_os = "windows")))]
pub fn capture_device<S, F>(
    device_name: S,
    rate: u32,
    channels: u8,
    mut callback: F,
) -> Result<(), error::Error>
where
    S: AsRef<str>,
    F: FnMut(&[u8]) -> crate::CaptureControl,
{
    Ok(())
}

#[cfg(not(any(target_os = "linux", target_os = "windows")))]
pub fn prepare_loopback(device_name: String, rate: u32, channels: u8) -> Result<(), error::Error> {
    Ok(())
}

pub fn convert_channels<T>(samples: &[T], source_channels: u8, destination_channels: u8) -> Vec<T>
where
    T: Sample,
{
    if source_channels == destination_channels {
        return samples.to_vec();
    }

    if source_channels > 1 && destination_channels == 1 {
        let mut result = Vec::with_capacity(samples.len() / source_channels as usize);

        for chunk in samples.chunks_exact(source_channels as usize) {
            let sum = chunk.iter().map(|&s| s.to_f64()).sum::<f64>();
            result.push(T::from_f64(sum / chunk.len() as f64));
        }

        result
    } else if source_channels == 1 && destination_channels > 1 {
        let mut result = Vec::with_capacity(samples.len() * destination_channels as usize);

        for &sample in samples {
            for _ in 0..destination_channels {
                result.push(sample);
            }
        }

        result
    } else {
        samples.to_vec()
    }
}

pub trait Sample: Copy {
    fn to_f64(self) -> f64;
    fn from_f64(value: f64) -> Self;
}

impl Sample for i16 {
    fn to_f64(self) -> f64 {
        self as f64
    }

    fn from_f64(value: f64) -> Self {
        value as Self
    }
}

impl Sample for f32 {
    fn to_f64(self) -> f64 {
        self as f64
    }

    fn from_f64(value: f64) -> Self {
        value as Self
    }
}

pub fn resample<T>(samples: &[T], source_rate: u32, destination_rate: u32) -> Vec<T>
where
    T: Sample,
{
    if source_rate == destination_rate {
        return samples.to_vec();
    }

    let output_size =
        (samples.len() as f64 * destination_rate as f64 / source_rate as f64) as usize;
    let mut result = Vec::with_capacity(output_size);

    let ratio = source_rate as f64 / destination_rate as f64;
    let input_size = samples.len();

    for output_index in 0..output_size {
        let source_index_f = output_index as f64 * ratio;
        let source_index = source_index_f as usize;

        if source_index + 1 < input_size {
            let fraction = source_index_f - source_index as f64;
            let interpolated_value = samples[source_index].to_f64() * (1.0 - fraction)
                + samples[source_index + 1].to_f64() * fraction;
            result.push(T::from_f64(interpolated_value));
        } else if source_index < input_size {
            result.push(samples[source_index]);
        }
    }

    result
}

pub fn rms<T>(samples: &[T]) -> f64
where
    T: Sample,
{
    if samples.is_empty() {
        return 0.0;
    }

    (samples
        .iter()
        .map(|&s| s.to_f64() * s.to_f64())
        .sum::<f64>()
        / samples.len() as f64)
        .sqrt()
}

pub fn peak_rms<T>(samples: &[T], window: usize) -> f64
where
    T: Sample,
{
    let last = samples.len().saturating_sub(window);

    samples
        .chunks_exact(window)
        .map(rms)
        .chain(std::iter::once(rms(&samples[last..])))
        .fold(0.0, f64::max)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn resample_keeps_duration_for_a_non_integer_ratio() {
        let samples = vec![0.0f32; 44100 * 3];
        assert_eq!(resample(&samples, 44100, 16000).len(), 16000 * 3);
    }

    #[test]
    fn resample_returns_the_input_for_an_equal_rate() {
        let samples = vec![1i16, 2, 3];
        assert_eq!(resample(&samples, 16000, 16000), samples);
    }

    #[test]
    fn resample_returns_nothing_for_an_empty_input() {
        assert!(resample::<f32>(&[], 44100, 16000).is_empty());
    }

    #[test]
    fn rms_measures_the_level_of_a_constant_amplitude() {
        assert_eq!(rms(&[0.5f32, -0.5, 0.5, -0.5]), 0.5);
    }

    #[test]
    fn rms_returns_zero_for_an_empty_input() {
        assert_eq!(rms::<f32>(&[]), 0.0);
    }

    #[test]
    fn peak_rms_measures_the_loudest_window() {
        assert_eq!(peak_rms(&[0.0f32, 0.0, 0.5, -0.5], 2), 0.5);
    }

    #[test]
    fn peak_rms_is_unaffected_by_silence_outside_the_loudest_window() {
        assert_eq!(
            peak_rms(&[0.5f32, -0.5, 0.0, 0.0, 0.0, 0.0], 2),
            peak_rms(&[0.5f32, -0.5], 2)
        );
    }

    #[test]
    fn peak_rms_measures_a_trailing_partial_window_at_full_length() {
        assert_eq!(
            peak_rms(&[0.0f32, 0.0, 0.0, 0.5], 3),
            rms(&[0.0f32, 0.0, 0.5])
        );
    }

    #[test]
    fn peak_rms_returns_zero_for_an_empty_input() {
        assert_eq!(peak_rms::<f32>(&[], 2), 0.0);
    }

    #[test]
    fn convert_channels_averages_each_frame_when_downmixing() {
        assert_eq!(convert_channels(&[1.0f32, 3.0, 2.0, 6.0], 2, 1), [2.0, 4.0]);
    }
}
