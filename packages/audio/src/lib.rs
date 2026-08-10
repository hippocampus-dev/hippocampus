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

pub fn convert_channels(samples: &[i16], src: u8, dst: u8) -> Vec<i16> {
    if src == dst {
        return samples.to_vec();
    }

    if src > 1 && dst == 1 {
        let mut result = Vec::with_capacity(samples.len() / src as usize);

        for chunk in samples.chunks_exact(src as usize) {
            result.push((chunk.iter().map(|&i| i as i32).sum::<i32>() / chunk.len() as i32) as i16);
        }

        result
    } else if src == 1 && dst > 1 {
        let mut result = Vec::with_capacity(samples.len() * dst as usize);

        for &sample in samples {
            for _ in 0..dst {
                result.push(sample);
            }
        }

        result
    } else {
        samples.to_vec()
    }
}

pub fn resample(samples: &[i16], src: u32, dst: u32) -> Vec<i16> {
    if src == dst {
        return samples.to_vec();
    }

    if src > dst {
        downsample_mean(samples, src, dst)
    } else {
        upsample_linear(samples, src, dst)
    }
}

pub fn downsample_mean(samples: &[i16], src: u32, dst: u32) -> Vec<i16> {
    let output_size = (samples.len() as f64 * dst as f64 / src as f64) as usize;
    let mut result = Vec::with_capacity(output_size);

    let samples_per_output = (src as f64 / dst as f64).ceil() as usize;
    let mut input_index = 0;

    while input_index < samples.len() {
        let end_index = std::cmp::min(input_index + samples_per_output, samples.len());

        if input_index < end_index {
            let sum = samples[input_index..end_index]
                .iter()
                .map(|&i| i as i32)
                .sum::<i32>();
            let count = (end_index - input_index) as i32;
            let average = sum / count;

            result.push(average as i16);
        }

        input_index += (src as f64 / dst as f64) as usize;
    }

    result
}

pub fn upsample_linear(samples: &[i16], src: u32, dst: u32) -> Vec<i16> {
    let output_size = (samples.len() as f64 * dst as f64 / src as f64) as usize;
    let mut result = Vec::with_capacity(output_size);

    let ratio = src as f64 / dst as f64;
    let input_size = samples.len();

    for output_index in 0..output_size {
        let src_index_f = output_index as f64 * ratio;
        let src_index = src_index_f as usize;

        if src_index + 1 < input_size {
            let next_input_index = src_index + 1;
            let fraction = src_index_f - src_index as f64;

            let interpolated_value = samples[src_index] as f64 * (1.0 - fraction)
                + samples[next_input_index] as f64 * fraction;

            result.push(interpolated_value as i16);
        } else if src_index < input_size {
            result.push(samples[src_index]);
        }
    }

    result
}
