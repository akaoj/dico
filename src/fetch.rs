use std::io;

use std::fs;
use std::panic;
use std::path::PathBuf;
use std::thread;

extern crate flume;
extern crate parse_wiktionary_en;
extern crate reqwest;

use dico::errors::DicoError;
use dico::Word;

pub fn fetch(path: &PathBuf, language: &str) -> Result<usize, DicoError> {

	Ok(1)
}
