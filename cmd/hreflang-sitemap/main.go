// Command hreflang-sitemap generates multilingual XML sitemaps from a JSON config.
//
//	hreflang-sitemap -config site.json -out ./public
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tenzo-run/hreflang-sitemap/sitemap"
)

func main() {
	var (
		cfgPath = flag.String("config", "sitemap.json", "path to JSON config")
		outDir  = flag.String("out", ".", "output directory")
		check   = flag.Bool("check", false, "validate config only, write nothing")
	)
	flag.Parse()

	if err := run(*cfgPath, *outDir, *check); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(cfgPath, outDir string, checkOnly bool) error {
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	var cfg sitemap.Config
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return fmt.Errorf("parse %s: %w", cfgPath, err)
	}

	entries, err := cfg.Build()
	if err != nil {
		return fmt.Errorf("invalid config:\n%w", err)
	}
	if checkOnly {
		fmt.Printf("✓ config OK — %d pages → %d URLs\n", len(cfg.Pages), len(entries))
		return nil
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	chunks := sitemap.Chunk(entries, cfg.MaxURLsPerFile)
	if len(chunks) <= 1 {
		if err := writeFile(filepath.Join(outDir, "sitemap.xml"), func(f *os.File) error {
			return sitemap.WriteURLSet(f, entries)
		}); err != nil {
			return err
		}
		fmt.Printf("✓ sitemap.xml — %d URLs\n", len(entries))
		return nil
	}

	base := strings.TrimRight(cfg.BaseURL, "/")
	var locs []string
	for i, chunk := range chunks {
		name := fmt.Sprintf("sitemap-%d.xml", i+1)
		if err := writeFile(filepath.Join(outDir, name), func(f *os.File) error {
			return sitemap.WriteURLSet(f, chunk)
		}); err != nil {
			return err
		}
		locs = append(locs, base+"/"+name)
		fmt.Printf("✓ %s — %d URLs\n", name, len(chunk))
	}
	if err := writeFile(filepath.Join(outDir, "sitemap.xml"), func(f *os.File) error {
		return sitemap.WriteIndex(f, locs, time.Now())
	}); err != nil {
		return err
	}
	fmt.Printf("✓ sitemap.xml — index of %d files\n", len(locs))
	return nil
}

// writeFile writes atomically: temp file + rename, so a crawler never sees half a sitemap.
func writeFile(path string, fn func(*os.File) error) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sitemap-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := fn(tmp); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
