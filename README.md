# sld-svg
Single-line diagram processing utilities - SVG

## svg-text

`svg-text` extracts translatable text from SLD SVG files (`data-name="..."` attributes and `<text>`/`<tspan>` content) into per-file `.csv` translation files, and later injects hand-translated text back into the original SVGs — byte-for-byte preserving everything else in the file.

Each SVG gets its own companion `.csv` file with three columns: `key` (a stable hash of the source string), `source`, and `translation` (left blank until filled in by a translator). Because the key is content-based rather than positional, the file can be freely reordered or hand-edited, and identical strings within a file automatically collapse to a single row.

### CSV format

- Semicolon-separated, UTF-8, with a header row: `key;source;translation`. A semicolon is used instead of a tab because source strings in this corpus routinely contain literal commas (Russian decimal notation, e.g. "2,5") but never semicolons — and unlike a tab, a semicolon can't get silently turned into spaces by an editor's "insert spaces for tabs" setting.
- `key` — the first 16 hex characters of the SHA-256 digest of `source`. It is how `inject` finds this row again — never edit it, and don't rely on row order or line numbers.
- `source` — the original text as it appears in the SVG (entities like `&#34;` already decoded to `"`), trimmed of surrounding whitespace. Read-only reference for the translator; `inject` ignores it.
- `translation` — empty by default. Fill in the target-language text; leave it empty to leave that string untouched on injection.
- Fields are quoted per standard CSV rules whenever they contain a semicolon, quote, or newline, so a `source`/`translation` spanning multiple lines is safe to store as-is.

Example (`strings/substation.csv`):
```
key;source;translation
8f0e3b60f9969858;1 c.;
b7acbb67fb16628c;1 c.-I СШ;
585a87153cf85f52;1 РЛЛ-180;
d0aa82f11bf543d7;1 РШ 1 СШ Л-203;
```

### Extract strings for translation

Single file:
```
go run ./cmd/svg-text extract -in examples/sld/substation.svg -out strings/substation.csv
```

Whole directory (recurses into subdirectories, mirroring their structure under `-out`, with `.svg` swapped for `.csv`):
```
go run ./cmd/svg-text extract -in examples/sld -out strings
```

Open the generated `.csv` files (they open cleanly in a spreadsheet app) and fill in the `translation` column. Re-running `extract` later (e.g. after an SVG changes) carries forward any translations already present under matching keys.

Use `-dictionary` to pre-fill still-blank translations from a lookup table of exact, whole-string matches — common single words/units that recur across many SVGs (`Гц;Hz`, `В;V`, `кВ;kV`, ...). [`dictionary.csv`](dictionary.csv) at the repo root is a starter set of electrical units; it's a plain two-column `source;translation` CSV (also `;`-separated), so add your own rows or point `-dictionary` at your own file:
```
go run ./cmd/svg-text extract -in examples/sld -out strings -dictionary dictionary.csv
```
```
key;source;translation
8ced98b958cbabb5;кВ;kV
1d15dbf70d129544;А;A
```

Add `--ru-en` to pre-fill any translation still blank after the dictionary lookup with a plain Cyrillic-to-Latin transliteration (а→a, б→b, ж→zh, щ→shch, ...) instead of leaving it empty — a starting point to hand-edit into a real translation, not a translation itself. Neither `-dictionary` nor `--ru-en` ever overwrites a translation that's already present, and they can be combined (dictionary matches win, everything else falls through to transliteration):
```
go run ./cmd/svg-text extract -in examples/sld -out strings -dictionary dictionary.csv --ru-en
```
```
key;source;translation
8ced98b958cbabb5;кВ;kV
72a1642d8862f47b;1 сш 10 кВ;1 ssh 10 kV
```

### Inject translations back into the SVGs

Single file:
```
go run ./cmd/svg-text inject -in examples/sld/substation.svg -translations strings/substation.csv -out translated/substation.svg
```

Whole directory:
```
go run ./cmd/svg-text inject -in examples/sld -translations strings -out translated
```

`inject` never modifies the originals in place — it writes translated copies under `-out`. Any occurrence with no (or a blank) translation is left completely untouched in the output.

Some SVGs have Cyrillic filenames themselves (e.g. `Схема ТП Пример Л-1 ПС Образец.svg`). Add `--filename-en` (inject only) to transliterate the output `.svg` filename too, the same way `--ru-en` transliterates text content:
```
go run ./cmd/svg-text inject -in examples/sld -translations strings -out translated --filename-en
```
`examples/sld/Схема ТП Пример Л-1 ПС Образец.svg` is written as `translated/Shema TP Primer L-1 PS Obrazets.svg`. Only the filename itself is transliterated — subdirectory names in the mirrored structure are left as-is.
