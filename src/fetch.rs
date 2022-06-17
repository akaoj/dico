use std::fs;
use std::io;
use std::panic;
use std::path::PathBuf;
use std::thread;

extern crate flume;
extern crate reqwest;

use dico::errors::DicoError;
use dico::Word;

pub fn fetch(stdin: io::Stdin, path: &PathBuf, language: &str) -> Result<usize, DicoError> {
	Ok(1)
}
