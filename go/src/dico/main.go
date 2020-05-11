package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dico/collect"
	dicodb "dico/db"
	"dico/search"
	"dico/utils"

	"github.com/pborman/getopt/v2"

	_ "github.com/mattn/go-sqlite3"
)

var VERSION string = "dev"

func main() {
	var helpOpt *bool = getopt.BoolLong("help", 'h', "", "show this help")
	var versionOpt *bool = getopt.BoolLong("version", 'v', "", "show dico version")
	var collectOpt *string = getopt.StringLong("collect", 'c', "", "collect words at <path>", "path")
	var dictPathOpt *string = getopt.StringLong("dictionary", 'd', "", "path of the dictionary", "path")
	var languageOpt *string = getopt.StringLong("language", 'l', "", "language")
	getopt.Parse()

	// Non-standard workflow
	switch {
	case *helpOpt:
		getopt.PrintUsage(os.Stderr)
		os.Exit(0)
	case *versionOpt:
		fmt.Fprintln(os.Stderr, "dico version " + VERSION)
		os.Exit(0)
	}

	// We will try to find a valid dictionary
	var dictPath string
	var err error

	dictPath, err = utils.FindDictionaryPath(*dictPathOpt)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// We always return an error when no dictionary is found except when we're collecting words (because
	// we'll create a new one if needed)
	if dictPath == "" {
		if *collectOpt == ""{
			fmt.Fprintln(os.Stderr, errors.New("Can't find the dictionary in any of " + strings.Join(utils.DictionaryPaths, ", ")))
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
	var ctx context.Context
	var ctxCancel context.CancelFunc

	db, err = sql.Open("sqlite3", dictPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, ctxCancel = context.WithCancel(context.Background())
	defer ctxCancel()

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
			fmt.Fprintln(os.Stderr, err.Error())
			ctxCancel()
			os.Exit(1)
		case <-doneChan:
			fmt.Println("Collection done")
			os.Exit(0)
		}
	}

	// Standard workflow: we need to retrieve the string the user search and process it
	var searchQuery string = strings.Join(getopt.Args(), " ")

	if *languageOpt == "" {
		// Default to English
		*languageOpt = "en"
	}

	var words []utils.Word

	words, err = search.Search(db, *languageOpt, searchQuery)

	for _, v := range words {
		fmt.Println(v.Format())
	}
}
