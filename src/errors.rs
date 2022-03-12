use std::error::Error;
use std::fs;

extern crate flume;
extern crate yaml_rust;

#[derive(Debug)]
pub struct DicoError {
    details: String,
}

impl DicoError {
    pub fn new(msg: String) -> DicoError {
        DicoError{ details: msg }
    }
}

impl std::fmt::Display for DicoError {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        write!(f, "{}", self.details)
    }
}

impl From<std::io::Error> for DicoError {
    fn from(err: std::io::Error) -> Self {
        DicoError::new(err.to_string())
    }
}
impl From<rusqlite::Error> for DicoError {
    fn from(err: rusqlite::Error) -> Self {
        DicoError::new(err.to_string())
    }
}
impl From<yaml_rust::ScanError> for DicoError {
    fn from(err: yaml_rust::ScanError) -> Self {
        DicoError::new(err.to_string())
    }
}
impl From<flume::SendError<fs::DirEntry>> for DicoError {
	fn from(err: flume::SendError<fs::DirEntry>) -> Self {
		DicoError::new(err.to_string())
	}
}

impl Error for DicoError {
    fn description(&self) -> &str {
        &self.details
    }
}
