package main

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"dico/utils"

	"golang.org/x/net/html"
)

var etymologyExtractRegexp *regexp.Regexp = regexp.MustCompile(`^\((.*?)\)$`)
// Space and non-break space
const spaces = "\u0020\u00A0"


// Exported for plugin system
var Fetcher fetcher


type fetcher struct{}

func (f fetcher) Fetch(rawWord string) (*utils.Word, error) {
	var err error
	var word utils.Word

	var resp *http.Response

	// This provider will return a 301 then a 200 for a found word and a 200 for an unknown word.
	// Sometimes, under "high" load, it will return a 500 (which means: "we can retry in a bit").
	// 503 and 504 will also be considered as retryable should they occur.
	// All other status codes will be considered an error.
	for i := 0; resp == nil && i < 10; i++ {
		// By default, the http library will follow redirections so we won't see the 301 but only the 200.
		// We don't need to know if we found the word or not because in both cases, the page will be parsed
		// and if no data is found, we assume the word was not found.
		resp, err = http.Get("https://www.larousse.fr/dictionnaires/francais/" + rawWord)
		if err != nil {
			return nil, err
		}

		switch resp.StatusCode {
		case 200:
			// resp is not nil, we will exit the loop
		case 500, 503, 504:
			resp = nil
			// Don't wait on the first run (i=0) as it will most probably succeed on the second try
			time.Sleep(time.Duration(i) * time.Second)
			continue
		default:
			// Any other code: failure to retrieve the word
			return nil, errors.New("Provider returned " + resp.Status + " for word: " + rawWord)
		}
	}

	if resp == nil {
		// Can't fetch the word: return no word
		return nil, nil
	}

	var body *html.Node
	body, err = html.Parse(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	var crawler func(*html.Node)
	var wType string
	var definitions []string = make([]string, 0, 5)  // we suppose most words won't have more than 5 definitions
	var etymology string

	// This function will parse a node and extract all text from all subnodes
	var extractWordsOnlyFromNode func(*html.Node) string
	extractWordsOnlyFromNode = func(node *html.Node) string {
		var ret string = ""
		if node.Type == html.ElementNode {
			// <p class="RubriqueDefinition"> Definiton "header" / "category": put it in parentheses before the definition
			if node.Data == "p" {
				for _, attr := range node.Attr {
					if attr.Key == "class" && attr.Val == "RubriqueDefinition" {
						ret += "(" + node.FirstChild.Data + ") "
						return ret
					}
				}
			}
			// <a class="lienconj"> Link to conjugation (we don't want this)
			if node.Data == "a" {
				for _, attr := range node.Attr {
					if attr.Key == "class" && attr.Val == "lienconj" {
						return ""
					}
				}
			}
		} else if node.Type == html.TextNode {
			ret += node.Data
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			ret += extractWordsOnlyFromNode(child)
		}
		return ret
	}

	crawler = func(node *html.Node) {
		if node.Type == html.ElementNode {
			// <li class="DivisionDefinition"> Definitions for the current word
			if node.Data == "li" {
				for _, attr := range node.Attr {
					if attr.Key == "class" && attr.Val == "DivisionDefinition" {
						definitions = append(definitions, strings.Trim(extractWordsOnlyFromNode(node), spaces))
					}
					return
				}
			// <p class="CatgramDefinition"> Type of the word
			// <p class="OrigineDefinition"> Etymology of the word (we need to remove the parenthesis though)
			} else if node.Data == "p" {
				for _, attr := range node.Attr {
					if attr.Key == "class" && attr.Val == "CatgramDefinition" {
						wType = strings.Trim(extractWordsOnlyFromNode(node), spaces)
						return
					}
					if attr.Key == "class" && attr.Val == "OrigineDefinition" {
						var etymologyRaw string = extractWordsOnlyFromNode(node)
						// Remove leading and trailing parentheses
						etymology = strings.Trim(etymologyExtractRegexp.FindStringSubmatch(etymologyRaw)[1], spaces)
						return
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			crawler(child)
		}
	}
	crawler(body)

	// Skip words without definition (this also includes unknown words)
	if len(definitions) == 0 {
		return nil, nil
	}

	// Create the utils.Word from it
	word = utils.Word {
		Language: "fr",
		Word: rawWord,
		Type: wType,
		Etymology: etymology,
		Definitions: definitions,
		Synonyms: make([]string, 0),  // FIXME: no synonyms for now with this provider
	}

	return &word, err
}
