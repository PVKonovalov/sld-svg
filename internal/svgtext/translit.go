package svgtext

import (
	"strings"
	"unicode"
)

// ruToLatin maps each lowercase Cyrillic letter to its Latin expansion.
// This is a practical, letter-by-letter romanization meant to give a
// translator a readable starting point (e.g. for the -ru-en extract flag),
// not an authoritative or reversible transliteration standard.
var ruToLatin = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e",
	'ж': "zh", 'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m",
	'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u",
	'ф': "f", 'х': "h", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "shch",
	'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
}

// Transliterate converts Cyrillic letters in s to their Latin equivalents
// (а -> a, б -> b, ...), leaving every other character (digits,
// punctuation, whitespace, already-Latin text) untouched.
func Transliterate(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		lower := unicode.ToLower(r)
		latin, ok := ruToLatin[lower]
		if !ok {
			b.WriteRune(r)
			continue
		}
		if r != lower && latin != "" {
			// Uppercase Cyrillic letter: capitalize just the first Latin
			// rune of its expansion (Ж -> Zh, not ZH).
			latinRunes := []rune(latin)
			b.WriteRune(unicode.ToUpper(latinRunes[0]))
			b.WriteString(string(latinRunes[1:]))
			continue
		}
		b.WriteString(latin)
	}
	return b.String()
}
