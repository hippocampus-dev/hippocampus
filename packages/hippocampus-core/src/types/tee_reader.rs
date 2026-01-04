#[derive(Clone, Debug)]
pub struct TeeReader<R: std::io::Read, W: std::io::Write> {
    reader: R,
    writer: W,
}

impl<R: std::io::Read, W: std::io::Write> TeeReader<R, W> {
    pub fn new(reader: R, writer: W) -> TeeReader<R, W> {
        TeeReader { reader, writer }
    }
}

impl<R: std::io::Read, W: std::io::Write> std::io::Read for TeeReader<R, W> {
    fn read(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
        let n = self.reader.read(buf)?;
        self.writer.write_all(&buf[..n])?;
        Ok(n)
    }
}
