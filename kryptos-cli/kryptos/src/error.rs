#[derive(Debug)]
pub enum KryptosError {
    Decrypt(aes_gcm::Error),
    InvalidUtf8(std::string::FromUtf8Error),
    ShortKey,
    KeyNotFound,
}

impl From<aes_gcm::Error> for KryptosError {
    fn from(e: aes_gcm::Error) -> Self { KryptosError::Decrypt(e) }
}

impl From<std::string::FromUtf8Error> for KryptosError {
    fn from(e: std::string::FromUtf8Error) -> Self { KryptosError::InvalidUtf8(e) }
}
