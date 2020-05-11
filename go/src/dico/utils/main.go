package utils

import (
	"fmt"
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

const (
	wordColor string = "\033[1;34m%s\033[0m"  // bold blue
	typeColor string = "\033[4;37m%s\033[0m"  // underlined light gray
	etymologyColor string = "\033[2;97m%s\033[0m"  // dim white
	definitionsColor string = "\033[0;97m%s\033[0m"  // normal white
	synonymsColor string = "\033[0;37m%s\033[0m"  // normal light gray
)

func (w *Word) Format() (text string) {
	text = fmt.Sprintf("\t" + wordColor, w.Word)
	text += fmt.Sprintf(" " + typeColor + "\n", "(" + w.Type + ")")

	//text += fmt.Sprintf(typeColor + " ", "Type:")
	//text += fmt.Sprintf(typeColor + "\n", w.Type)

	text += fmt.Sprintf(etymologyColor + " ", "Etymology:")
	text += fmt.Sprintf(etymologyColor +"\n", w.Etymology)

	text += fmt.Sprintf(definitionsColor + "\n", "Definitions:")
	for i, def := range w.Definitions {
		text += fmt.Sprintf(" %d. " + definitionsColor + "\n", i+1, def)
	}

	if len(w.Synonyms) > 0 {
		text += fmt.Sprintf(synonymsColor + "\n", "Synonyms:")
		for i, syn := range w.Synonyms {
			text += fmt.Sprintf(" %d. " + synonymsColor +"\n", i+1, syn)
		}
	}

	return text
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
