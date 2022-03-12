pub mod errors;

use std::fs;
use std::path::PathBuf;

use errors::DicoError;

const SYSTEM_DICO: &str = "/var/lib/dico";
const LOCAL_DICO: &str = ".local/share/dico";
const DICO_FILE: &str = "dico.db";

pub struct Word {
	pub word: String,
	pub language: String,
	pub kind: String,
	pub etymology: String,
	pub definitions: Vec<String>,
	pub synonyms: Vec<String>,
}

impl Word {
	pub fn to_human(&self) -> String {
		let mut msg = format!("\
			\n\
			{word} ({kind})\n\
			\n\
			Etymology: {etymology}", word=&self.word, kind=&self.kind, etymology=&self.etymology
		);

		msg.push_str("\n\nDefinitions:");
		for (i, d) in self.definitions.iter().enumerate() {
			msg.push_str(format!("\n {i}. {d}", i=i+1, d=d).as_ref());
		}

		if ! self.synonyms.is_empty() {
			// FIXME: When no synonyms exist, the vector has one element: the empty string
			if ! self.synonyms[0].is_empty() {
				msg.push_str("\n\nSynonyms:");
				for s in &self.synonyms {
					msg.push_str(format!("\n- {}\n", s).as_ref());
				}
			}
		}

		msg
	}
}

pub trait DB {
	fn new(path: PathBuf) -> Result<Self, DicoError> where Self: Sized;
	fn select(&self, word: &str, language: &str) -> Result<Option<Word>, DicoError>;
	fn upsert(&self, word: Word) -> Result<(), DicoError>;
	fn create_table(&self, language: &str) -> Result<(), DicoError>;
}

/// Find the given dictionary or the first available dictionary on the filesystem if no path is
/// given.
/// Returns Ok(PathBuf) with the path found (if any) or Err() if an error occurs.
/// If we can't find a dictionary, we create one in the local path.
/// If a dictionary is given as argument and can't be found, no other dictionary will be searched
/// and an error will be returned.
pub fn find_dictionary(custom_path: Option<PathBuf>) -> Result<PathBuf, DicoError> {
	// Check if the given dict exists
	if let Some(custom_path) = custom_path {
		match custom_path.exists() {
			true => return Ok(custom_path),
			false => {
				let mut err_msg = String::from(custom_path.to_string_lossy());
				err_msg.push_str(" does not exist");
				return Err(DicoError::new(err_msg))
			}
		}
	}

	// If no dict is given, try to find one locally
	let global_path = PathBuf::from(format!("{}/{}", SYSTEM_DICO, DICO_FILE));
	let local_path = directories::BaseDirs::new().map(|dir|
		dir.home_dir().join(LOCAL_DICO).join(DICO_FILE)
	);

	match local_path.as_ref().map(|p| p.exists()) {
		Some(true) => Ok(local_path.unwrap()),
		_ => match global_path.exists() {
			true => Ok(global_path),
			false => {
				// No dictionary found, create an empty one
				fs::create_dir_all(LOCAL_DICO)?;
				let local_path = local_path.unwrap();
				fs::File::create(&local_path)?;
				Ok(local_path)
			}
		}
	}
}
