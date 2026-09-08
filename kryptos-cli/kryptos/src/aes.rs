use aes_gcm::{
    aead::{Aead, Generate, Key, KeyInit},
    Aes256Gcm, Nonce
};
use aes_gcm::aead::consts::U12;

use crate::KryptosError;

pub struct AESKeyNounce {
    pub key: Key<Aes256Gcm>,
    pub nonce: Nonce<U12>,
    pub ciphertext: Vec<u8>,
}

impl std::fmt::Debug for AESKeyNounce {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "AESKeyNounce {{ key: {:?}, nonce: {:?}, ciphertext: {:?} }}", self.key, self.nonce, self.ciphertext)
    }
}

pub fn encrypt_string(content: String) -> Result<AESKeyNounce, KryptosError> {
    let key = Key::<Aes256Gcm>::generate();
    let cipher = Aes256Gcm::new(&key);
    let nonce = Nonce::generate();
    let ciphertext = cipher.encrypt(&nonce, content.as_bytes())?;
    return Ok(AESKeyNounce {
        key: key,
        nonce: nonce,
        ciphertext: ciphertext,
    });
}

pub fn decrypt_string(aes_key_nounce: AESKeyNounce) -> Result<String, KryptosError> {
    let cipher = Aes256Gcm::new(&aes_key_nounce.key);
    let plaintext = cipher.decrypt(&aes_key_nounce.nonce, &*aes_key_nounce.ciphertext)?;
    return Ok(String::from_utf8(plaintext)?);
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn round_trip_recovers_original_plaintext() {
        let original = "the quick brown fox".to_string();
        let encrypted = encrypt_string(original.clone()).expect("encryption should succeed");
        let decrypted = decrypt_string(encrypted).expect("decryption should succeed");
        assert_eq!(decrypted, original);
    }

    #[test]
    fn ciphertext_does_not_contain_plaintext() {
        let original = "a very secret message".to_string();
        let encrypted = encrypt_string(original.clone()).expect("encryption should succeed");
        assert_ne!(encrypted.ciphertext, original.into_bytes());
    }

    #[test]
    fn each_encryption_uses_a_fresh_nonce() {
        let a = encrypt_string("same message".to_string()).expect("encryption should succeed");
        let b = encrypt_string("same message".to_string()).expect("encryption should succeed");
        assert_ne!(a.nonce, b.nonce, "nonces must never repeat, even for identical plaintext");
    }

    #[test]
    fn each_encryption_uses_a_fresh_key() {
        let a = encrypt_string("same message".to_string()).expect("encryption should succeed");
        let b = encrypt_string("same message".to_string()).expect("encryption should succeed");
        assert_ne!(a.key, b.key);
    }

    #[test]
    fn decrypt_fails_when_ciphertext_is_tampered_with() {
        let mut encrypted = encrypt_string("tamper test".to_string()).expect("encryption should succeed");
        // Flip a byte in the ciphertext -- GCM's authentication tag should catch this.
        let last = encrypted.ciphertext.len() - 1;
        encrypted.ciphertext[last] ^= 0xFF;
        assert!(decrypt_string(encrypted).is_err());
    }

    #[test]
    fn decrypt_fails_with_wrong_key() {
        let encrypted = encrypt_string("wrong key test".to_string()).expect("encryption should succeed");
        let wrong_key = Key::<Aes256Gcm>::generate();
        let swapped = AESKeyNounce {
            key: wrong_key,
            nonce: encrypted.nonce,
            ciphertext: encrypted.ciphertext,
        };
        assert!(decrypt_string(swapped).is_err());
    }

    #[test]
    fn encrypt_handles_empty_string() {
        let encrypted = encrypt_string(String::new()).expect("encryption should succeed");
        let decrypted = decrypt_string(encrypted).expect("decryption should succeed");
        assert_eq!(decrypted, "");
    }

    #[test]
    fn ciphertext_length_includes_gcm_tag_overhead() {
        let plaintext = "twelve chars";
        let encrypted = encrypt_string(plaintext.to_string()).expect("encryption should succeed");
        // AES-GCM appends a 16-byte authentication tag to the ciphertext.
        assert_eq!(encrypted.ciphertext.len(), plaintext.len() + 16);
    }

    #[test]
    fn encrypting_the_same_message_twice_produces_different_ciphertext() {
        let a = encrypt_string("same message".to_string()).expect("encryption should succeed");
        let b = encrypt_string("same message".to_string()).expect("encryption should succeed");
        assert_ne!(a.ciphertext, b.ciphertext, "fresh key+nonce each call should change the ciphertext bytes");
    }
}
