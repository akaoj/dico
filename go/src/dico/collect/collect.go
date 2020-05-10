package collect

import (
	"context"
	"database/sql"
	"errors"
	"io/ioutil"
	"os"
	"sync"

	dicodb "dico/db"
	"dico/utils"

	"gopkg.in/yaml.v2"
)

// Collect will browse the given path and launch one goroutine per language. Each goroutine will parse all words
// (YAML files) inside the language folder and send utils.Word in the channel Collect will return.
// You have to provide an errChan channel to know if a goroutine failed.
func Collect(ctx context.Context, errChan chan error, db *sql.DB, path string) <-chan utils.Word {
	// Retrieve folders (= languages)
	var content []os.FileInfo
	var language os.FileInfo
	var wg sync.WaitGroup

	var err error
	var wordsChan chan utils.Word = make(chan utils.Word, 100)

	content, err = ioutil.ReadDir(path)
	if err != nil {
		errChan <- err
		return wordsChan
	}

	// How to process a lanuage folder
	processLang := func(language string) {
		var languagePath string = path + "/" + language

		var words []os.FileInfo
		var word os.FileInfo

		// Retrieve all words
		words, err = ioutil.ReadDir(languagePath)
		if err != nil {
			errChan <- err
			return
		}

		// Process words
		for _, word = range words {
			var fileYaml []byte
			var file utils.Word

			fileYaml, err = ioutil.ReadFile(languagePath + "/" + word.Name())
			if err != nil {
				errChan <- err
				return
			}

			err = yaml.Unmarshal(fileYaml, &file)
			if err != nil {
				errChan <- errors.New("Error while processing YAML file " + word.Name() + ": " + err.Error())
				return
			}

			// Add the language to the word data (not present in YAML file)
			file.Language = language

			// And send the word in the pipe for processing in the database
			select {
			case <-ctx.Done():
				return
			case wordsChan<- file:
			}
		}

		wg.Done()
	}

	// Run a goroutine per language
	for _, language = range content {
		// Before inserting data in the DB, make sure the table exists for this language
		err = dicodb.CreateTable(db, language.Name())
		if err != nil {
			errChan <- err
			return wordsChan
		}

		wg.Add(1)
		go processLang(language.Name())
	}

	// Make sure we close the words channel when all files are processed
	go func() {
		wg.Wait()
		close(wordsChan)
	}()

	return wordsChan
}
