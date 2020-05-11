package search

import (
	"database/sql"

	dicodb "dico/db"
	"dico/utils"
)


func Search(db *sql.DB, language string, words string) ([]utils.Word, error) {
	return dicodb.Select(db, language, words)
}
