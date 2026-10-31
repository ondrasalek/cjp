package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ondrejsalek/cjp/internal/api"
	"github.com/ondrejsalek/cjp/internal/db"
	"github.com/ondrejsalek/cjp/internal/hunspell"
	"github.com/ondrejsalek/cjp/internal/hyphen"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "data/cjp.db", "Path to SQLite database")
	rawDir := flag.String("raw-dir", "data/raw", "Path to raw pattern/affix files")
	webDir := flag.String("web-dir", "web/dist", "Path to built frontend static directory (SPA)")
	useHunspell := flag.Bool("hunspell", true, "Load Hunspell cs_CZ in memory for dynamic affix expansion & spellcheck")
	rateLimitRPS := flag.Float64("rate-limit", 5.0, "Per-IP sustained API rate limit in requests/sec (<= 0 disables)")
	rateLimitBurst := flag.Int("rate-burst", 30, "Per-IP API burst capacity")
	flag.Parse()

	if _, err := os.Stat(*dbPath); err != nil {
		log.Fatalf("database %s not found (run 'cjp-etl fetch && cjp-etl build' first): %v", *dbPath, err)
	}

	store, err := db.Open(*dbPath, true)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer store.Close()

	var hyph *hyphen.Hyphenator
	hyphPath := filepath.Join(*rawDir, "hyph_cs_CZ.dic")
	if h, err := hyphen.LoadFile(hyphPath); err == nil {
		hyph = h
		log.Printf("loaded Czech hyphenation patterns from %s", hyphPath)
	} else {
		log.Printf("warning: hyphenation file not loaded (%v)", err)
	}

	var hun *hunspell.Dictionary
	if *useHunspell {
		affPath := filepath.Join(*rawDir, "cs_CZ.aff")
		dicPath := filepath.Join(*rawDir, "cs_CZ.dic")
		if _, err := os.Stat(dicPath); err == nil {
			t0 := time.Now()
			h, err := hunspell.Load(affPath, dicPath)
			if err != nil {
				log.Printf("warning: failed to load hunspell (%v)", err)
			} else {
				hun = h
				log.Printf("loaded Hunspell cs_CZ (%d stems) in %s", h.StemsCount(), time.Since(t0).Round(time.Millisecond))
			}
		}
	}

	srv := api.NewServer(api.Config{
		Store:      store,
		Hyphenator: hyph,
		Hunspell:   hun,
		WebDir:     *webDir,
		RateLimit: api.RateLimitConfig{
			RPS:   *rateLimitRPS,
			Burst: *rateLimitBurst,
		},
	})

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		fmt.Printf("Czech Lexical API listening on http://localhost%s\n", *addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http listen: %v", err)
		}
	}()

	<-ctx.Done()
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutCtx)
}
