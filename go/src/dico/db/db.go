package db

import (
	"bytes"
	"context"
	"database/sql"
	"strings"
	"sync"
	"text/template"

	"dico/utils"
)

const sqlCreateTable = `CREATE TABLE IF NOT EXISTS {{.Name}}(word TEXT PRIMARY KEY, type TEXT NOT NULL, etymology TEXT, definitions TEXT NOT NULL, synonyms TEXT)`
const sqlUpsertInto = `INSERT OR REPLACE INTO {{.Name}}(word, type, etymology, definitions, synonyms) VALUES(?, ?, ?, ?, ?)`
const sqlSelect = `SELECT word, type, etymology, definitions, synonyms FROM {{.Name}} WHERE word LIKE ?`


func Upsert(ctx context.Context, errChan chan error, wordsChan <-chan utils.Word, db *sql.DB) <-chan bool {
	var err error
	var word utils.Word
	var doneChan chan bool = make(chan bool)
	var wg sync.WaitGroup

	var stmt *sql.Stmt
	var tmpl *template.Template
	var parsedTmpl bytes.Buffer

	// Prepare the query before so we don't have to parse for every word
	// Because all languages are in different tables, we have to save prepared statements for all languages so we don't
	// lose time recreating it every time
	var preparedStatements map[string]*sql.Stmt = make(map[string]*sql.Stmt)
	defer func() {
		for _, stmt = range preparedStatements {
			stmt.Close()
		}
	}()

	processWords := func() {
		for word = range wordsChan {
			select {
			case <-ctx.Done():
				return
			default:
				var prepStmt *sql.Stmt
				var ok bool

				prepStmt, ok = preparedStatements[word.Language]
				if ! ok {
					tmpl, err = template.New("sqlUpsertInto").Parse(sqlUpsertInto)
					if err != nil {
						errChan<- err
						return
					}

					data := struct{
						Name string
					}{
						Name: word.Language,
					}

					err = tmpl.Execute(&parsedTmpl, data)
					if err != nil {
						errChan<- err
						return
					}

					stmt, err = db.Prepare(parsedTmpl.String())
					if err != nil {
						errChan<- err
						return
					}

					prepStmt = stmt
					preparedStatements[word.Language] = stmt
				}

				_, err = prepStmt.Exec(word.Word, word.Type, word.Etymology, strings.Join(word.Definitions, "\n"), strings.Join(word.Synonyms, "\n"))
				if err != nil {
					errChan<- err
					return
				}
			}
		}
		wg.Done()
	}

	wg.Add(1)
	go processWords()

	go func() {
		wg.Wait()
		doneChan<- true
		close(doneChan)
	}()

	return doneChan
}

func CreateTable(db *sql.DB, name string) error {
	var err error

	var tmpl *template.Template
	var parsedTmpl bytes.Buffer

	tmpl, err = template.New("sqlCreateTable").Parse(sqlCreateTable)
	if err != nil {
		return err
	}

	data := struct{
		Name string
	}{
		Name: name,
	}

	err = tmpl.Execute(&parsedTmpl, data)
	if err != nil {
		return err
	}

	_, err = db.Exec(parsedTmpl.String())
	if err != nil {
		return err
	}

	return nil
}

func Select(db *sql.DB, tableName string, word string) (words []utils.Word, err error) {
	var stmt *sql.Stmt
	var tmpl *template.Template
	var rows *sql.Rows
	var parsedTmpl bytes.Buffer

	tmpl, err = template.New("sqlSelect").Parse(sqlSelect)
	if err != nil {
		return words, err
	}

	data := struct{
		Name string
	}{
		Name: tableName,
	}

	err = tmpl.Execute(&parsedTmpl, data)
	if err != nil {
		return words, err
	}

	stmt, err = db.Prepare(parsedTmpl.String())
	if err != nil {
		return words, err
	}

	rows, err = stmt.Query(word)
	if err != nil {
		return words, err
	}
	defer rows.Close()

	for rows.Next() {
		var word, wType, etymology, definitions, synonyms string

		err = rows.Scan(&word, &wType, &etymology, &definitions, &synonyms)
		if err != nil {
			return words, err
		}

		var definitionsList []string = strings.Split(definitions, "\n")
		if definitions == "" {
			definitionsList = make([]string, 0)
		}

		var synonymsList []string = strings.Split(synonyms, "\n")
		if synonyms == "" {
			synonymsList = make([]string, 0)
		}

		words = append(words, utils.Word{
			Language: tableName,
			Word: word,
			Type: wType,
			Etymology: etymology,
			Definitions: definitionsList,
			Synonyms: synonymsList,
		})
	}

	return words, nil
}
