pub mod aes;
pub mod keyring_tool;
pub mod error;

use aes::{encrypt_string, decrypt_string};
 //TODO: look at argon2


// TODO: Make this a exposable function through "C" nomangle and returns the nounce
pub fn encrypt() {
   match encrypt_string("test".to_string()) {
       Ok(aes_key_nounce) => {
           println!("encrypted: {:?}", aes_key_nounce);
       },
       Err(e) => {
           println!("encryption failed: {:?}", e);
       }
   }
}

pub fn decrypt() {}
