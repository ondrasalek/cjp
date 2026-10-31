package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/ondrejsalek/cjp/internal/etl"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	ctx := context.Background()
	sub := os.Args[1]
	switch sub {
	case "fetch":
		fs := flag.NewFlagSet("fetch", flag.ExitOnError)
		rawDir := fs.String("raw-dir", "data/raw", "Directory for downloaded raw files")
		includeGPL := fs.Bool("include-gpl", true, "Also download GPL/GFDL Hunspell & Thesaurus files")
		force := fs.Bool("force", false, "Re-download even if files exist")
		_ = fs.Parse(os.Args[2:])

		if err := etl.FetchDatasets(ctx, *rawDir, *includeGPL, *force); err != nil {
			fmt.Fprintf(os.Stderr, "fetch error: %v\n", err)
			os.Exit(1)
		}

	case "build":
		fs := flag.NewFlagSet("build", flag.ExitOnError)
		rawDir := fs.String("raw-dir", "data/raw", "Directory containing downloaded raw files")
		outDB := fs.String("db", "data/cjp.db", "Output SQLite database path")
		includeGPL := fs.Bool("include-gpl", true, "Include GPL/GFDL Hunspell & Thesaurus datasets")
		_ = fs.Parse(os.Args[2:])

		if err := etl.BuildDatabase(ctx, etl.BuildOptions{
			RawDir:     *rawDir,
			OutDBPath:  *outDB,
			IncludeGPL: *includeGPL,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "build error: %v\n", err)
			os.Exit(1)
		}

	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: cjp-etl <fetch|build> [flags]\n")
}
