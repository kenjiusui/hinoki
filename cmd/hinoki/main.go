// Command hinoki converts Hugo articles into site.standard.* AT Protocol
// records and syncs them to a Personal Data Server (PDS).
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/harutsugegusa/hinoki/internal/atproto"
	"github.com/harutsugegusa/hinoki/internal/config"
	"github.com/harutsugegusa/hinoki/internal/hugo"
	"github.com/harutsugegusa/hinoki/internal/standardsite"
	"github.com/harutsugegusa/hinoki/internal/state"
)

const (
	configPath = "hinoki.yaml"
	statePath  = ".hinoki-state.json"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "init":
		err = cmdInit()
	case "sync":
		force := false
		for _, arg := range os.Args[2:] {
			if arg == "--force" || arg == "-f" {
				force = true
			}
		}
		err = cmdSync(force)
	case "-h", "--help", "help":
		usage()
		return
	default:
		usage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`hinoki - publish Hugo articles as site.standard.site records

Usage:
  hinoki init          Set up hinoki.yaml in the current directory
  hinoki sync          Publish new/changed articles to your PDS
  hinoki sync --force  Republish every article regardless of whether it changed
                        (use after upgrading hinoki, if it now maps front
                        matter to the PDS records differently)`)
}

func cmdInit() error {
	reader := bufio.NewReader(os.Stdin)
	prompt := func(label, def string) string {
		if def != "" {
			fmt.Printf("%s [%s]: ", label, def)
		} else {
			fmt.Printf("%s: ", label)
		}
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			return def
		}
		return line
	}

	handle := prompt("Bluesky handle (e.g. alice.bsky.social)", "")
	pds := prompt("PDS URL", "https://bsky.social")
	contentDir := prompt("Hugo content directory", "content")
	siteURL := prompt("Published site base URL (e.g. https://example.com)", "")
	siteName := prompt("Site name", "")
	siteDescription := prompt("Site description (optional)", "")
	excludeDirsRaw := prompt("Directories under content/ to exclude (comma-separated, e.g. docs,books), optional", "")

	excludeFilesRaw := prompt("Filenames to exclude (comma-separated glob patterns, e.g. about.md,privacy.md), optional", "")

	splitList := func(raw string) []string {
		var out []string
		for _, d := range strings.Split(raw, ",") {
			if d = strings.TrimSpace(d); d != "" {
				out = append(out, d)
			}
		}
		return out
	}
	excludeDirs := splitList(excludeDirsRaw)
	excludeFiles := splitList(excludeFilesRaw)

	cfg := &config.Config{
		Handle:          handle,
		PDS:             pds,
		ContentDir:      contentDir,
		SiteURL:         siteURL,
		SiteName:        siteName,
		SiteDescription: siteDescription,
		ExcludeDirs:     excludeDirs,
		ExcludeFiles:    excludeFiles,
	}
	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("saving %s: %w", configPath, err)
	}
	fmt.Printf(`Wrote %s (no password stored in it).

Before running "hinoki sync", provide your app password (from Bluesky
settings > App passwords — not your regular login password) either via:

  export HINOKI_APP_PASSWORD=xxxx-xxxx-xxxx-xxxx

or you will be prompted for it interactively at sync time.
`, configPath)
	return nil
}

func readAppPassword() (string, error) {
	if pw := os.Getenv("HINOKI_APP_PASSWORD"); pw != "" {
		return pw, nil
	}
	fmt.Print("App password (from Bluesky settings > App passwords): ")
	pwBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("reading app password: %w", err)
	}
	return strings.TrimSpace(string(pwBytes)), nil
}

func cmdSync(force bool) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading %s (run `hinoki init` first): %w", configPath, err)
	}

	st, err := state.Load(statePath)
	if err != nil {
		return fmt.Errorf("loading %s: %w", statePath, err)
	}

	appPassword, err := readAppPassword()
	if err != nil {
		return err
	}

	client := atproto.NewClient(cfg.PDS)
	if err := client.CreateSession(cfg.Handle, appPassword); err != nil {
		return err
	}

	if err := ensurePublication(client, cfg); err != nil {
		return err
	}

	articles, err := hugo.Walk(cfg.ContentDir, cfg.ExcludeDirs, cfg.ExcludeFiles)
	if err != nil {
		return fmt.Errorf("reading Hugo content: %w", err)
	}

	siteRef := fmt.Sprintf("at://%s/%s/%s", client.DID, standardsite.CollectionPublication, cfg.PublicationRkey)

	seen := make(map[string]bool, len(articles))

	var created, updated, skipped, draftsSkipped int
	for _, a := range articles {
		seen[a.SourcePath] = true

		if a.Draft && !cfg.IncludeDrafts {
			draftsSkipped++
			continue
		}

		doc := standardsite.Document{
			Type:        standardsite.CollectionDocument,
			Site:        siteRef,
			Title:       a.Title,
			Path:        a.Slug,
			Description: a.Description,
			TextContent: a.Body,
			Content:     standardsite.NewMarkdownContent(a.Body),
			Tags:        a.Tags,
			PublishedAt: formatTime(a.PublishedAt),
			UpdatedAt:   formatTimeOmitEmpty(a.UpdatedAt),
		}

		hash := hashDocument(doc)
		entry, exists := st.Documents[a.SourcePath]
		if exists && entry.Hash == hash && !force {
			skipped++
			continue
		}

		rkey := entry.Rkey
		if rkey == "" {
			rkey = atproto.NewTID()
		}

		if _, _, err := client.PutRecord(standardsite.CollectionDocument, rkey, doc); err != nil {
			return fmt.Errorf("publishing %s: %w", a.SourcePath, err)
		}

		st.Documents[a.SourcePath] = state.Entry{Rkey: rkey, Hash: hash}
		if err := st.Save(statePath); err != nil {
			return fmt.Errorf("saving %s: %w", statePath, err)
		}

		if exists {
			updated++
			fmt.Printf("updated  %s\n", a.Slug)
		} else {
			created++
			fmt.Printf("created  %s\n", a.Slug)
		}
	}

	var deleted int
	for sourcePath, entry := range st.Documents {
		if seen[sourcePath] {
			continue
		}
		if err := client.DeleteRecord(standardsite.CollectionDocument, entry.Rkey); err != nil {
			return fmt.Errorf("deleting record for removed article %s: %w", sourcePath, err)
		}
		delete(st.Documents, sourcePath)
		if err := st.Save(statePath); err != nil {
			return fmt.Errorf("saving %s: %w", statePath, err)
		}
		deleted++
		fmt.Printf("deleted  %s (source removed)\n", sourcePath)
	}

	fmt.Printf("\nDone. %d created, %d updated, %d deleted, %d unchanged, %d drafts skipped.\n", created, updated, deleted, skipped, draftsSkipped)
	return nil
}

func ensurePublication(client *atproto.Client, cfg *config.Config) error {
	pub := standardsite.Publication{
		Type:        standardsite.CollectionPublication,
		URL:         cfg.SiteURL,
		Name:        cfg.SiteName,
		Description: cfg.SiteDescription,
	}

	if cfg.PublicationRkey != "" {
		var existing standardsite.Publication
		ok, err := client.GetRecord(standardsite.CollectionPublication, cfg.PublicationRkey, &existing)
		if err != nil {
			return fmt.Errorf("checking existing publication record: %w", err)
		}
		if ok {
			if existing != pub {
				if _, _, err := client.PutRecord(standardsite.CollectionPublication, cfg.PublicationRkey, pub); err != nil {
					return fmt.Errorf("updating publication record: %w", err)
				}
			}
			return nil
		}
		// rkey configured but missing on the PDS; fall through and recreate.
	}

	rkey := atproto.NewTID()
	if _, _, err := client.PutRecord(standardsite.CollectionPublication, rkey, pub); err != nil {
		return fmt.Errorf("creating publication record: %w", err)
	}
	cfg.PublicationRkey = rkey
	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("saving publication rkey to %s: %w", configPath, err)
	}
	fmt.Printf("created publication record (rkey=%s)\n", rkey)
	return nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.UTC().Format(time.RFC3339)
}

func formatTimeOmitEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func hashDocument(d standardsite.Document) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00%v\x00%s",
		d.Title, d.Path, d.Description, d.TextContent, d.PublishedAt, d.Tags, d.UpdatedAt)
	return hex.EncodeToString(h.Sum(nil))
}
