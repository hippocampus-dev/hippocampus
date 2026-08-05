//! Trained projection layer: thinker hidden states -> talker embedding space.
//!
//! Loads the GGUF exported by `notebooks/thinker-talker-projection.ipynb`
//! (F32 `fc1`/`fc2` tensors plus `projection.*` metadata) and runs the
//! fc1 -> SiLU -> fc2 forward pass on the CPU. The result stands in for the
//! talker's own text-token embeddings, so `output_dim` is the talker's
//! `n_embd`; the talker, not this layer, samples the audio codes.

const GGUF_MAGIC: &[u8; 4] = b"GGUF";
const GGUF_VERSION: u32 = 3;
const GGUF_DEFAULT_ALIGNMENT: u64 = 32;
const GGUF_TYPE_UINT32: u32 = 4;
const GGUF_TYPE_STRING: u32 = 8;
const GGML_TYPE_F32: u32 = 0;

pub struct ProjectionModel {
    pub input_dim: usize,
    pub hidden_dim: usize,
    pub output_dim: usize,
    /// Row-major `[hidden_dim, input_dim]`.
    fc1_weight: Vec<f32>,
    fc1_bias: Vec<f32>,
    /// Row-major `[output_dim, hidden_dim]`.
    fc2_weight: Vec<f32>,
    fc2_bias: Vec<f32>,
}

struct GgufCursor<'a> {
    bytes: &'a [u8],
    position: usize,
}

impl<'a> GgufCursor<'a> {
    fn take(&mut self, count: usize) -> Result<&'a [u8], error::Error> {
        let end = self
            .position
            .checked_add(count)
            .filter(|&end| end <= self.bytes.len())
            .ok_or_else(|| error::error!("Unexpected end of GGUF file"))?;
        let slice = &self.bytes[self.position..end];
        self.position = end;
        Ok(slice)
    }

    fn read_u32(&mut self) -> Result<u32, error::Error> {
        Ok(u32::from_le_bytes(self.take(4)?.try_into().unwrap()))
    }

    fn read_u64(&mut self) -> Result<u64, error::Error> {
        Ok(u64::from_le_bytes(self.take(8)?.try_into().unwrap()))
    }

    fn read_string(&mut self) -> Result<String, error::Error> {
        let length = self.read_u64()? as usize;
        String::from_utf8(self.take(length)?.to_vec())
            .map_err(|e| error::error!("Invalid UTF-8 in GGUF string: {}", e))
    }
}

struct GgufTensorInfo {
    name: String,
    /// GGUF `ne` order: fastest-varying dimension first (reverse of row-major).
    dimensions: Vec<u64>,
    offset: u64,
}

impl ProjectionModel {
    #[tracing::instrument]
    pub fn load_from_gguf(path: &std::path::Path) -> Result<Self, error::Error> {
        let bytes = std::fs::read(path).map_err(|e| {
            error::error!("Failed to read projection GGUF '{}': {}", path.display(), e)
        })?;
        let mut cursor = GgufCursor {
            bytes: &bytes,
            position: 0,
        };

        if cursor.take(4)? != GGUF_MAGIC {
            return Err(error::error!("'{}' is not a GGUF file", path.display()));
        }
        let version = cursor.read_u32()?;
        if version != GGUF_VERSION {
            return Err(error::error!(
                "Unsupported GGUF version {} in '{}' (expected {})",
                version,
                path.display(),
                GGUF_VERSION
            ));
        }
        let tensor_count = cursor.read_u64()?;
        let metadata_count = cursor.read_u64()?;

        let mut metadata_strings = std::collections::HashMap::new();
        let mut metadata_integers = std::collections::HashMap::new();
        for _ in 0..metadata_count {
            let key = cursor.read_string()?;
            let value_type = cursor.read_u32()?;
            match value_type {
                GGUF_TYPE_STRING => {
                    metadata_strings.insert(key, cursor.read_string()?);
                }
                GGUF_TYPE_UINT32 => {
                    metadata_integers.insert(key, cursor.read_u32()?);
                }
                _ => {
                    return Err(error::error!(
                        "Unsupported GGUF metadata type {} for key '{}'",
                        value_type,
                        key
                    ));
                }
            }
        }

        let architecture = metadata_strings
            .get("general.architecture")
            .map(String::as_str)
            .unwrap_or("");
        if architecture != "projection" {
            return Err(error::error!(
                "'{}' has architecture '{}', expected 'projection'",
                path.display(),
                architecture
            ));
        }

        let dimension = |key: &str| -> Result<usize, error::Error> {
            metadata_integers
                .get(key)
                .map(|&value| value as usize)
                .ok_or_else(|| error::error!("Missing '{}' in projection GGUF", key))
        };
        let input_dim = dimension("projection.input_dim")?;
        let hidden_dim = dimension("projection.hidden_dim")?;
        let output_dim = dimension("projection.output_dim")?;
        if input_dim == 0 || hidden_dim == 0 || output_dim == 0 {
            return Err(error::error!(
                "Projection dimensions must be non-zero: {} -> {} -> {}",
                input_dim,
                hidden_dim,
                output_dim
            ));
        }

        // Counts come from the file: a tensor needs at least its name length
        // and dtype, and a dimension its own u64, so anything claiming more
        // than the file could hold is rejected before it is reserved.
        if tensor_count > bytes.len() as u64 {
            return Err(error::error!(
                "Projection GGUF claims {} tensors, more than '{}' could hold",
                tensor_count,
                path.display()
            ));
        }
        let mut tensor_infos = Vec::with_capacity(tensor_count as usize);
        for _ in 0..tensor_count {
            let name = cursor.read_string()?;
            let dimension_count = cursor.read_u32()? as usize;
            if dimension_count > bytes.len() {
                return Err(error::error!(
                    "Tensor '{}' claims {} dimensions, more than '{}' could hold",
                    name,
                    dimension_count,
                    path.display()
                ));
            }
            let mut dimensions = Vec::with_capacity(dimension_count);
            for _ in 0..dimension_count {
                dimensions.push(cursor.read_u64()?);
            }
            let dtype = cursor.read_u32()?;
            if dtype != GGML_TYPE_F32 {
                return Err(error::error!(
                    "Tensor '{}' has ggml type {}, expected F32",
                    name,
                    dtype
                ));
            }
            let offset = cursor.read_u64()?;
            tensor_infos.push(GgufTensorInfo {
                name,
                dimensions,
                offset,
            });
        }

        let alignment = metadata_integers
            .get("general.alignment")
            .map(|&value| value as u64)
            .filter(|&value| value > 0)
            .unwrap_or(GGUF_DEFAULT_ALIGNMENT);
        let data_start = (cursor.position as u64).div_ceil(alignment) * alignment;

        // Offsets and counts come from the file; checked arithmetic keeps a
        // crafted GGUF from wrapping past the bounds guard.
        let tensor = |name: &str, expected_ne: &[u64]| -> Result<Vec<f32>, error::Error> {
            let info = tensor_infos
                .iter()
                .find(|info| info.name == name)
                .ok_or_else(|| error::error!("Missing tensor '{}' in projection GGUF", name))?;
            if info.dimensions != expected_ne {
                return Err(error::error!(
                    "Tensor '{}' has dimensions {:?}, expected {:?}",
                    name,
                    info.dimensions,
                    expected_ne
                ));
            }
            let count: u64 = info.dimensions.iter().product();
            let out_of_bounds =
                || error::error!("Tensor '{}' extends past the end of the file", name);
            let begin = data_start
                .checked_add(info.offset)
                .and_then(|value| usize::try_from(value).ok())
                .ok_or_else(out_of_bounds)?;
            let end = count
                .checked_mul(4)
                .and_then(|length| usize::try_from(length).ok())
                .and_then(|length| begin.checked_add(length))
                .filter(|&end| end <= bytes.len())
                .ok_or_else(out_of_bounds)?;
            Ok(bytes[begin..end]
                .chunks_exact(4)
                .map(|chunk| f32::from_le_bytes(chunk.try_into().unwrap()))
                .collect())
        };

        let model = Self {
            input_dim,
            hidden_dim,
            output_dim,
            fc1_weight: tensor("fc1.weight", &[input_dim as u64, hidden_dim as u64])?,
            fc1_bias: tensor("fc1.bias", &[hidden_dim as u64])?,
            fc2_weight: tensor("fc2.weight", &[hidden_dim as u64, output_dim as u64])?,
            fc2_bias: tensor("fc2.bias", &[output_dim as u64])?,
        };

        tracing::info!(
            "Loaded projection: {} -> {} -> {}",
            model.input_dim,
            model.hidden_dim,
            model.output_dim
        );

        Ok(model)
    }

    /// Position-wise fc1 -> SiLU -> fc2 over `n_tokens = len / input_dim`
    /// hidden states; returns `n_tokens * output_dim` talker embeddings.
    #[tracing::instrument(skip(self, hidden_states))]
    pub fn forward(&self, hidden_states: &[f32]) -> Result<Vec<f32>, error::Error> {
        if hidden_states.is_empty() || !hidden_states.len().is_multiple_of(self.input_dim) {
            return Err(error::error!(
                "Hidden states length {} is not a multiple of input_dim {}",
                hidden_states.len(),
                self.input_dim
            ));
        }
        let n_tokens = hidden_states.len() / self.input_dim;
        let mut projected = vec![0.0f32; n_tokens * self.output_dim];

        // Positions are independent, and each is two dense matmuls; shard them
        // round-robin across workers as `embd_to_audio` shards its frames.
        let worker_count = std::thread::available_parallelism()
            .map(|count| count.get())
            .unwrap_or(4)
            .min(n_tokens);
        std::thread::scope(|scope| {
            let mut work: Vec<Vec<(usize, &mut [f32])>> =
                (0..worker_count).map(|_| Vec::new()).collect();
            for (index, output) in projected.chunks_mut(self.output_dim).enumerate() {
                work[index % worker_count].push((index, output));
            }
            for chunk in work {
                scope.spawn(move || {
                    let mut hidden = vec![0.0f32; self.hidden_dim];
                    for (index, output) in chunk {
                        let input =
                            &hidden_states[index * self.input_dim..(index + 1) * self.input_dim];
                        for (j, value) in hidden.iter_mut().enumerate() {
                            let row =
                                &self.fc1_weight[j * self.input_dim..(j + 1) * self.input_dim];
                            let sum: f32 = row.iter().zip(input).map(|(w, x)| w * x).sum::<f32>()
                                + self.fc1_bias[j];
                            // SiLU: x * sigmoid(x)
                            *value = sum / (1.0 + (-sum).exp());
                        }
                        for (k, value) in output.iter_mut().enumerate() {
                            let row =
                                &self.fc2_weight[k * self.hidden_dim..(k + 1) * self.hidden_dim];
                            *value = row.iter().zip(&hidden).map(|(w, h)| w * h).sum::<f32>()
                                + self.fc2_bias[k];
                        }
                    }
                });
            }
        });

        Ok(projected)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn write_test_gguf(
        path: &std::path::Path,
        architecture: &str,
        tensors: &[(&str, Vec<u64>, Vec<f32>)],
        input_dim: u32,
        hidden_dim: u32,
        output_dim: u32,
    ) {
        let mut bytes = Vec::new();
        bytes.extend_from_slice(GGUF_MAGIC);
        bytes.extend_from_slice(&GGUF_VERSION.to_le_bytes());
        bytes.extend_from_slice(&(tensors.len() as u64).to_le_bytes());
        bytes.extend_from_slice(&5u64.to_le_bytes());

        let write_string = |bytes: &mut Vec<u8>, value: &str| {
            bytes.extend_from_slice(&(value.len() as u64).to_le_bytes());
            bytes.extend_from_slice(value.as_bytes());
        };

        write_string(&mut bytes, "general.architecture");
        bytes.extend_from_slice(&GGUF_TYPE_STRING.to_le_bytes());
        write_string(&mut bytes, architecture);
        for (key, value) in [
            ("general.alignment", GGUF_DEFAULT_ALIGNMENT as u32),
            ("projection.input_dim", input_dim),
            ("projection.hidden_dim", hidden_dim),
            ("projection.output_dim", output_dim),
        ] {
            write_string(&mut bytes, key);
            bytes.extend_from_slice(&GGUF_TYPE_UINT32.to_le_bytes());
            bytes.extend_from_slice(&value.to_le_bytes());
        }

        let mut offset = 0u64;
        let mut offsets = Vec::new();
        for (_, dimensions, data) in tensors {
            offset = offset.div_ceil(GGUF_DEFAULT_ALIGNMENT) * GGUF_DEFAULT_ALIGNMENT;
            offsets.push(offset);
            assert_eq!(
                dimensions.iter().product::<u64>() as usize,
                data.len(),
                "test tensor shape mismatch"
            );
            offset += (data.len() * 4) as u64;
        }
        for ((name, dimensions, _), tensor_offset) in tensors.iter().zip(&offsets) {
            write_string(&mut bytes, name);
            bytes.extend_from_slice(&(dimensions.len() as u32).to_le_bytes());
            for dimension in dimensions {
                bytes.extend_from_slice(&dimension.to_le_bytes());
            }
            bytes.extend_from_slice(&GGML_TYPE_F32.to_le_bytes());
            bytes.extend_from_slice(&tensor_offset.to_le_bytes());
        }

        while !(bytes.len() as u64).is_multiple_of(GGUF_DEFAULT_ALIGNMENT) {
            bytes.push(0);
        }
        let data_start = bytes.len();
        for ((_, _, data), tensor_offset) in tensors.iter().zip(&offsets) {
            while ((bytes.len() - data_start) as u64) < *tensor_offset {
                bytes.push(0);
            }
            for value in data {
                bytes.extend_from_slice(&value.to_le_bytes());
            }
        }

        std::fs::write(path, bytes).unwrap();
    }

    fn test_model_path(name: &str) -> std::path::PathBuf {
        let mut path = std::env::temp_dir();
        path.push(format!(
            "rust_de_llama-projection-test-{name}-{}.gguf",
            std::process::id()
        ));
        path
    }

    fn identity_like_tensors() -> Vec<(&'static str, Vec<u64>, Vec<f32>)> {
        // input_dim=2, hidden_dim=2, output_dim=3
        Vec::from([
            (
                "fc1.weight",
                Vec::from([2u64, 2]),
                Vec::from([1.0f32, 0.0, 0.0, 1.0]),
            ),
            ("fc1.bias", Vec::from([2u64]), Vec::from([0.0f32, 0.0])),
            (
                "fc2.weight",
                Vec::from([2u64, 3]),
                Vec::from([1.0f32, 0.0, 0.0, 1.0, 1.0, 1.0]),
            ),
            ("fc2.bias", Vec::from([3u64]), Vec::from([0.0f32, 0.5, 0.0])),
        ])
    }

    #[test]
    fn test_load_and_forward() {
        let path = test_model_path("forward");
        write_test_gguf(&path, "projection", &identity_like_tensors(), 2, 2, 3);
        let model = ProjectionModel::load_from_gguf(&path).unwrap();
        std::fs::remove_file(&path).ok();

        assert_eq!(
            (model.input_dim, model.hidden_dim, model.output_dim),
            (2, 2, 3)
        );

        let silu = |x: f32| x / (1.0 + (-x).exp());
        let logits = model.forward(&[1.0, 2.0]).unwrap();
        let (h0, h1) = (silu(1.0), silu(2.0));
        let expected = [h0, h1 + 0.5, h0 + h1];
        for (actual, expected) in logits.iter().zip(expected) {
            assert!((actual - expected).abs() < 1e-6, "{actual} vs {expected}");
        }
    }

    #[test]
    fn test_rejects_zero_dimensions() {
        let path = test_model_path("zero-dims");
        write_test_gguf(&path, "projection", &identity_like_tensors(), 0, 2, 3);
        let result = ProjectionModel::load_from_gguf(&path);
        std::fs::remove_file(&path).ok();
        assert!(result.is_err());
    }

    #[test]
    fn test_rejects_wrong_architecture() {
        let path = test_model_path("arch");
        write_test_gguf(&path, "llama", &identity_like_tensors(), 2, 2, 3);
        let result = ProjectionModel::load_from_gguf(&path);
        std::fs::remove_file(&path).ok();
        assert!(result.is_err());
    }

    #[test]
    fn test_forward_rejects_mismatched_length() {
        let path = test_model_path("length");
        write_test_gguf(&path, "projection", &identity_like_tensors(), 2, 2, 3);
        let model = ProjectionModel::load_from_gguf(&path).unwrap();
        std::fs::remove_file(&path).ok();
        assert!(model.forward(&[1.0, 2.0, 3.0]).is_err());
    }
}
