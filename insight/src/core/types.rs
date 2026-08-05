#[derive(Clone, Debug, Default)]
pub struct Histogram(pub Vec<u32>);

impl From<&Histogram> for Vec<(String, u32)> {
    fn from(value: &Histogram) -> Self {
        let mut i_max = -1;
        for i in 0..value.0.len() {
            let val = value.0[i];
            if val > 0 {
                i_max = i as i32;
            }
        }
        let mut v = Vec::new();
        for i in 0..i_max as usize {
            let val = value.0[i];
            let low = 1 << i;
            let high = (1 << (i + 1)) - 1;
            v.push((format!("{low}-{high}"), val))
        }
        v
    }
}

impl std::fmt::Display for Histogram {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let v: Vec<(String, u32)> = self.into();
        for (k, value) in v {
            write!(f, "{k}: {value}")?;
        }
        std::fmt::Result::Ok(())
    }
}
