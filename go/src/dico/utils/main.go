package utils

import (
	"os"
	"os/user"
)

type Word struct {
	Language string
	Word string `yaml:"word"`
	Type string `yaml:"type"`
	Etymology string `yaml:"etymology"`
	Definitions []string `yaml:"definitions"`
	Synonyms []string `yaml:"synonyms"`
}

var currentUser, _ = user.Current()
var home = currentUser.HomeDir

var DictionaryPaths = []string {
	home + "/.local/share/dico/dico.db",
	"/var/lib/dico/dico.db",
}

// Return the path to the first dictionary found or the empty string if no dictionary is found
func FindDictionaryPath(path string) (pathFound string, err error) {
	var paths []string

	if path != "" {
		paths = append([]string{path}, DictionaryPaths...)
	} else {
		paths = DictionaryPaths
	}

	for _, p := range(paths) {
		// File exists, store the existing path and exit the loop right away
		if _, err := os.Stat(p); err == nil {
			pathFound = p
			break
		}
		// Else check the next one
	}

	return pathFound, nil
}
