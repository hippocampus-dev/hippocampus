#[cfg(debug_assertions)]
use elapsed::prelude::*;

#[derive(Clone, Debug)]
pub struct TfIdf {
    indexed_count: i64,
}

impl TfIdf {
    pub fn new(indexed_count: i64) -> Self {
        Self { indexed_count }
    }
}

impl crate::scorer::Scorer for TfIdf {
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self)))]
    #[cfg_attr(debug_assertions, elapsed)]
    fn calculate(&self, parameter: &crate::scorer::Parameter) -> Result<f64, error::Error> {
        let tf = parameter.positions_count as f64;
        let idf = (self.indexed_count as f64 / parameter.documents_count as f64).log2() + 1.0;
        let score = tf * idf;

        if !score.is_finite() {
            error::bail!("TF-IDF score is not finite");
        }

        Ok(score)
    }
}
