<div align="center">

# 🌍 hreflang-sitemap

**Multilingual XML sitemaps with correct `hreflang` alternates — a tiny, zero-dependency Go CLI & library.**

![CI](https://github.com/tenzo-run/hreflang-sitemap/actions/workflows/ci.yml/badge.svg)
![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![coverage](https://img.shields.io/badge/coverage-96%25-brightgreen)
![deps](https://img.shields.io/badge/dependencies-0-brightgreen)
![license](https://img.shields.io/badge/license-MIT-blue)

</div>

Born from shipping programmatic SEO for sites in **English, Russian and Chinese**.
Getting hreflang right by hand is where international SEO quietly breaks:
missing self-references, non-reciprocal links, no `x-default`, a 50 000-URL limit you hit
the day a new country launches. This tool makes those mistakes impossible.

## What it does

- 🔁 Expands **pages × locales** into URLs, each with the **full reciprocal alternate set** (self-reference included)
- 🎯 Adds **`x-default`** from your default locale
- 🌐 Locale prefixes as **subpaths** (`/ru`) or **subdomains / ccTLDs** (`https://cn.example.com`)
- 🧩 Per-page locale subsets — a blog post that exists only in RU/EN gets exactly those alternates
- 📦 Auto-splits into **sitemap index** past 50 000 URLs (or your own limit)
- 🛡 **Validates everything at once** — bad hreflang codes, duplicates, unknown locales, priority range, typos in JSON keys
- ⚛️ **Atomic writes** — a crawler never fetches a half-written sitemap
- 📐 Deterministic, sorted output → clean git diffs

## Install

```bash
go install github.com/tenzo-run/hreflang-sitemap/cmd/hreflang-sitemap@latest
```

## Usage

```jsonc
// site.json
{
  "baseURL": "https://example.com",
  "defaultLocale": "en",
  "locales": [
    { "code": "en",      "prefix": "" },
    { "code": "ru",      "prefix": "/ru" },
    { "code": "zh-Hans", "prefix": "/zh" }
  ],
  "pages": [
    { "path": "/",        "priority": 1.0, "changefreq": "daily" },
    { "path": "/pricing", "priority": 0.8, "lastmod": "2026-09-01T00:00:00Z" },
    { "path": "/blog/kazakhstan-logistics", "locales": ["en", "ru"] }
  ]
}
```

```bash
$ hreflang-sitemap -config site.json -out ./public
✓ sitemap.xml — 8 URLs

$ hreflang-sitemap -config site.json -check   # CI-friendly validation
✓ config OK — 3 pages → 8 URLs
```

Output:

```xml
<url>
  <loc>https://example.com/ru/pricing</loc>
  <lastmod>2026-09-01</lastmod>
  <priority>0.8</priority>
  <xhtml:link rel="alternate" hreflang="en"        href="https://example.com/pricing"/>
  <xhtml:link rel="alternate" hreflang="ru"        href="https://example.com/ru/pricing"/>
  <xhtml:link rel="alternate" hreflang="x-default" href="https://example.com/pricing"/>
  <xhtml:link rel="alternate" hreflang="zh-Hans"   href="https://example.com/zh/pricing"/>
</url>
```

Validation reports **all** problems in one run:

```text
error: invalid config:
baseURL "example.com" must be an absolute URL
duplicate locale "EN"
pages[0]: path "pricing" must start with /
pages[0]: unknown locale "fr"
```

## As a library

```go
import "github.com/tenzo-run/hreflang-sitemap/sitemap"

cfg := &sitemap.Config{ /* ... or generate pages from your DB */ }
entries, err := cfg.Build()
if err != nil { log.Fatal(err) }

for i, chunk := range sitemap.Chunk(entries, 0) {
    f, _ := os.Create(fmt.Sprintf("sitemap-%d.xml", i+1))
    sitemap.WriteURLSet(f, chunk)
    f.Close()
}
```

Generate `pages` from your database and you have **programmatic SEO** sitemaps for thousands
of service × country pages in a few lines.

## Config reference

| Field | Description |
| --- | --- |
| `baseURL` | Absolute origin, e.g. `https://example.com` |
| `locales[].code` | hreflang code: `en`, `ru-KZ`, `zh-Hans` |
| `locales[].prefix` | `""`, `/ru`, or an absolute URL for subdomains |
| `defaultLocale` | Emitted as `x-default` |
| `trailingSlash` | Force trailing `/` on every URL |
| `maxURLsPerFile` | Split threshold (default and max 50 000) |
| `pages[].path` | Path starting with `/` |
| `pages[].locales` | Optional subset of locales |
| `pages[].lastmod` / `changefreq` / `priority` | Standard sitemap fields |

## Development

```bash
go test -race -cover ./...
```

## License

MIT © [Sanzhar Abdurakhmanov](https://github.com/tenzo-run)
