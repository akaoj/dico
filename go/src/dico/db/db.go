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
const sqlSelect = `SELECT word, type, etymology, definitions, synonyms FROM {{.Name}} WHERE word LIKE ?`


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
