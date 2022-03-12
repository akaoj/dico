use std::fs;
use std::panic;
use std::path::PathBuf;
use std::thread;

extern crate yaml_rust;
extern crate flume;

use dico::DB;
use dico::errors::DicoError;
use dico::Word;

pub fn collect(db: &impl DB, path: PathBuf) -> Result<usize, DicoError> {
	if ! path.is_dir() {
		return Err(DicoError::new(format!("the path {} is not a directory", path.to_string_lossy())));
	}

	// We will crawl the given folder for files. Every file will be pushed in a channel and threads
	// (the amount depends on the amount of available CPUs) will read these files, parse them as
	// YAML, build Words from them and push them in a second channel. A last thread will collect
	// all the Words and upsert them in the database.
	let (reader_tx, reader_rx) = flume::bounded(1000);
	let (word_tx, word_rx) = flume::bounded(1000);

	// Initialize tables (one table per language for smaller indices, therefore faster searches)
	for language in fs::read_dir(&path)? {
		let language = language?;
		db.create_table(language.file_name().as_os_str().to_string_lossy().as_ref())?;
	}

	let crawler = thread::spawn(move || -> Result<(), DicoError> {
		for language in fs::read_dir(path)? {
			let language = language?;
			for word in fs::read_dir(language.path())? {
				let word = word?;
				reader_tx.send(word)?;
			}
		}
		Ok(())
	});

	let amount_threads: usize = thread::available_parallelism()?.get();
	let generated_threads = amount_threads * 2;  // it's faster with more threads
	let mut threads = Vec::with_capacity(generated_threads);

	for _ in 0..generated_threads {
		let reader_rx = reader_rx.clone();
		let word_tx = word_tx.clone();

		let reader = thread::spawn(move || -> Result<(), DicoError> {
			for file in reader_rx {
				let content = fs::read_to_string(file.path())?;
				let yaml = yaml_rust::YamlLoader::load_from_str(&content)?;
				let doc = &yaml[0];

				let w_word = String::from(match doc["word"].as_str() {
					Some(w) => w,
					None => continue,
				});
				let w_language = String::from(match doc["language"].as_str() {
					Some(l) => l,
					None => continue,
				});
				let w_kind = String::from(match doc["kind"].as_str() {
					Some(k) => k,
					None => doc["type"].as_str().unwrap_or(""),
				});
				let w_etymology = String::from(doc["etymology"].as_str().unwrap_or(""));
				let w_definitions = doc["definitions"].as_vec().unwrap_or(&Vec::new()).iter().map(
					|d| String::from(d.as_str().unwrap_or(""))
				).collect();
				let w_synonyms = doc["synonyms"].as_vec().unwrap_or(&Vec::new()).iter().map(
					|s| String::from(s.as_str().unwrap_or(""))
				).collect();

				let word = Word {
					word: w_word,
					language: w_language,
					kind: w_kind,
					etymology: w_etymology,
					definitions: w_definitions,
					synonyms: w_synonyms,
				};
				word_tx.send(word).unwrap();
			}
			Ok(())
		});
		threads.push(reader);
	}
	drop(word_tx);
	let mut i: usize = 0;
	for word in word_rx {
		db.upsert(word)?;
		i += 1;
	}

	for reader in threads {
		match reader.join() {
			Err(e) => panic::resume_unwind(e),  // thread error
			Ok(Err(e)) => return Err(DicoError::new(e.to_string())),  // code error
			Ok(Ok(_)) => {},
		}
	}
	match crawler.join() {
		Err(e) => panic::resume_unwind(e),  // thread error
		Ok(Err(e)) => return Err(DicoError::new(e.to_string())),  // code error
		Ok(Ok(i)) => i,
	};

	Ok(i)
}
