use std::collections::HashMap;
use serde::{Serialize, Deserialize};

use crate::error::KryptosError;

#[derive(Serialize, Deserialize)]
pub struct Key {
    pub key: Vec<u8>,
    pub file_name: String,
}


pub struct Keyring {
    pub ring: HashMap<String, Key>
}

impl Keyring {
    pub fn new() -> Self {
        Keyring {
            ring: HashMap::new()
        }
    }
    
    // TODO: Add a function to add a key to the keyring
    pub fn attach_key(&mut self, key: Key) -> Result<(), KryptosError> {
        if key.key.len() != 32 {
            return Err(KryptosError::ShortKey);
        }
        self.ring.insert(key.file_name.clone(), key);
        Ok(())
    }

    pub fn use_key(&self, key_name: String) -> Option<Key> {
       !unimplemented!()
    }

    pub fn remove_key(&mut self, filename: String) {

    }

    pub fn 
}

// pub fn test() {
//       let mut my_map: HashMap<String, String> = HashMap::new();
//     my_map.insert("key1".to_string(), "value1".to_string());
//     my_map.insert("key2".to_string(), "value2".to_string());
//
//     let serialized = serde_json::to_string(&my_map).unwrap();
//
//     let mut file = File::create("output.json").unwrap();
//     file.write_all(serialized.as_bytes()).unwrap();
// }
