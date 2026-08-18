package svgtext

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashKey returns a stable key for a (trimmed, entity-decoded) source
// string: the first 16 hex characters of its SHA-256 digest. Identical
// source strings always produce the same key, which is what lets a
// translation file be reordered freely without breaking the link back to
// its place in the SVG.
func HashKey(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:8])
}
