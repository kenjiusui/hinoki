package hugo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalk(t *testing.T) {
	dir := t.TempDir()
	posts := filepath.Join(dir, "posts")
	if err := os.MkdirAll(posts, 0755); err != nil {
		t.Fatal(err)
	}

	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(posts, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	write("hello.md", `---
title: "Hello World"
date: 2026-08-01T10:00:00+09:00
tags: ["go", "atproto"]
description: "My first post"
draft: false
---

This is **the body** of my first post.

Second paragraph.
`)

	write("toml-post.md", `+++
title = "TOML Post"
date = 2026-08-02T00:00:00Z
+++
Body here.
`)

	if err := os.WriteFile(filepath.Join(dir, "_index.md"), []byte(`---
title: "Home"
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	arts, err := Walk(dir, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 articles (section index excluded), got %d", len(arts))
	}

	var hello *Article
	for i := range arts {
		if arts[i].Title == "Hello World" {
			hello = &arts[i]
		}
	}
	if hello == nil {
		t.Fatal("did not find Hello World article")
	}
	if hello.Slug != "/posts/hello/" {
		t.Errorf("slug = %q, want /posts/hello/", hello.Slug)
	}
	if hello.Description != "My first post" {
		t.Errorf("description = %q", hello.Description)
	}
	if len(hello.Tags) != 2 || hello.Tags[0] != "go" {
		t.Errorf("tags = %v", hello.Tags)
	}
	if hello.PublishedAt.Year() != 2026 {
		t.Errorf("publishedAt = %v", hello.PublishedAt)
	}
	if hello.Body == "" {
		t.Errorf("body should not be empty")
	}
}

func TestWalkExcludeDirs(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"posts", "docs", "docs/nested"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			t.Fatal(err)
		}
	}

	write := func(rel, content string) {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("posts/kept.md", "---\ntitle: Kept\n---\nbody\n")
	write("docs/excluded.md", "---\ntitle: Excluded\n---\nbody\n")
	write("docs/nested/also-excluded.md", "---\ntitle: Also Excluded\n---\nbody\n")

	arts, err := Walk(dir, []string{"docs"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("expected 1 article after excluding docs/, got %d: %+v", len(arts), arts)
	}
	if arts[0].Title != "Kept" {
		t.Errorf("title = %q, want Kept", arts[0].Title)
	}
	if arts[0].SourcePath != filepath.Join("posts", "kept.md") {
		t.Errorf("sourcePath = %q", arts[0].SourcePath)
	}
}

func TestWalkExcludeFiles(t *testing.T) {
	dir := t.TempDir()

	write := func(rel, content string) {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("hello.md", "---\ntitle: Hello\n---\nbody\n")
	write("about.md", "---\ntitle: About\n---\nbody\n")
	write("privacy.md", "---\ntitle: Privacy\n---\nbody\n")

	arts, err := Walk(dir, nil, []string{"about.md", "priv*.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("expected 1 article after excluding about.md and priv*.md, got %d: %+v", len(arts), arts)
	}
	if arts[0].Title != "Hello" {
		t.Errorf("title = %q, want Hello", arts[0].Title)
	}
}
