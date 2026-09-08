use keyring::Entry;
use crate::KryptosError;

pub fn create_entry(key_id: &str) -> Result<Entry, KryptosError> {
    let entry = Entry::new("kryptos", key_id);
    Ok(entry?)
}

pub fn set_password(entry: Entry, password: &str) -> Result<(), KryptosError> {
    entry.set_password(password)?;
    Ok(())
}

pub fn get_password(entry: Entry) -> Result<String, KryptosError> {
    let password = entry.get_password()?;
    Ok(password)
}

pub fn delete_entry(entry: Entry) -> Result<(), KryptosError> {
    entry.set_password("")?;
    Ok(())
}
