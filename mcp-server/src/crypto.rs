use aes_gcm::{Aes256Gcm, Key, Nonce};
use aes_gcm::aead::{Aead, KeyInit};
use base64::{Engine, engine::general_purpose::STANDARD};
use crate::error::{Result, ValtError};

/// Decrypt AES-256-GCM ciphertext.
/// Expected format: base64(nonce[12] || ciphertext+tag)
#[allow(dead_code)]
pub fn decrypt_aes_gcm(encoded: &str, key_bytes: &[u8]) -> Result<Vec<u8>> {
    let data = STANDARD.decode(encoded)
        .map_err(|e| ValtError::Crypto(format!("base64 decode: {e}")))?;
    if data.len() < 12 {
        return Err(ValtError::Crypto("ciphertext too short".into()));
    }
    let (nonce_bytes, ciphertext) = data.split_at(12);
    let key = Key::<Aes256Gcm>::from_slice(key_bytes);
    let cipher = Aes256Gcm::new(key);
    let nonce = Nonce::from_slice(nonce_bytes);
    cipher.decrypt(nonce, ciphertext)
        .map_err(|_| ValtError::Crypto("decryption failed".into()))
}

#[cfg(test)]
mod tests {
    use super::*;
    use aes_gcm::aead::OsRng;
    use aes_gcm::{Aes256Gcm, KeyInit};
    use aes_gcm::aead::{Aead, AeadCore};
    use base64::{Engine, engine::general_purpose::STANDARD};

    #[test]
    fn test_decrypt_roundtrip() {
        let key = Aes256Gcm::generate_key(OsRng);
        let cipher = Aes256Gcm::new(&key);
        let nonce = Aes256Gcm::generate_nonce(OsRng);
        let plaintext = b"hello valtag";
        let ciphertext = cipher.encrypt(&nonce, plaintext.as_ref()).unwrap();
        let mut combined = nonce.to_vec();
        combined.extend_from_slice(&ciphertext);
        let encoded = STANDARD.encode(&combined);
        let decrypted = decrypt_aes_gcm(&encoded, &key).unwrap();
        assert_eq!(decrypted, plaintext);
    }

    #[test]
    fn test_decrypt_wrong_key_fails() {
        let key = Aes256Gcm::generate_key(OsRng);
        let cipher = Aes256Gcm::new(&key);
        let nonce = Aes256Gcm::generate_nonce(OsRng);
        let ciphertext = cipher.encrypt(&nonce, b"secret".as_ref()).unwrap();
        let mut combined = nonce.to_vec();
        combined.extend_from_slice(&ciphertext);
        let encoded = STANDARD.encode(&combined);
        // Use wrong key
        let wrong_key = Aes256Gcm::generate_key(OsRng);
        assert!(decrypt_aes_gcm(&encoded, &wrong_key).is_err());
    }
}
