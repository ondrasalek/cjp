package etl

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// RemoteDataset describes an open linguistic resource to download.
type RemoteDataset struct {
	Name        string
	Filename    string
	URLs        []string // Primary + fallback URLs
	License     string
	GPLOptional bool
}

// DefaultDatasets returns the open Czech lexical resources.
func DefaultDatasets(includeGPL bool) []RemoteDataset {
	ds := []RemoteDataset{
		{
			Name:     "Czech Wiktionary (Kaikki JSONL)",
			Filename: "cs-extract.jsonl.gz",
			URLs: []string{
				"https://kaikki.org/dictionary/downloads/cs/cs-extract.jsonl.gz",
			},
			License: "CC BY-SA 4.0",
		},
		{
			Name:     "UniMorph Czech (Wiktionary branch)",
			Filename: "unimorph_ces.tsv",
			URLs: []string{
				"https://raw.githubusercontent.com/unimorph/ces/master/ces",
			},
			License: "CC BY-SA 3.0",
		},
		{
			Name:     "Czech TeX/LibreOffice Hyphenation Patterns",
			Filename: "hyph_cs_CZ.dic",
			URLs: []string{
				"https://raw.githubusercontent.com/LibreOffice/dictionaries/master/cs_CZ/hyph_cs_CZ.dic",
			},
			License: "GPL (external runtime pattern file)",
		},
	}

	if includeGPL {
		ds = append(ds,
			RemoteDataset{
				Name:     "Czech Hunspell Dictionary (.dic)",
				Filename: "cs_CZ.dic",
				URLs: []string{
					"https://raw.githubusercontent.com/LibreOffice/dictionaries/master/cs_CZ/cs_CZ.dic",
				},
				License:     "GPL",
				GPLOptional: true,
			},
			RemoteDataset{
				Name:     "Czech Hunspell Affixes (.aff)",
				Filename: "cs_CZ.aff",
				URLs: []string{
					"https://raw.githubusercontent.com/LibreOffice/dictionaries/master/cs_CZ/cs_CZ.aff",
				},
				License:     "GPL",
				GPLOptional: true,
			},
			RemoteDataset{
				Name:     "Czech LibreOffice Thesaurus (.dat)",
				Filename: "th_cs_CZ.dat",
				URLs: []string{
					"https://raw.githubusercontent.com/LibreOffice/dictionaries/master/cs_CZ/thes_cs_CZ.dat",
					"https://raw.githubusercontent.com/LibreOffice/dictionaries/master/cs_CZ/th_cs_CZ_v2.dat",
					"https://raw.githubusercontent.com/LibreOffice/dictionaries/master/cs_CZ/th_cs_CZ.dat",
				},
				License:     "GFDL / GPL",
				GPLOptional: true,
			},
		)
	}

	return ds
}

// FetchDatasets downloads missing datasets into rawDir.
func FetchDatasets(ctx context.Context, rawDir string, includeGPL bool, force bool) error {
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		return fmt.Errorf("create raw dir: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Minute,
	}

	for _, ds := range DefaultDatasets(includeGPL) {
		dest := filepath.Join(rawDir, ds.Filename)
		if !force {
			if st, err := os.Stat(dest); err == nil && st.Size() > 0 {
				fmt.Printf("[skip] %s already exists (%s, %.2f MB)\n", ds.Filename, ds.License, float64(st.Size())/(1024*1024))
				continue
			}
		}

		var lastErr error
		downloaded := false
		for _, u := range ds.URLs {
			fmt.Printf("[fetch] %s (%s) <- %s ...\n", ds.Name, ds.License, u)
			n, err := downloadFile(ctx, client, u, dest)
			if err == nil {
				fmt.Printf("[done]  %s (%.2f MB)\n", ds.Filename, float64(n)/(1024*1024))
				downloaded = true
				break
			}
			lastErr = err
		}
		if !downloaded {
			return fmt.Errorf("failed to download %s: %w", ds.Name, lastErr)
		}
	}

	return nil
}

func downloadFile(ctx context.Context, client *http.Client, url, dest string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "cjp-etl/1.0 (Open Czech Lexical Database Builder)")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}

	n, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return 0, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return 0, closeErr
	}

	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	return n, nil
}
