package main

import (
	"net/http"
	"regexp"

	"dico/utils"

	"golang.org/x/net/html"
)

var etymologyExtractRegexp *regexp.Regexp = regexp.MustCompile(`^\((.*?)\)$`)

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

	var extractWordsOnlyFromNode func(*html.Node) string
	extractWordsOnlyFromNode = func(node *html.Node) string {
		var ret string = ""
		if node.Type == html.TextNode {
			ret += node.Data
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			ret += extractWordsOnlyFromNode(child)
		}
		return ret
	}

	crawler = func(node *html.Node) {
		//var etymology string

		if node.Type == html.ElementNode {
			if node.Data == "li" {
				for _, attr := range node.Attr {
					if attr.Key == "class" && attr.Val == "DivisionDefinition" {
						definitions = append(definitions, extractWordsOnlyFromNode(node))
					}
				}
			} else if node.Data == "p" {
				for _, attr := range node.Attr {
					if attr.Key == "class" && attr.Val == "CatgramDefinition" {
						wType = extractWordsOnlyFromNode(node)
					}
					if attr.Key == "class" && attr.Val == "OrigineDefinition" {
						var etymologyRaw string = extractWordsOnlyFromNode(node)
						// Remove leading and trailing parentheses
						etymology = etymologyExtractRegexp.FindStringSubmatch(etymologyRaw)[1]
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
