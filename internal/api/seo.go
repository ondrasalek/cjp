package api

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const sitemapPageSize = 25000

func requestBaseURL(r *http.Request) string {
	proto := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0])
	if proto != "https" && proto != "http" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}

	host := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Host"), ",")[0])
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if host == "" {
		host = "localhost:5173"
	}
	// Strip control chars / slashes defensively
	host = strings.Map(func(ch rune) rune {
		if ch <= 0x20 || ch == '/' || ch == '\\' || ch == '<' || ch == '>' || ch == '"' {
			return -1
		}
		return ch
	}, host)

	return proto + "://" + host
}

func (s *Server) getSitemapLemmas(r *http.Request) ([]string, error) {
	s.statsMu.RLock()
	cached := s.sitemapLemmas
	s.statsMu.RUnlock()
	if cached != nil {
		return cached, nil
	}

	lemmas, err := s.store.ListSitemapLemmas(r.Context())
	if err != nil {
		return nil, err
	}
	s.statsMu.Lock()
	s.sitemapLemmas = lemmas
	s.statsMu.Unlock()
	return lemmas, nil
}

func (s *Server) handleRobotsTxt(w http.ResponseWriter, r *http.Request) {
	base := requestBaseURL(r)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = fmt.Fprintf(w, "User-agent: *\nAllow: /\nDisallow: /api/\n\nSitemap: %s/sitemap.xml\n", base)
}

func (s *Server) handleSitemapIndex(w http.ResponseWriter, r *http.Request) {
	lemmas, err := s.getSitemapLemmas(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	pages := (len(lemmas) + sitemapPageSize - 1) / sitemapPageSize
	if pages < 1 {
		pages = 1
	}

	base := requestBaseURL(r)
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(`<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for i := 1; i <= pages; i++ {
		loc := fmt.Sprintf("%s/sitemaps/words-%d.xml", base, i)
		buf.WriteString("  <sitemap><loc>")
		_ = xml.EscapeText(&buf, []byte(loc))
		buf.WriteString("</loc></sitemap>\n")
	}
	buf.WriteString("</sitemapindex>\n")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) handleSitemapChunk(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if !strings.HasPrefix(name, "words-") || !strings.HasSuffix(name, ".xml") {
		http.NotFound(w, r)
		return
	}
	numStr := strings.TrimSuffix(strings.TrimPrefix(name, "words-"), ".xml")
	page, err := strconv.Atoi(numStr)
	if err != nil || page < 1 {
		http.NotFound(w, r)
		return
	}

	lemmas, err := s.getSitemapLemmas(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	start := (page - 1) * sitemapPageSize
	if start > len(lemmas) || (start == len(lemmas) && page > 1) {
		http.NotFound(w, r)
		return
	}
	end := min(start+sitemapPageSize, len(lemmas))
	chunk := lemmas[start:end]

	base := requestBaseURL(r)
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")

	if page == 1 {
		buf.WriteString("  <url><loc>")
		_ = xml.EscapeText(&buf, []byte(base+"/"))
		buf.WriteString("</loc><changefreq>weekly</changefreq><priority>1.0</priority></url>\n")
	}

	for _, lemma := range chunk {
		loc := base + "/slovo/" + url.PathEscape(lemma)
		buf.WriteString("  <url><loc>")
		_ = xml.EscapeText(&buf, []byte(loc))
		buf.WriteString("</loc><changefreq>monthly</changefreq><priority>0.8</priority></url>\n")
	}
	buf.WriteString("</urlset>\n")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(buf.Bytes())
}
