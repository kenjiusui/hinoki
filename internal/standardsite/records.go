// Package standardsite defines Go representations of the site.standard.*
// AT Protocol lexicon records (https://standard.site/docs/lexicons/).
package standardsite

const (
	CollectionPublication = "site.standard.publication"
	CollectionDocument    = "site.standard.document"
)

type Publication struct {
	Type        string `json:"$type"`
	URL         string `json:"url"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type Document struct {
	Type        string   `json:"$type"`
	Site        string   `json:"site"`
	Title       string   `json:"title"`
	PublishedAt string   `json:"publishedAt"`
	Path        string   `json:"path,omitempty"`
	Description string   `json:"description,omitempty"`
	TextContent string   `json:"textContent,omitempty"`
	Content     any      `json:"content,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	UpdatedAt   string   `json:"updatedAt,omitempty"`
}

// MarkdownContent is the at.markpub.markdown lexicon, one of the variants
// usable in Document.Content to carry richly-formatted body text (see
// https://markpub.at/). It wraps a MarkdownText using the plain-string form.
type MarkdownContent struct {
	Type string       `json:"$type"`
	Text MarkdownText `json:"text"`
}

type MarkdownText struct {
	Type     string `json:"$type"`
	Markdown string `json:"markdown"`
}

// NewMarkdownContent builds an at.markpub.markdown content value carrying
// the given Markdown body inline (as opposed to the blob or facets forms).
func NewMarkdownContent(markdown string) MarkdownContent {
	return MarkdownContent{
		Type: "at.markpub.markdown",
		Text: MarkdownText{
			Type:     "at.markpub.text",
			Markdown: markdown,
		},
	}
}
