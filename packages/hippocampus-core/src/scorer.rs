pub mod tf_idf;

pub trait Scorer {
    fn calculate(&self, _: &Parameter) -> Result<f64, error::Error>;
}

#[derive(Clone, Debug)]
pub struct Parameter {
    pub documents_count: i64,
    pub positions_count: i64,
}
