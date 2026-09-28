pub fn bytes_to_i16_samples(bytes: &[u8]) -> Vec<i16> {
    let mut samples = Vec::with_capacity(bytes.len() / 2);

    for chunk in bytes.chunks_exact(2) {
        samples.push(i16::from_le_bytes([chunk[0], chunk[1]]));
    }

    samples
}

pub fn i16_samples_to_bytes(samples: &[i16]) -> Vec<u8> {
    let mut bytes = Vec::with_capacity(samples.len() * 2);

    for &sample in samples {
        bytes.extend_from_slice(&sample.to_le_bytes());
    }

    bytes
}
