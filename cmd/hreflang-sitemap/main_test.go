package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWritesSingleSitemap(t *testing.T) {
	dir := t.TempDir()
	if err := run("../../examples/site.json", dir, false); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "sitemap.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "<urlset") {
		t.Error("expected a urlset")
	}
}

func TestRunSplitsIntoIndex(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "big.json")
	var pages []string
	for i := 0; i < 5; i++ {
		pages = append(pages, fmt.Sprintf(`{"path":"/p%d"}`, i))
	}
	os.WriteFile(cfg, []byte(`{"baseURL":"https://x.io","maxURLsPerFile":4,
		"locales":[{"code":"en"},{"code":"ru","prefix":"/ru"}],
		"pages":[`+strings.Join(pages, ",")+`]}`), 0o644)

	if err := run(cfg, dir, false); err != nil {
		t.Fatal(err)
	}
	idx, _ := os.ReadFile(filepath.Join(dir, "sitemap.xml"))
	if !strings.Contains(string(idx), "<sitemapindex") || !strings.Contains(string(idx), "https://x.io/sitemap-3.xml") {
		t.Errorf("expected index of 3 files:\n%s", idx)
	}
}

func TestRunRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "bad.json")
	os.WriteFile(cfg, []byte(`{"baseURL":"https://x.io","locales":[{"code":"en"}],"pagez":[]}`), 0o644)
	if err := run(cfg, dir, true); err == nil {
		t.Error("expected unknown-field error")
	}
}
