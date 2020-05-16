package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dico/cli"
	"dico/collect"
	dicodb "dico/db"
	"dico/search"
	"dico/utils"

	"github.com/pborman/getopt/v2"

	_ "github.com/mattn/go-sqlite3"
)

var VERSION string = "dev"

func main() {
	var err error
	var helpOpt *bool = getopt.BoolLong("help", 'h', "Show this help.")
	var versionOpt *bool = getopt.BoolLong("version", 'v', "Show dico version.")
	var collectOpt *string = getopt.StringLong("collect", 0, "", "Collect words at path.\n" +
	                                                             "Use this option when you want to generate a database from YAML files.\n" +
	                                                             "Note that the language flag does not apply here given that all languages found in the collect folder will be collected.", "path")
	var dictPathOpt *string = getopt.StringLong("dictionary", 'd', "", "Path of the dictionary.", "path")
	var languageOpt *string = getopt.StringLong("language", 'l', "", "Language to work on.\n" +
	                                                                 "This flag may be provided for fetching and searching words.", "language")
	var fetchOpt *bool = getopt.BoolLong("fetch", 0, "Fetch words given to stdin from authoritative dictionaries online.\n" +
	                                                 "It has to be a list of words separated by a newline (\"\\n\").\n" +
	                                                 "These words will be sent to the online provider for definition retrieval.\n" +
	                                                 "If a word is not found, it will be silently ignored.\n" +
	                                                 "For every word found, a corresponding YAML file will be created in the path defined by --fetch-to.\n" +
	                                                 "Note that you also must set the --fetch-to flag with this option." )
	var fetchToOpt *string = getopt.StringLong("fetch-to", 0, "", "Fetch words to the given path.\n" +
	                                                              "Already existing words will be overwritten.", "path")
	var fetchConcurrency *int = getopt.IntLong("fetch-concurrency", 0, 50, "Amount of concurrent fetches to the provider.")
	getopt.Parse()

	const usageExamples string = `
Examples:

To retrieve the definitions for a list of French words and store them in the data/ folder:
  echo -e "baguette\noui\nmerci" | dico -l fr --fetch --fetch-to=words/ --fetch-concurrency=3
or
  cat wordsList.txt | dico -l fr --fetch --fetch-to=words/ --fetch-concurrency=200

To collect the definitions available in data/ and put them in the database
(database location defaults at ~/.local/share/dico/dico.db):
  dico --collect=words/

To search for a definition for a French word (default language, when --language is not provided, is "en"):
  dico -l fr baguette

Note: search functionality requires a database.`

	if *languageOpt == "" {
		// Default to English
		*languageOpt = "en"
	}

	var ctx context.Context
	var ctxCancel context.CancelFunc

	ctx, ctxCancel = context.WithCancel(context.Background())
	defer ctxCancel()

	// Non-standard workflow
	switch {
	case *helpOpt:
		getopt.PrintUsage(os.Stderr)
		fmt.Fprintln(os.Stderr, usageExamples)
		os.Exit(0)
	case *versionOpt:
		fmt.Fprintln(os.Stderr, "dico version " + VERSION)
		os.Exit(0)
	case *fetchOpt:
		if *fetchToOpt == "" {
			fmt.Fprintln(os.Stderr, "You must provide the --fetch-to flag")
			os.Exit(1)
		}
		fmt.Println("Fetching words; this may take a very long time depending on the amount of words to fetch")
		var amount uint32
		amount, err = cli.FetchWords(ctx, os.Stdin, *languageOpt, *fetchToOpt, *fetchConcurrency)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: " + err.Error())
			os.Exit(1)
		}
		fmt.Print(amount)  // uint32, can't strconv.Itoa easily
		fmt.Println(" words fetched, stored in folder " + *fetchToOpt)
		os.Exit(0)
	}

	// We will try to find a valid dictionary
	var dictPath string

	dictPath, err = utils.FindDictionaryPath(*dictPathOpt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: " + err.Error())
		os.Exit(1)
	}

	// We always return an error when no dictionary is found except when we're collecting words (because
	// we'll create a new one if needed)
	if dictPath == "" {
		if *collectOpt == ""{
			fmt.Fprintln(os.Stderr, errors.New("Error: Can't find the dictionary in any of " + strings.Join(utils.DictionaryPaths, ", ")))
			os.Exit(1)
		} else {
			// If we're collecting words and no dictionary is found, we create it
			dictPath = *dictPathOpt
			if dictPath == "" {
				dictPath = utils.DictionaryPaths[0]
			}
			fmt.Println("No database found, creating a new one at " + dictPath)

			err = os.MkdirAll(filepath.Dir(dictPath), 0755)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			var newFile *os.File
			newFile, err = os.Create(dictPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			err = newFile.Chmod(0644)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
	}

	// Dictionary is available, we can prepare a connection to it (all subsequent steps will need a DB connection)
	var db *sql.DB

	// Because we don't care about safe commits (we only write when collecting and if the collect fails, we simply run
	// it again), we disable synchronous writes so we can save LOTS of time
	db, err = sql.Open("sqlite3", dictPath + "?_synchronous=0")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: " + err.Error())
		os.Exit(1)
	}
	defer db.Close()

	// Special case: collect words from folder
	if *collectOpt != "" {
		fmt.Println("Collecting words in " + *collectOpt + ", populating database at " + dictPath)
		var errChan chan error = make(chan error, 1)
		defer close(errChan)

		var wordsChan <-chan utils.Word
		var doneChan <-chan bool

		wordsChan = collect.Collect(ctx, errChan, db, *collectOpt)

		doneChan = dicodb.Upsert(ctx, errChan, wordsChan, db)

		select {
		case err = <- errChan:
			fmt.Fprintln(os.Stderr, "Error: " + err.Error())
			ctxCancel()
			os.Exit(1)
		case <-doneChan:
			fmt.Println("Collection done")
			os.Exit(0)
		}
	}

	// Standard workflow: we need to retrieve the string the user search and process it
	var searchQuery string = strings.Join(getopt.Args(), " ")

	var words []utils.Word

	words, err = search.Search(db, *languageOpt, searchQuery)

	for _, v := range words {
		fmt.Println(v.Format())
	}
}
