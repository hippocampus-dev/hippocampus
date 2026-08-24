pub fn prepare_loopback(
    _device_name: String,
    _rate: u32,
    _channels: u8,
) -> Result<(), error::Error> {
    Ok(())
}

pub fn capture_device<S, F>(
    _device_name: S,
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
