package fetch

import (
	"dico/utils"
)

// Used for providers plugins
type Fetcher interface {
	Fetch(word string) (*utils.Word, error)
}
