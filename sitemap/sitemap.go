// Package sitemap builds multilingual XML sitemaps with hreflang alternates,
// following Google's guidelines for localized versions of a page:
// every URL lists all of its language alternates (including itself) and an
// optional x-default, and large sets are split into a sitemap index.
package sitemap

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// MaxURLsPerFile is the protocol limit for a single sitemap file.
const MaxURLsPerFile = 50000

// Locale maps a hreflang code (e.g. "en", "zh-Hans", "ru-KZ") to a URL prefix.
type Locale struct {
	Code   string `json:"code"`
	Prefix string `json:"prefix"` // "" for the root, "/ru", "https://ru.example.com", ...
}

// Page is a logical page that exists in several locales.
type Page struct {
	Path       string    `json:"path"`              // "/pricing"
	Locales    []string  `json:"locales,omitempty"` // subset of site locales; empty = all
	LastMod    time.Time `json:"lastmod,omitempty"`
	ChangeFreq string    `json:"changefreq,omitempty"` // daily, weekly, ...
	Priority   float64   `json:"priority,omitempty"`   // 0.0–1.0
}

// Config describes a whole site.
type Config struct {
	BaseURL        string   `json:"baseURL"`
	Locales        []Locale `json:"locales"`
	DefaultLocale  string   `json:"defaultLocale,omitempty"` // emitted as x-default
	TrailingSlash  bool     `json:"trailingSlash,omitempty"`
	Pages          []Page   `json:"pages"`
	MaxURLsPerFile int      `json:"maxURLsPerFile,omitempty"`
}

var (
	langRe      = regexp.MustCompile(`^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})*$`)
	validFreqs  = map[string]bool{"": true, "always": true, "hourly": true, "daily": true, "weekly": true, "monthly": true, "yearly": true, "never": true}
	errNoLocale = errors.New("at least one locale is required")
)

// Validate reports every configuration problem it finds at once.
func (c *Config) Validate() error {
	var errs []error
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		errs = append(errs, fmt.Errorf("baseURL %q must be an absolute URL", c.BaseURL))
	}
	if len(c.Locales) == 0 {
		errs = append(errs, errNoLocale)
	}
	codes := map[string]bool{}
	for _, l := range c.Locales {
		if !langRe.MatchString(l.Code) {
			errs = append(errs, fmt.Errorf("locale %q is not a valid hreflang code", l.Code))
		}
		if codes[strings.ToLower(l.Code)] {
			errs = append(errs, fmt.Errorf("duplicate locale %q", l.Code))
		}
		codes[strings.ToLower(l.Code)] = true
	}
	if c.DefaultLocale != "" && !codes[strings.ToLower(c.DefaultLocale)] {
		errs = append(errs, fmt.Errorf("defaultLocale %q is not in locales", c.DefaultLocale))
	}
	for i, p := range c.Pages {
		if !strings.HasPrefix(p.Path, "/") {
			errs = append(errs, fmt.Errorf("pages[%d]: path %q must start with /", i, p.Path))
		}
		if !validFreqs[p.ChangeFreq] {
			errs = append(errs, fmt.Errorf("pages[%d]: invalid changefreq %q", i, p.ChangeFreq))
		}
		if p.Priority < 0 || p.Priority > 1 {
			errs = append(errs, fmt.Errorf("pages[%d]: priority must be within 0..1", i))
		}
		for _, lc := range p.Locales {
			if !codes[strings.ToLower(lc)] {
				errs = append(errs, fmt.Errorf("pages[%d]: unknown locale %q", i, lc))
			}
		}
	}
	return errors.Join(errs...)
}

// ─── XML model ───────────────────────────────────────────────────────────────

type urlset struct {
	XMLName xml.Name  `xml:"urlset"`
	NS      string    `xml:"xmlns,attr"`
	XHTML   string    `xml:"xmlns:xhtml,attr"`
	URLs    []urlNode `xml:"url"`
}

type urlNode struct {
	Loc        string     `xml:"loc"`
	LastMod    string     `xml:"lastmod,omitempty"`
	ChangeFreq string     `xml:"changefreq,omitempty"`
	Priority   string     `xml:"priority,omitempty"`
	Links      []linkNode `xml:"xhtml:link"`
}

type linkNode struct {
	Rel      string `xml:"rel,attr"`
	HrefLang string `xml:"hreflang,attr"`
	Href     string `xml:"href,attr"`
}

type sitemapIndex struct {
	XMLName  xml.Name     `xml:"sitemapindex"`
	NS       string       `xml:"xmlns,attr"`
	Sitemaps []sitemapRef `xml:"sitemap"`
}

type sitemapRef struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

const (
	nsSitemap = "http://www.sitemaps.org/schemas/sitemap/0.9"
	nsXHTML   = "http://www.w3.org/1999/xhtml"
)

// ─── Building ────────────────────────────────────────────────────────────────

// Entry is one localized URL with its full alternate set.
type Entry struct {
	Loc        string
	Alternates map[string]string // hreflang → href (includes itself and x-default)
	LastMod    time.Time
	ChangeFreq string
	Priority   float64
}

// Build expands pages × locales into entries, sorted for stable diffs.
func (c *Config) Build() ([]Entry, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	byCode := map[string]Locale{}
	for _, l := range c.Locales {
		byCode[strings.ToLower(l.Code)] = l
	}

	var out []Entry
	for _, p := range c.Pages {
		codes := p.Locales
		if len(codes) == 0 {
			for _, l := range c.Locales {
				codes = append(codes, l.Code)
			}
		}
		alts := make(map[string]string, len(codes)+1)
		for _, code := range codes {
			l := byCode[strings.ToLower(code)]
			alts[l.Code] = c.urlFor(l, p.Path)
		}
		if c.DefaultLocale != "" {
			if href, ok := alts[byCode[strings.ToLower(c.DefaultLocale)].Code]; ok {
				alts["x-default"] = href
			}
		}
		// A lone locale without alternates doesn't need hreflang at all.
		if len(codes) == 1 {
			alts = nil
		}
		for _, code := range codes {
			l := byCode[strings.ToLower(code)]
			out = append(out, Entry{
				Loc:        c.urlFor(l, p.Path),
				Alternates: alts,
				LastMod:    p.LastMod,
				ChangeFreq: p.ChangeFreq,
				Priority:   p.Priority,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Loc < out[j].Loc })
	return out, nil
}

func (c *Config) urlFor(l Locale, path string) string {
	base := strings.TrimRight(c.BaseURL, "/")
	prefix := strings.TrimRight(l.Prefix, "/")
	if strings.HasPrefix(prefix, "http://") || strings.HasPrefix(prefix, "https://") {
		base, prefix = prefix, ""
	}
	p := path
	if p == "/" {
		p = ""
	}
	full := base + prefix + p
	if c.TrailingSlash && !strings.HasSuffix(full, "/") {
		full += "/"
	}
	if !c.TrailingSlash && prefix == "" && p == "" {
		full += "/" // bare origin always gets a slash
	}
	return full
}

// ─── Writing ─────────────────────────────────────────────────────────────────

// WriteURLSet writes a single <urlset> document.
func WriteURLSet(w io.Writer, entries []Entry) error {
	set := urlset{NS: nsSitemap, XHTML: nsXHTML}
	for _, e := range entries {
		n := urlNode{Loc: e.Loc, ChangeFreq: e.ChangeFreq}
		if !e.LastMod.IsZero() {
			n.LastMod = e.LastMod.UTC().Format("2006-01-02")
		}
		if e.Priority > 0 {
			n.Priority = fmt.Sprintf("%.1f", e.Priority)
		}
		keys := make([]string, 0, len(e.Alternates))
		for k := range e.Alternates {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			n.Links = append(n.Links, linkNode{Rel: "alternate", HrefLang: k, Href: e.Alternates[k]})
		}
		set.URLs = append(set.URLs, n)
	}
	return encode(w, set)
}

// WriteIndex writes a <sitemapindex> pointing at the given sitemap URLs.
func WriteIndex(w io.Writer, locs []string, lastMod time.Time) error {
	idx := sitemapIndex{NS: nsSitemap}
	for _, l := range locs {
		ref := sitemapRef{Loc: l}
		if !lastMod.IsZero() {
			ref.LastMod = lastMod.UTC().Format("2006-01-02")
		}
		idx.Sitemaps = append(idx.Sitemaps, ref)
	}
	return encode(w, idx)
}

// Chunk splits entries into groups that respect the per-file limit.
// Alternates of the same page may land in different files — that is valid,
// because every entry carries its full alternate set.
func Chunk(entries []Entry, size int) [][]Entry {
	if size <= 0 || size > MaxURLsPerFile {
		size = MaxURLsPerFile
	}
	var out [][]Entry
	for len(entries) > size {
		out = append(out, entries[:size])
		entries = entries[size:]
	}
	if len(entries) > 0 {
		out = append(out, entries)
	}
	return out
}

func encode(w io.Writer, v any) error {
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}
