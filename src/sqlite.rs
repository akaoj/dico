use std::path::PathBuf;

use dico::DB;
use dico::errors::DicoError;
use dico::Word;

pub struct SQLite {
	connection: rusqlite::Connection,
}

impl DB for SQLite {
	fn new(path: PathBuf) -> Result<SQLite, DicoError> {
		let connection = rusqlite::Connection::open(path)?;
		// Disable write commits as we don't really care about data integrity (the only place we
		// write in the database is while collecting and if collecting fails, we can just rerun
		// it). Disabling write commits *greatly increases* performance of collection.
		connection.pragma_update(None, "synchronous", "OFF")?;
		Ok(SQLite{ connection: connection })
	}

	fn create_table(&self, language: &str) -> Result<(), DicoError> {
		self.connection.execute(
			&format!(
				"CREATE TABLE IF NOT EXISTS {name}\
				 (word TEXT PRIMARY KEY, kind TEXT NOT NULL, etymology TEXT, \
				 definitions TEXT NOT NULL, synonyms TEXT)", name=language
			),
			rusqlite::params![],
		)?;
		Ok(())
	}

	fn select(&self, word: &str, language: &str) -> Result<Option<Word>, DicoError> {
		let mut stmt = self.connection.prepare(
			&format!("SELECT kind, etymology, definitions, synonyms FROM {} WHERE word = :w", &language)
			)?;

		let mut rows = stmt.query(&[(":w", &word)])?;

		let row = rows.next()?;

		if row.is_none() {
			return Ok(None);
		}

		let row = row.unwrap();

		Ok(Some(Word {
			word: String::from(word),
			language: String::from(language),
			kind: row.get(0).unwrap(),
			etymology: row.get(1).unwrap(),
			definitions: row.get::<usize, String>(2).unwrap().split('\n').map(String::from).collect(),
			synonyms: row.get::<usize, String>(3).unwrap().split('\n').map(String::from).collect(),
		}))
	}

	fn upsert(&self, word: Word) -> Result<(), DicoError> {
		self.connection.execute(
			&format!(
				"INSERT OR REPLACE INTO {}(word, kind, etymology, definitions, synonyms) \
				VALUES(?1, ?2, ?3, ?4, ?5)", &word.language
			),
			rusqlite::params![&word.word, &word.kind, &word.etymology,
			                  &word.definitions.join("\n"), &word.synonyms.join("\n")],
		)?;
		Ok(())
	}
}
