package db

import (
	"bytes"
	"context"
	"database/sql"
	"strings"
	"text/template"

	"dico/utils"
)

const sqlCreateTable = `CREATE TABLE IF NOT EXISTS {{.Name}}(word TEXT PRIMARY KEY, type TEXT NOT NULL, etymology TEXT, definitions TEXT NOT NULL, synonyms TEXT)`
const sqlUpsertInto = `INSERT OR REPLACE INTO {{.Name}}(word, type, etymology, definitions, synonyms) VALUES(?, ?, ?, ?, ?)`


func Upsert(ctx context.Context, errChan chan error, wordsChan <-chan utils.Word, db *sql.DB) <-chan bool {
	var err error
	var word utils.Word
	var doneChan chan bool = make(chan bool)

	processWords := func() {
		for word = range wordsChan {
			select {
			case <-ctx.Done():
				return
			default:
				err = upsertInto(db, word.Language, word.Word, word.Type, word.Etymology, word.Definitions, word.Synonyms)
				if err != nil {
					errChan <- err
					return
				}
			}
		}

		doneChan <- false
		close(doneChan)
	}

	go processWords()

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

func upsertInto(db *sql.DB, name string, word string, wType string, etymology string, definitions []string, synonyms []string) error {
	var err error
	var stmt *sql.Stmt
	var tmpl *template.Template
	var parsedTmpl bytes.Buffer

	tmpl, err = template.New("sqlUpsertInto").Parse(sqlUpsertInto)
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

	stmt, err = db.Prepare(parsedTmpl.String())
	if err != nil {
		return err
	}

	_, err = stmt.Exec(word, wType, etymology, strings.Join(definitions, "\n"), strings.Join(synonyms, "\n"))
	if err != nil {
		return err
	}

	return nil
}
