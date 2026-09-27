package sitemap

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

func testConfig() *Config {
	return &Config{
		BaseURL: "https://example.com",
		Locales: []Locale{
			{Code: "en", Prefix: ""},
			{Code: "ru", Prefix: "/ru"},
			{Code: "zh-Hans", Prefix: "https://cn.example.com"},
		},
		DefaultLocale: "en",
		Pages: []Page{
			{Path: "/", Priority: 1, ChangeFreq: "daily"},
			{Path: "/pricing", LastMod: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)},
			{Path: "/blog/ru-only", Locales: []string{"ru"}},
		},
	}
}

func TestBuildExpandsPagesAndLocales(t *testing.T) {
	entries, err := testConfig().Build()
	if err != nil {
		t.Fatal(err)
	}
	// 3 locales × 2 pages + 1 single-locale page
	if got, want := len(entries), 7; got != want {
		t.Fatalf("entries = %d, want %d", got, want)
	}

	var pricingRU *Entry
	for i := range entries {
		if entries[i].Loc == "https://example.com/ru/pricing" {
			pricingRU = &entries[i]
		}
	}
	if pricingRU == nil {
		t.Fatal("ru pricing entry missing")
	}
	want := map[string]string{
		"en":        "https://example.com/pricing",
		"ru":        "https://example.com/ru/pricing",
		"zh-Hans":   "https://cn.example.com/pricing",
		"x-default": "https://example.com/pricing",
	}
	for k, v := range want {
		if pricingRU.Alternates[k] != v {
			t.Errorf("alternate %s = %q, want %q", k, pricingRU.Alternates[k], v)
		}
	}
}

func TestRootURLs(t *testing.T) {
	entries, _ := testConfig().Build()
	locs := map[string]bool{}
	for _, e := range entries {
		locs[e.Loc] = true
	}
	for _, l := range []string{"https://example.com/", "https://example.com/ru", "https://cn.example.com/"} {
		if !locs[l] {
			t.Errorf("missing root url %s (have %v)", l, locs)
		}
	}
}

func TestTrailingSlash(t *testing.T) {
	c := testConfig()
	c.TrailingSlash = true
	entries, _ := c.Build()
	for _, e := range entries {
		if !strings.HasSuffix(e.Loc, "/") {
			t.Errorf("%s should end with /", e.Loc)
		}
	}
}

func TestSingleLocalePageHasNoHreflang(t *testing.T) {
	entries, _ := testConfig().Build()
	for _, e := range entries {
		if strings.HasSuffix(e.Loc, "/ru-only") && e.Alternates != nil {
			t.Errorf("single-locale page should not have alternates: %v", e.Alternates)
		}
	}
}

func TestValidateCollectsAllErrors(t *testing.T) {
	c := &Config{
		BaseURL:       "example.com",
		Locales:       []Locale{{Code: "en"}, {Code: "EN"}, {Code: "not a code"}},
		DefaultLocale: "de",
		Pages:         []Page{{Path: "pricing", Priority: 2, ChangeFreq: "sometimes", Locales: []string{"fr"}}},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, frag := range []string{"absolute URL", "duplicate", "valid hreflang", "defaultLocale", "must start with /", "changefreq", "priority", "unknown locale"} {
		if !strings.Contains(err.Error(), frag) {
			t.Errorf("error should mention %q:\n%v", frag, err)
		}
	}
}

func TestWriteURLSetIsValidXML(t *testing.T) {
	entries, _ := testConfig().Build()
	var buf bytes.Buffer
	if err := WriteURLSet(&buf, entries); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, frag := range []string{
		`xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"`,
		`xmlns:xhtml="http://www.w3.org/1999/xhtml"`,
		`<xhtml:link rel="alternate" hreflang="x-default" href="https://example.com/pricing"></xhtml:link>`,
		`<lastmod>2026-09-01</lastmod>`,
		`<priority>1.0</priority>`,
	} {
		if !strings.Contains(out, frag) {
			t.Errorf("output missing %s\n%s", frag, out)
		}
	}
	var probe struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(buf.Bytes(), &probe); err != nil {
		t.Fatalf("invalid xml: %v", err)
	}
	if len(probe.URLs) != len(entries) {
		t.Errorf("parsed %d urls, want %d", len(probe.URLs), len(entries))
	}
}

func TestChunk(t *testing.T) {
	e := make([]Entry, 10)
	if got := len(Chunk(e, 3)); got != 4 {
		t.Errorf("chunks = %d, want 4", got)
	}
	if got := len(Chunk(e, 0)); got != 1 {
		t.Errorf("default chunk size should fit 10 entries in 1 file, got %d", got)
	}
	if got := len(Chunk(nil, 3)); got != 0 {
		t.Errorf("empty input should produce 0 chunks, got %d", got)
	}
}

func TestWriteIndex(t *testing.T) {
	var buf bytes.Buffer
	_ = WriteIndex(&buf, []string{"https://example.com/sitemap-1.xml"}, time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(buf.String(), "<sitemapindex") || !strings.Contains(buf.String(), "<lastmod>2026-09-27</lastmod>") {
		t.Errorf("unexpected index:\n%s", buf.String())
	}
}
