// Package slddoc models a single-line diagram (SLD) as a standalone XML
// document — elements, their coordinates, and the electrical topology
// connecting them — extracted from an xsde2svg-generated SVG, and later
// rendered back into a fresh SVG using a symbol library.
//
// Unlike internal/svgtext, this package does not aim for byte-identical
// round-tripping: it builds a new object model from the SVG's geometry and
// re-renders from scratch, so exact original formatting is not preserved.
package slddoc
