pub mod aes;
pub mod keyring_tool;

use aes::{encrypt_string, decrypt_string};
 //TODO: look at argon2
#[derive(Debug)]
pub enum KryptosError {
    Decrypt(aes_gcm::Error),
    InvalidUtf8(std::string::FromUtf8Error),
    Keyring(keyring::Error),
}

impl From<aes_gcm::Error> for KryptosError {
    fn from(e: aes_gcm::Error) -> Self { KryptosError::Decrypt(e) }
}
impl From<std::string::FromUtf8Error> for KryptosError {
    fn from(e: std::string::FromUtf8Error) -> Self { KryptosError::InvalidUtf8(e) }
}
impl From<keyring::Error> for KryptosError {
    fn from(e: keyring::Error) -> Self { KryptosError::Keyring(e) }
}

pub fn encrypt() {
   match encrypt_string("test".to_string()) {
       Ok(aes_key_nounce) => {
           println!("encrypted: {:?}", aes_key_nounce);
           keyring_tool::set_password(keyring_tool::create_entry("test").unwrap(), "test").unwrap();
       },
       Err(e) => {
           println!("encryption failed: {:?}", e);
       }
   }
}

pub fn decrypt() {}
