#[derive(Clone, Debug)]
struct UnionFind {
    parent: Vec<usize>,
    size: Vec<usize>,
}

impl UnionFind {
    fn new(n: std::num::NonZero<usize>) -> UnionFind {
        let mut vec = vec![0; n.get()];
        for i in 0..n.get() {
            vec[i] = i;
        }
        UnionFind {
            parent: vec,
            size: vec![0; n.get()],
        }
    }

    fn union(&mut self, a: usize, b: usize) {
        let ap = self.find(a);
        let bp = self.find(b);
        if self.size[ap] > self.size[bp] {
            self.parent[bp] = ap;
        } else {
            self.parent[ap] = bp;
            if self.size[ap] == self.size[bp] {
                self.size[bp] += 1;
            }
        }
    }

    fn find(&mut self, x: usize) -> usize {
        if x == self.parent[x] {
            x
        } else {
            self.parent[x] = self.find(self.parent[x]);
            self.parent[x]
        }
    }

    fn size(&mut self, x: usize) -> usize {
        let p = self.find(x);
        self.size[p]
    }
}

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
