package main

import (
	"net/http"
	"regexp"
	"strings"

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

	resp, err = http.Get("https://www.larousse.fr/dictionnaires/francais/" + rawWord)
	if err != nil {
		return nil, err
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
