use std::io::Write;
#[cfg(target_os = "linux")]
use std::os::unix::fs::{DirBuilderExt, OpenOptionsExt};

const DIRECTORY: &str = "teleput";

#[cfg(target_os = "linux")]
pub fn base_directory(variable: &str, fallback: &str) -> Result<std::path::PathBuf, error::Error> {
    // Relative values are invalid and must be ignored: https://specifications.freedesktop.org/basedir-spec/latest/
    if let Some(value) =
        std::env::var_os(variable).filter(|value| std::path::Path::new(value).is_absolute())
    {
        return Ok(std::path::PathBuf::from(value).join(DIRECTORY));
    }
    let home = std::env::var_os("HOME").ok_or_else(|| error::error!("HOME is not set"))?;
    Ok(std::path::PathBuf::from(home)
        .join(fallback)
        .join(DIRECTORY))
}

#[cfg(target_os = "linux")]
fn configuration_directory() -> Result<std::path::PathBuf, error::Error> {
    base_directory("XDG_CONFIG_HOME", ".config")
}

#[cfg(target_os = "windows")]
fn configuration_directory() -> Result<std::path::PathBuf, error::Error> {
    let application_data =
        std::env::var_os("APPDATA").ok_or_else(|| error::error!("APPDATA is not set"))?;
    Ok(std::path::PathBuf::from(application_data).join(DIRECTORY))
}

pub fn write_private(path: &std::path::Path, contents: &[u8]) -> Result<(), error::Error> {
    let directory = path
        .parent()
        .ok_or_else(|| error::error!("{} has no parent directory", path.display()))?;
    let mut directory_builder = std::fs::DirBuilder::new();
    directory_builder.recursive(true);
    #[cfg(target_os = "linux")]
    directory_builder.mode(0o700);
    directory_builder.create(directory)?;
    let temporary = path.with_extension("tmp");
    let mut options = std::fs::OpenOptions::new();
    options.write(true).create(true).truncate(true);
    #[cfg(target_os = "linux")]
    options.mode(0o600);
    let mut file = options.open(&temporary)?;
    file.write_all(contents)?;
    file.sync_all()?;
    std::fs::rename(&temporary, path)?;
    Ok(())
}

pub struct Identity {
    pub certificate: rustls::pki_types::CertificateDer<'static>,
    pub key: rustls::pki_types::PrivateKeyDer<'static>,
}

pub fn load_or_generate() -> Result<Identity, error::Error> {
    let directory = configuration_directory()?;
    let certificate_path = directory.join("certificate.der");
    let key_path = directory.join("key.der");
    if !certificate_path.is_file() || !key_path.is_file() {
        let certified =
            rcgen::generate_simple_self_signed(vec![crate::protocol::SERVER_NAME.to_string()])?;
        write_private(&key_path, &certified.key_pair.serialize_der())?;
        write_private(&certificate_path, certified.cert.der())?;
    }
    Ok(Identity {
        certificate: rustls::pki_types::CertificateDer::from(std::fs::read(&certificate_path)?),
        key: rustls::pki_types::PrivateKeyDer::Pkcs8(rustls::pki_types::PrivatePkcs8KeyDer::from(
            std::fs::read(&key_path)?,
        )),
    })
}

pub fn fingerprint(certificate: &rustls::pki_types::CertificateDer<'_>) -> String {
    use sha2::Digest;
    sha2::Sha256::digest(certificate.as_ref())
        .iter()
        .map(|byte| format!("{byte:02X}"))
        .collect::<Vec<_>>()
        .join(":")
}

fn pin_path(name: &str) -> Result<std::path::PathBuf, error::Error> {
    Ok(configuration_directory()?.join(name))
}

pub fn load_pin(name: &str) -> Result<Option<String>, error::Error> {
    match std::fs::read_to_string(pin_path(name)?) {
        Ok(pin) => Ok(Some(pin.trim().to_string())),
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(None),
        Err(e) => Err(e.into()),
    }
}

pub fn confirm_pairing(
    peer: &rustls::pki_types::CertificateDer<'_>,
    name: &str,
) -> Result<String, error::Error> {
    println!("peer:         {}", fingerprint(peer));
    print!("Does \"peer\" match \"this machine\" printed on the other side? [y/N] ");
    std::io::stdout().flush()?;
    let mut answer = String::new();
    std::io::stdin().read_line(&mut answer)?;
    if answer.trim() != "y" {
        error::bail!("pairing rejected");
    }
    let pin = fingerprint(peer);
    write_private(&pin_path(name)?, pin.as_bytes())?;
    println!("paired");
    Ok(pin)
}

pub fn provider() -> std::sync::Arc<rustls::crypto::CryptoProvider> {
    std::sync::Arc::new(rustls::crypto::ring::default_provider())
}

#[derive(Debug)]
pub struct PeerVerifier {
    pins: std::sync::Mutex<Vec<Option<String>>>,
    algorithms: rustls::crypto::WebPkiSupportedAlgorithms,
    presented: std::sync::Mutex<Option<rustls::pki_types::CertificateDer<'static>>>,
}

impl PeerVerifier {
    pub fn new(pins: Vec<Option<String>>) -> Self {
        PeerVerifier {
            pins: std::sync::Mutex::new(pins),
            algorithms: provider().signature_verification_algorithms,
            presented: std::sync::Mutex::new(None),
        }
    }

    pub fn presented(&self) -> Option<rustls::pki_types::CertificateDer<'static>> {
        self.presented.lock().unwrap().clone()
    }

    pub fn matched(&self) -> Option<usize> {
        let presented = fingerprint(self.presented.lock().unwrap().as_ref()?);
        self.pins
            .lock()
            .unwrap()
            .iter()
            .position(|pin| pin.as_deref() == Some(presented.as_str()))
    }

    #[cfg(target_os = "linux")]
    pub fn unpaired(&self) -> Vec<usize> {
        self.pins
            .lock()
            .unwrap()
            .iter()
            .enumerate()
            .filter(|(_, pin)| pin.is_none())
            .map(|(index, _)| index)
            .collect()
    }

    #[cfg(target_os = "linux")]
    pub fn set_pin(&self, index: usize, pin: String) {
        self.pins.lock().unwrap()[index] = Some(pin);
    }

    fn verify(
        &self,
        end_entity: &rustls::pki_types::CertificateDer<'_>,
    ) -> Result<(), rustls::Error> {
        *self.presented.lock().unwrap() = Some(end_entity.clone().into_owned());
        let presented = fingerprint(end_entity);
        if self
            .pins
            .lock()
            .unwrap()
            .iter()
            .any(|pin| pin.as_ref().is_none_or(|pin| *pin == presented))
        {
            Ok(())
        } else {
            Err(rustls::Error::General(format!(
                "peer certificate {presented} is not the paired one"
            )))
        }
    }
}

impl rustls::client::danger::ServerCertVerifier for PeerVerifier {
    fn verify_server_cert(
        &self,
        end_entity: &rustls::pki_types::CertificateDer<'_>,
        _intermediates: &[rustls::pki_types::CertificateDer<'_>],
        _server_name: &rustls::pki_types::ServerName<'_>,
        _ocsp_response: &[u8],
        _now: rustls::pki_types::UnixTime,
    ) -> Result<rustls::client::danger::ServerCertVerified, rustls::Error> {
        self.verify(end_entity)?;
        Ok(rustls::client::danger::ServerCertVerified::assertion())
    }

    fn verify_tls12_signature(
        &self,
        message: &[u8],
        certificate: &rustls::pki_types::CertificateDer<'_>,
        signature: &rustls::DigitallySignedStruct,
    ) -> Result<rustls::client::danger::HandshakeSignatureValid, rustls::Error> {
        rustls::crypto::verify_tls12_signature(message, certificate, signature, &self.algorithms)
    }

    fn verify_tls13_signature(
        &self,
        message: &[u8],
        certificate: &rustls::pki_types::CertificateDer<'_>,
        signature: &rustls::DigitallySignedStruct,
    ) -> Result<rustls::client::danger::HandshakeSignatureValid, rustls::Error> {
        rustls::crypto::verify_tls13_signature(message, certificate, signature, &self.algorithms)
    }

    fn supported_verify_schemes(&self) -> Vec<rustls::SignatureScheme> {
        self.algorithms.supported_schemes()
    }
}

impl rustls::server::danger::ClientCertVerifier for PeerVerifier {
    fn root_hint_subjects(&self) -> &[rustls::DistinguishedName] {
        &[]
    }

    fn verify_client_cert(
        &self,
        end_entity: &rustls::pki_types::CertificateDer<'_>,
        _intermediates: &[rustls::pki_types::CertificateDer<'_>],
        _now: rustls::pki_types::UnixTime,
    ) -> Result<rustls::server::danger::ClientCertVerified, rustls::Error> {
        self.verify(end_entity)?;
        Ok(rustls::server::danger::ClientCertVerified::assertion())
    }

    fn verify_tls12_signature(
        &self,
        message: &[u8],
        certificate: &rustls::pki_types::CertificateDer<'_>,
        signature: &rustls::DigitallySignedStruct,
    ) -> Result<rustls::client::danger::HandshakeSignatureValid, rustls::Error> {
        rustls::crypto::verify_tls12_signature(message, certificate, signature, &self.algorithms)
    }

    fn verify_tls13_signature(
        &self,
        message: &[u8],
        certificate: &rustls::pki_types::CertificateDer<'_>,
        signature: &rustls::DigitallySignedStruct,
    ) -> Result<rustls::client::danger::HandshakeSignatureValid, rustls::Error> {
        rustls::crypto::verify_tls13_signature(message, certificate, signature, &self.algorithms)
    }

    fn supported_verify_schemes(&self) -> Vec<rustls::SignatureScheme> {
        self.algorithms.supported_schemes()
    }
}
