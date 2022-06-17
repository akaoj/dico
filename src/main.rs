use std::path::PathBuf;
use std::io;

use clap::Parser;

use dico::DB;
use dico::errors::DicoError;
mod collect;
mod fetch;
mod sqlite;

#[derive(Parser, Debug)]
#[clap(author, version, long_about = "The dictionary, in your terminal.")]
struct Args {
	/// The word you're looking for.
	word: Option<String>,

	/// Collect words at path.
	/// Use this option when you want to generate a database from YAML files.
	#[clap(long)]
	collect: Option<String>,

	/// Path of the dictionary.
	#[clap(short, long)]
	dictionary: Option<String>,

	/// Language to work on. This flag may be provided for either fetching or searching words.
	#[clap(short, long, default_value="en")]
	language: String,

	/// Fetch words given to stdin from authoritative dictionaries online.
	/// It has to be a list of words separated by a newline ("\n").
	/// These words will be sent to the online provider for definition retrieval.
	/// If a word is not found, it will be silently ignored.
	/// Words will be fetched as YAML files to the given path, in a subfolder with its name being
	/// the current language dico is working on. Already existing words will be overwritten.
	#[clap(long)]
	fetch: Option<String>,

	/// Amount of concurrent fetches to the provider. Defaults to 50.
	#[clap(long)]
	fetch_concurrency: Option<u16>,

	/// Run dico as a REST HTTP API, listening to the given host:port.
	#[clap(long)]
	api: Option<String>,
}

fn main() {
	let res = run();
	if res.is_err() {
		eprintln!("Error: {}.", res.err().unwrap());
		std::process::exit(1);
	}
}

fn run() -> Result<(), DicoError> {
	let args = Args::parse();

	if args.language.is_empty() {
		return Err(DicoError::new("you have to provide a language".to_owned()));
	}

	let dictionary = dico::find_dictionary(args.dictionary.map(PathBuf::from))?;

	let db = sqlite::SQLite::new(dictionary)?;

	if let Some(path_str) = args.collect {
		let path = PathBuf::from(path_str);
		println!("Populating database, please wait…");
		let amount = collect::collect(&db, path)?;
		println!(r"Populated {a} words", a=amount);
		return Ok(())
	}
	else if let Some(path_str) = args.fetch {
		let stdin = io::stdin();
		let path = PathBuf::from(path_str);
		let amount = fetch::fetch(stdin, &path, &args.language)?;
		println!(r#"Retrieved {a} words from "{l}" online dictionary"#, a=amount, l=args.language);
		return Ok(())
	}
	else if let Some(address) = args.api {
		let host_port: Vec<&str> = address.split(":").collect();

		if host_port.len() != 2 {
			return Err(DicoError::new(format!(r#"The address {a} is not valid"#, a=&address)))
		}
		let (host, port) = (host_port[0], host_port[1]);

		println!("The API server is listening on {h}:{p}", h=host, p=port);
		api::serve(host, port);
		Ok(());
	}
	else {
		let word = args.word.unwrap_or_else(|| "".to_owned());
		match search(&db, &word, &args.language)? {
			None => return Err(DicoError::new(
				format!(r#"word "{w}" not found in "{l}" language"#, w=&word, l=&args.language)
			)),
			Some(w) => println!("{}", w.to_human()),
		}
	}

	Ok(())
}

fn search(db: &impl DB, word: &str, language: &str) -> Result<Option<dico::Word>, DicoError> {
	if word.is_empty() {
		return Err(DicoError::new("you have to provide a word".to_owned()));
	}
	db.select(word, language)
}
