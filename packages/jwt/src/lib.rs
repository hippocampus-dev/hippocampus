//! <https://datatracker.ietf.org/doc/html/rfc7519>

#[cfg(feature = "derive")]
pub use jwt_derive::*;
use rsa::PublicKey;
use sha2::Digest;

#[derive(Debug)]
enum Error {
    InvalidAlgorithm,
    InvalidSignature(rsa::errors::Error),
}

impl std::error::Error for Error {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            Error::InvalidAlgorithm => None,
            Error::InvalidSignature(e) => Some(e),
        }
    }
}
impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Error::InvalidAlgorithm => write!(f, "invalid algorithm"),
            Error::InvalidSignature(e) => write!(f, "invalid signature: {e}"),
        }
    }
}

pub fn verify_with_rsa<S>(public_key: &rsa::RsaPublicKey, jwt: S) -> Result<(), error::Error>
where
    S: AsRef<str>,
{
    let jwt_parts = jwt.as_ref().split('.').collect::<Vec<&str>>();
    let (encoded_header, input, signature) =
        (jwt_parts[0], jwt_parts[0..2].join("."), jwt_parts[2]);
    let header = Header::decode(encoded_header)?;

    let verification = match header.alg {
        Algorithm::RS256 => {
            let mut hasher = sha2::Sha256::default();
            hasher.update(input.as_bytes());
            public_key.verify(
                rsa::PaddingScheme::new_pkcs1v15_sign::<sha2::Sha256>(),
                &hasher.finalize()[..],
                &base64::decode_config(signature, base64::URL_SAFE)?,
            )
        }
        Algorithm::RS384 => {
            let mut hasher = sha2::Sha384::default();
            hasher.update(input.as_bytes());
            public_key.verify(
                rsa::PaddingScheme::new_pkcs1v15_sign::<sha2::Sha384>(),
                &hasher.finalize()[..],
                &base64::decode_config(signature, base64::URL_SAFE)?,
            )
        }
        Algorithm::RS512 => {
            let mut hasher = sha2::Sha512::default();
            hasher.update(input.as_bytes());
            public_key.verify(
                rsa::PaddingScheme::new_pkcs1v15_sign::<sha2::Sha512>(),
                &hasher.finalize()[..],
                &base64::decode_config(signature, base64::URL_SAFE)?,
            )
        }
        _ => return Err(Error::InvalidAlgorithm.into()),
    };
    verification.map_err(|e| Error::InvalidSignature(e).into())
}

pub fn sign_with_rsa<T>(
    private_key: &rsa::RsaPrivateKey,
    header: &Header,
    claims: &T,
) -> Result<String, error::Error>
where
    T: Encode + serde::Serialize,
{
    let encoded_header = header.encode()?;
    let encoded_claims = claims.encode()?;
    let input = format!("{encoded_header}.{encoded_claims}");

    let signature = match header.alg {
        Algorithm::RS256 => {
            let mut hasher = sha2::Sha256::default();
            hasher.update(input.as_bytes());
            private_key.sign(
                rsa::PaddingScheme::new_pkcs1v15_sign::<sha2::Sha256>(),
                &hasher.finalize()[..],
            )?
        }
        Algorithm::RS384 => {
            let mut hasher = sha2::Sha384::default();
            hasher.update(input.as_bytes());
            private_key.sign(
                rsa::PaddingScheme::new_pkcs1v15_sign::<sha2::Sha384>(),
                &hasher.finalize()[..],
            )?
        }
        Algorithm::RS512 => {
            let mut hasher = sha2::Sha512::default();
            hasher.update(input.as_bytes());
            private_key.sign(
                rsa::PaddingScheme::new_pkcs1v15_sign::<sha2::Sha512>(),
                &hasher.finalize()[..],
            )?
        }
        _ => return Err(Error::InvalidAlgorithm.into()),
    };
    Ok(format!(
        "{}.{}",
        input,
        base64::encode_config(signature, base64::URL_SAFE)
    ))
}

pub trait Encode {
    fn encode(&self) -> Result<String, error::Error>
    where
        Self: serde::Serialize,
    {
        let bytes = serde_json::to_vec(self)?;
        Ok(base64::encode_config(bytes, base64::URL_SAFE))
    }
}

/// <https://datatracker.ietf.org/doc/html/rfc7519#section-4>
#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct Claims {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub iss: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sub: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub aud: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub exp: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub nbf: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub iat: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub jti: Option<String>,
}

impl Encode for Claims {}

/// <https://datatracker.ietf.org/doc/html/rfc7518>
#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub enum Algorithm {
    HS256,
    HS384,
    HS512,
    ES256,
    ES384,
    RS256,
    RS384,
    RS512,
    PS256,
    PS384,
    PS512,
    #[serde(rename = "none")]
    None,
}

/// <https://datatracker.ietf.org/doc/html/rfc7515>
#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct Header {
    pub alg: Algorithm,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub jku: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub jwk: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub kid: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub x5u: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub x5c: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub x5t: Option<String>,
    #[serde(rename = "x5t#S256", skip_serializing_if = "Option::is_none")]
    pub x5t_s256: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub typ: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cty: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub crit: Option<String>,
}

impl Header {
    fn decode<T>(encoded: T) -> Result<Self, error::Error>
    where
        T: AsRef<[u8]>,
    {
        let bytes = base64::decode_config(encoded, base64::URL_SAFE)?;
        Ok(serde_json::from_slice(&bytes)?)
    }
}

impl Encode for Header {}
