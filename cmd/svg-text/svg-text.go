// Command svg-text extracts translatable text (data-name="..." attributes
// and <text>/<tspan> content) from single-line-diagram SVG files into
// per-file .csv translation files, and later injects hand-translated text
// back into the original SVGs.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"sld-svg/internal/svgtext"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "extract":
		err = runExtract(os.Args[2:])
	case "inject":
		err = runInject(os.Args[2:])
	case "-h", "-help", "--help":
		usage()
		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "svg-text:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage:
  svg-text extract -in <svg-file-or-dir> -out <csv-file-or-dir>
  svg-text inject  -in <svg-file-or-dir> -translations <csv-file-or-dir> -out <svg-file-or-dir>

A directory -in is walked recursively for *.svg files; the corresponding
-out (and -translations) paths mirror its subdirectory structure, with the
.svg extension swapped for .csv where applicable.`)
}

func runExtract(args []string) error {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	inPath := fs.String("in", "", "SVG file or directory to scan (required)")
	outPath := fs.String("out", "", "Output .csv file, or directory when -in is a directory (required)")
	ruEn := fs.Bool("ru-en", false, "Pre-fill blank translations with a Cyrillic-to-Latin transliteration (а->a, б->b, ...) instead of leaving them empty")
	fs.Parse(args)

	if *inPath == "" || *outPath == "" {
		return fmt.Errorf("extract: -in and -out are required")
	}

	info, err := os.Stat(*inPath)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return extractOne(*inPath, *outPath, *ruEn)
	}

	files, err := walkSVGFiles(*inPath)
	if err != nil {
		return err
	}
	for _, f := range files {
		rel, err := filepath.Rel(*inPath, f)
		if err != nil {
			return err
		}
		if err := extractOne(f, filepath.Join(*outPath, swapExt(rel, ".csv")), *ruEn); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
	}
	return nil
}

func extractOne(svgPath, outCSVPath string, ruEn bool) error {
	raw, err := os.ReadFile(svgPath)
	if err != nil {
		return err
	}
	entries, err := svgtext.ExtractFile(raw)
	if err != nil {
		return err
	}

	existing := map[string]svgtext.Entry{}
	if b, err := os.ReadFile(outCSVPath); err == nil {
		existing, err = svgtext.LoadTranslations(bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("reading existing %s: %w", outCSVPath, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	added, dropped, transliterated := 0, 0, 0
	for k, e := range entries {
		if old, ok := existing[k]; ok {
			e.Translation = old.Translation
		} else {
			added++
		}
		if ruEn && e.Translation == "" {
			e.Translation = svgtext.Transliterate(e.Source)
			transliterated++
		}
		entries[k] = e
	}
	for k := range existing {
		if _, ok := entries[k]; !ok {
			dropped++
		}
	}

	if err := os.MkdirAll(filepath.Dir(outCSVPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(outCSVPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := svgtext.SaveTranslations(f, entries); err != nil {
		return err
	}

	fmt.Printf("%s -> %s: %d strings (%d new, %d matched existing, %d dropped",
		svgPath, outCSVPath, len(entries), added, len(entries)-added, dropped)
	if ruEn {
		fmt.Printf(", %d transliterated", transliterated)
	}
	fmt.Println(")")
	return nil
}

func runInject(args []string) error {
	fs := flag.NewFlagSet("inject", flag.ExitOnError)
	inPath := fs.String("in", "", "SVG file or directory to translate (required)")
	translationsPath := fs.String("translations", "", "Translated .csv file, or directory when -in is a directory (required)")
	outPath := fs.String("out", "", "Output .svg file, or directory when -in is a directory (required)")
	filenameEn := fs.Bool("filename-en", false, "Transliterate output SVG filenames from Cyrillic to Latin (а->a, б->b, ...)")
	fs.Parse(args)

	if *inPath == "" || *translationsPath == "" || *outPath == "" {
		return fmt.Errorf("inject: -in, -translations and -out are required")
	}

	info, err := os.Stat(*inPath)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return injectOne(*inPath, *translationsPath, *outPath, *filenameEn)
	}

	files, err := walkSVGFiles(*inPath)
	if err != nil {
		return err
	}
	for _, f := range files {
		rel, err := filepath.Rel(*inPath, f)
		if err != nil {
			return err
		}
		csvPath := filepath.Join(*translationsPath, swapExt(rel, ".csv"))
		outSVGPath := filepath.Join(*outPath, rel)
		if err := injectOne(f, csvPath, outSVGPath, *filenameEn); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
	}
	return nil
}

func injectOne(svgPath, csvPath, outSVGPath string, filenameEn bool) error {
	if filenameEn {
		outSVGPath = filepath.Join(filepath.Dir(outSVGPath), svgtext.Transliterate(filepath.Base(outSVGPath)))
	}

	raw, err := os.ReadFile(svgPath)
	if err != nil {
		return err
	}

	translations := map[string]svgtext.Entry{}
	if b, err := os.ReadFile(csvPath); err == nil {
		translations, err = svgtext.LoadTranslations(bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("reading %s: %w", csvPath, err)
		}
	} else if os.IsNotExist(err) {
		fmt.Printf("warning: %s: no translation file %s, writing unmodified\n", svgPath, csvPath)
	} else {
		return err
	}

	out, report, err := svgtext.InjectFile(raw, translations)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outSVGPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outSVGPath, out, 0o644); err != nil {
		return err
	}

	fmt.Printf("%s -> %s: %d/%d occurrences translated", svgPath, outSVGPath, report.Applied, report.Occurrences)
	if n := len(report.Missing); n > 0 {
		fmt.Printf(", %d missing translations", n)
	}
	if n := len(report.Invalid); n > 0 {
		fmt.Printf(", %d invalid (control characters, skipped)", n)
	}
	if n := len(report.StaleKeys); n > 0 {
		fmt.Printf(", %d stale keys unused", n)
	}
	fmt.Println()
	return nil
}

func walkSVGFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".svg") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func swapExt(path, newExt string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + newExt
}
