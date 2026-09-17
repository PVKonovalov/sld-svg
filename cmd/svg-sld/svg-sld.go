// Command svg-sld extracts a single-line-diagram XML document (elements,
// coordinates, and electrical topology) from an xsde2svg-generated SVG, and
// renders such a document back into a fresh SVG using a symbol library.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/PVKonovalov/slddoc"
)

// defaultStateColors preserves this tool's own previous fixed switching-device
// fill convention (open/closed/other), from before Render took a
// caller-supplied legend.
var defaultStateColors = []slddoc.StateColor{
	{State: 0, Color: "red"},
	{State: 1, Color: "lawngreen"},
	{State: 2, Color: "yellow"},
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "extract":
		err = runExtract(os.Args[2:])
	case "render":
		err = runRender(os.Args[2:])
	case "-h", "-help", "--help":
		usage()
		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "svg-sld:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage:
  svg-sld extract -in <svg-file-or-dir> -out <xml-file-or-dir> [-voltage-hints <csv>]
  svg-sld render  -in <xml-file-or-dir> -symbols <symbols.xml> -out <svg-file-or-dir>

A directory -in is walked recursively; the corresponding -out path mirrors
its subdirectory structure, with the extension swapped as appropriate.

svg-sld only understands a representative subset of xsde2svg's equipment
catalog (busbars, generic wires, junction points, breakers, disconnectors,
ground switches, 2-winding power transformers, lamp status indicators, and
fault passage indicators). Everything else in a source SVG is left out of
the extracted diagram; extract reports what it skipped.`)
}

func runExtract(args []string) error {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	inPath := fs.String("in", "", "SVG file or directory to scan (required)")
	outPath := fs.String("out", "", "Output .xml file, or directory when -in is a directory (required)")
	hintsPath := fs.String("voltage-hints", "", "Optional hexColor;name CSV of known voltage-class colors, see voltage-hints.csv")
	fs.Parse(args)

	if *inPath == "" || *outPath == "" {
		return fmt.Errorf("extract: -in and -out are required")
	}

	var hints map[string]string
	if *hintsPath != "" {
		f, err := os.Open(*hintsPath)
		if err != nil {
			return err
		}
		defer f.Close()
		hints, err = slddoc.LoadVoltageHints(f)
		if err != nil {
			return fmt.Errorf("reading voltage hints %s: %w", *hintsPath, err)
		}
	}

	info, err := os.Stat(*inPath)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return extractOne(*inPath, *outPath, hints)
	}

	files, err := walkFiles(*inPath, ".svg")
	if err != nil {
		return err
	}
	for _, f := range files {
		rel, err := filepath.Rel(*inPath, f)
		if err != nil {
			return err
		}
		if err := extractOne(f, filepath.Join(*outPath, swapExt(rel, ".xml")), hints); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
	}
	return nil
}

func extractOne(svgPath, outXMLPath string, hints map[string]string) error {
	raw, err := os.ReadFile(svgPath)
	if err != nil {
		return err
	}

	d, report, err := slddoc.Extract(raw, filepath.Base(svgPath), hints)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outXMLPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(outXMLPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := d.Save(f); err != nil {
		return err
	}

	fmt.Printf("%s -> %s: %d elements, %d connectors, %d nodes, %d labels",
		svgPath, outXMLPath, report.Elements, report.Connectors, report.Nodes, report.Labels)
	if len(report.Skipped) > 0 {
		fmt.Printf(", skipped types: %v", report.Skipped)
	}
	if n := len(report.Failed); n > 0 {
		fmt.Printf(", %d failed to parse: %v", n, report.Failed)
	}
	fmt.Println()
	return nil
}

func runRender(args []string) error {
	fs := flag.NewFlagSet("render", flag.ExitOnError)
	inPath := fs.String("in", "", "Diagram .xml file or directory to render (required)")
	symbolsPath := fs.String("symbols", "", "Symbol library .xml file (required), see symbols.xml")
	outPath := fs.String("out", "", "Output .svg file, or directory when -in is a directory (required)")
	fs.Parse(args)

	if *inPath == "" || *symbolsPath == "" || *outPath == "" {
		return fmt.Errorf("render: -in, -symbols and -out are required")
	}

	sf, err := os.Open(*symbolsPath)
	if err != nil {
		return err
	}
	lib, err := slddoc.LoadSymbolLibrary(sf)
	sf.Close()
	if err != nil {
		return fmt.Errorf("reading symbol library %s: %w", *symbolsPath, err)
	}

	info, err := os.Stat(*inPath)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return renderOne(*inPath, *outPath, lib)
	}

	files, err := walkFiles(*inPath, ".xml")
	if err != nil {
		return err
	}
	for _, f := range files {
		rel, err := filepath.Rel(*inPath, f)
		if err != nil {
			return err
		}
		if err := renderOne(f, filepath.Join(*outPath, swapExt(rel, ".svg")), lib); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
	}
	return nil
}

func renderOne(xmlPath, outSVGPath string, lib *slddoc.SymbolLibrary) error {
	f, err := os.Open(xmlPath)
	if err != nil {
		return err
	}
	d, err := slddoc.Load(f)
	f.Close()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outSVGPath), 0o755); err != nil {
		return err
	}
	out, err := os.Create(outSVGPath)
	if err != nil {
		return err
	}
	defer out.Close()

	renderErr := slddoc.Render(d, lib, out, slddoc.Static, nil, defaultStateColors...)
	fmt.Printf("%s -> %s: %d elements, %d connectors\n", xmlPath, outSVGPath, len(d.Elements), len(d.Connectors))
	return renderErr
}

func walkFiles(root, ext string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, dEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dEntry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ext) {
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
