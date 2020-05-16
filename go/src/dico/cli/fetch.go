package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"plugin"
	"sync"
	"sync/atomic"

	"dico/fetch"
	"dico/utils"

	"gopkg.in/yaml.v3"
)



func FetchWords(ctx context.Context, stream io.Reader, language string, to string, concurrencyLimit int) (uint32, error) {
	var err error
	var scanner *bufio.Scanner = bufio.NewScanner(stream)

	var rawWordsChan chan string = make(chan string, 100)
	var wordsChan chan utils.Word = make(chan utils.Word, 100)
	var errChan chan error = make(chan error)
	var doneChan chan bool = make(chan bool)

	// Load the language provider
	var binDir string
	binDir, err = filepath.Abs(filepath.Dir(os.Args[0]))
	var pluginPath string = binDir + "/providers/" + language + ".so"

	if _, err = os.Stat(pluginPath); err != nil {
		return 0, errors.New("The provider for language " + language + " does not exist")
	}

	var plug *plugin.Plugin
	plug, err = plugin.Open(pluginPath)
	if err != nil {
		return 0, err
	}

	var fetchSymbol plugin.Symbol
	fetchSymbol, err = plug.Lookup("Fetcher")
	if err != nil {
		return 0, err
	}

	var fetcher fetch.Fetcher
	var ok bool
	fetcher, ok = fetchSymbol.(fetch.Fetcher)
	if !ok {
		return 0, errors.New("Unexpected type for Fetcher module symbol")
	}

	var subCtxCancel context.CancelFunc
	var subCtx context.Context
	subCtx, subCtxCancel = context.WithCancel(ctx)
	defer subCtxCancel()

	// Read words from source
	go func() {
		defer close(rawWordsChan)
		for scanner.Scan() {
			select {
			case <-subCtx.Done():
				return
			case rawWordsChan<- scanner.Text():
			}
		}
	}()

	// Run an army of goroutines to fetch all words
	var wg sync.WaitGroup

	for i := 1; i <= concurrencyLimit; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var rawWord string
			var word *utils.Word
			var err error

			// Fetch words on the online dictionary
			for rawWord = range rawWordsChan {
				word, err = fetcher.Fetch(rawWord)
				if err != nil {
					errChan<- err
					return
				}

				// Word not found
				if word == nil {
					fmt.Println("Skipping word " + rawWord + " (provider can't retrieve it)")
					continue
				}

				select {
				case <-subCtx.Done():
					return
				case wordsChan<- *word:
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(wordsChan)
	}()

	var amount uint32 = 0

	// Generate YAML files from all the fetched data
	go func() {
		// Create the output dir if needed
		err = os.MkdirAll(to + "/" + language, 0755)
		if err != nil {
			errChan<- err
			return
		}

		var word utils.Word
		for word = range wordsChan {
			select {
			case <-subCtx.Done():
				return
			default:
				var fileYaml []byte
				var filePath string = to + "/" + language + "/" + word.Word + ".yml"

				fileYaml, err = yaml.Marshal(word)
				if err != nil {
					errChan<- err
					return
				}

				// Create the file (truncate it if it already exists - that's OK: we want to fetch new data for a word)
				var file *os.File
				file, err = os.Create(filePath)
				if err != nil {
					errChan<- err
					return
				}

				err = file.Chmod(0644)
				if err != nil {
					errChan<- err
					return
				}

				_, err = file.Write(fileYaml)
				if err != nil {
					errChan<- err
					return
				}
				file.Close()

				atomic.AddUint32(&amount, 1)
			}
		}
		doneChan<- true
		close(doneChan)
	}()

	// Wait for goroutines to finish
	select {
	case err = <-errChan:
		subCtxCancel()
		return 0, err
	case <-doneChan:
	}

	return amount, nil
}
