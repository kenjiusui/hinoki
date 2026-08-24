// Package hugo parses Hugo content files (Markdown with YAML/TOML front matter)
// into a normalized Article representation.
package hugo

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

type Article struct {
	// SourcePath is the path to the source .md file, relative to the content dir.
	SourcePath string
	// Slug is the site-relative path the article will be published at, e.g. "/posts/my-post/".
	Slug        string
	Title       string
	Description string
	Tags        []string
	Draft       bool
	PublishedAt time.Time
	UpdatedAt   time.Time
	Body        string
}

type frontMatter struct {
	Title       string    `yaml:"title" toml:"title"`
	Date        time.Time `yaml:"date" toml:"date"`
	LastMod     time.Time `yaml:"lastmod" toml:"lastmod"`
	Draft       bool      `yaml:"draft" toml:"draft"`
	Description string    `yaml:"description" toml:"description"`
	Summary     string    `yaml:"summary" toml:"summary"`
	Slug        string    `yaml:"slug" toml:"slug"`
	Tags        []string  `yaml:"tags" toml:"tags"`
	Categories  []string  `yaml:"categories" toml:"categories"`
}

// Walk finds all content Markdown files under contentDir (excluding Hugo
// section index files, i.e. _index.md) and parses them into Articles.
// excludeDirs lists content-relative directory paths (e.g. "docs",
// "books/cookbook") to skip entirely, along with everything under them.
// excludeFiles lists filename glob patterns (e.g. "about.md", "_*.md"),
// matched against both the file's base name and its content-relative path,
// to skip individual files.
func Walk(contentDir string, excludeDirs, excludeFiles []string) ([]Article, error) {
	excludedDirs := make(map[string]bool, len(excludeDirs))
	for _, d := range excludeDirs {
		excludedDirs[filepath.ToSlash(strings.Trim(d, "/"))] = true
	}

	var articles []Article
	err := filepath.Walk(contentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path == contentDir {
				return nil
			}
			rel, err := filepath.Rel(contentDir, path)
			if err != nil {
				return err
			}
			if excludedDirs[filepath.ToSlash(rel)] {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		base := filepath.Base(path)
		if base == "_index.md" {
			return nil
		}

		rel, err := filepath.Rel(contentDir, path)
		if err != nil {
			return err
		}
		if matchesAny(excludeFiles, base, filepath.ToSlash(rel)) {
			return nil
		}

		art, err := parseFile(path, rel)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		articles = append(articles, art)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return articles, nil
}

func matchesAny(patterns []string, base, rel string) bool {
	for _, p := range patterns {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if ok, _ := filepath.Match(p, base); ok {
			return true
		}
		if ok, _ := filepath.Match(p, rel); ok {
			return true
		}
	}
	return false
}

func parseFile(path, rel string) (Article, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Article{}, err
	}

	fm, body, err := splitFrontMatter(data)
	if err != nil {
		return Article{}, err
	}

	tags := fm.Tags
	if len(tags) == 0 {
		tags = fm.Categories
	}

	desc := fm.Description
	if desc == "" {
		desc = fm.Summary
	}

	title := fm.Title
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	}

	slug := fm.Slug
	if slug == "" {
		slug = slugFromPath(rel)
	} else {
		slug = "/" + strings.Trim(slug, "/") + "/"
	}

	updated := fm.LastMod
	if updated.IsZero() {
		updated = fm.Date
	}

	return Article{
		SourcePath:  rel,
		Slug:        slug,
		Title:       title,
		Description: desc,
		Tags:        tags,
		Draft:       fm.Draft,
		PublishedAt: fm.Date,
		UpdatedAt:   updated,
		Body:        strings.TrimSpace(body),
	}, nil
}

// slugFromPath derives a Hugo-style URL path from a content-relative file path.
// "posts/my-post.md" -> "/posts/my-post/"
// "posts/my-post/index.md" -> "/posts/my-post/"
func slugFromPath(rel string) string {
	rel = filepath.ToSlash(rel)
	rel = strings.TrimSuffix(rel, ".md")
	rel = strings.TrimSuffix(rel, "/index")
	return "/" + strings.Trim(rel, "/") + "/"
}

func splitFrontMatter(data []byte) (frontMatter, string, error) {
	var fm frontMatter

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	if !scanner.Scan() {
		return fm, "", nil
	}
	first := scanner.Text()

	var delim string
	switch strings.TrimSpace(first) {
	case "---":
		delim = "---"
	case "+++":
		delim = "+++"
	default:
		// No front matter; treat entire file as body.
		return fm, string(data), nil
	}

	var fmLines []string
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == delim {
			found = true
			break
		}
		fmLines = append(fmLines, line)
	}
	if !found {
		return fm, string(data), nil
	}

	fmBlock := strings.Join(fmLines, "\n")
	switch delim {
	case "---":
		if err := yaml.Unmarshal([]byte(fmBlock), &fm); err != nil {
			return fm, "", fmt.Errorf("parsing YAML front matter: %w", err)
		}
	case "+++":
		if err := toml.Unmarshal([]byte(fmBlock), &fm); err != nil {
			return fm, "", fmt.Errorf("parsing TOML front matter: %w", err)
		}
	}

	var bodyBuf bytes.Buffer
	for scanner.Scan() {
		bodyBuf.WriteString(scanner.Text())
		bodyBuf.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return fm, "", err
	}

	return fm, bodyBuf.String(), nil
}
