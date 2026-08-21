# sld-svg
Single-line diagram processing utilities - SVG

## svg-text

`svg-text` extracts translatable text from SLD SVG files (`data-name="..."` attributes and `<text>`/`<tspan>` content) into per-file `.csv` translation files, and later injects hand-translated text back into the original SVGs — byte-for-byte preserving everything else in the file.

Each SVG gets its own companion `.csv` file with three columns: `key` (a stable hash of the source string), `source`, and `translation` (left blank until filled in by a translator). Because the key is content-based rather than positional, the file can be freely reordered or hand-edited, and identical strings within a file automatically collapse to a single row.

### CSV format

- Semicolon-separated, UTF-8, with a header row: `key;source;translation`. A semicolon is used instead of a tab because source strings in this corpus routinely contain literal commas (Russian decimal notation, e.g. "2,5") but never semicolons — and unlike a tab, a semicolon can't get silently turned into spaces by an editor's "insert spaces for tabs" setting.
- `key` — the first 16 hex characters of the SHA-256 digest of `source`. It is how `inject` finds this row again — never edit it, and don't rely on row order or line numbers.
- `source` — the original text as it appears in the SVG (entities like `&#34;` already decoded to `"`), trimmed of surrounding whitespace. Read-only reference for the translator; `inject` ignores it.
- `translation` — empty by default. Fill in the target-language text; leave it empty to leave that string untouched on injection. Set it to the literal word `null` (case-insensitive) to mean something different from "leave untouched": *delete* this string, replacing it with an empty string in the injected output. This is meant for placeholder/template text that shouldn't be translated at all — e.g. these SLD SVGs embed literal live-clock placeholders like `DD.MM.YYYY` and `DD:MM:YYYY HH:MM:SS`, which `dictionary.csv` already maps to `null` so they're stripped out automatically.
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

Open the generated `.csv` files (they open cleanly in a spreadsheet app) and fill in the `translation` column. Re-running `extract` later (e.g. after an SVG changes) carries forward any translations already present under matching keys — it never overwrites a translation that's already there, dictionary/`--ru-en` included, since those already-filled rows look no different from a manually-confirmed translation.

If you've since improved `-dictionary` and want those improvements applied to `.csv` files that were already fully filled by an earlier `--ru-en`/`-dictionary` run, add `-rewrite` to discard the existing output and recompute every row from scratch instead of preserving what's on disk:
```
go run ./cmd/svg-text extract -in examples/sld -out strings -dictionary dictionary.csv --ru-en -rewrite
```

Use `-dictionary` to pre-fill still-blank translations from a lookup table of exact, case- and whitespace-insensitive, whole-string matches — single words or whole phrases that recur across many SVGs (`Гц;Hz`, `В;V`, `кВ;kV`, `1СШ 10кВ;1SEC 10kV`, ...). Whitespace-insensitive means every space is dropped before comparing, not just collapsed, so `1СШ 10кВ` also matches `1 сш 10 кВ` — the same tokens with a space landing in a different place, a real formatting variance in this corpus. Matching is still on the entire source string, not a substring, so a phrase entry only fires when a source string equals it exactly (case/spacing aside) — it won't partially translate inside a longer string. [`dictionary.csv`](dictionary.csv) at the repo root is a starter set of electrical units; it's a plain two-column `source;translation` CSV (also `;`-separated), so add your own rows (single words or phrases) or point `-dictionary` at your own file:
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

## svg-sld

`svg-sld` extracts a diagram's placed elements, their coordinates, and the electrical topology connecting them out of an SLD SVG into a standalone `.xml` document, and can render such a document back into a fresh SVG using a symbol library.

It only understands a subset of the equipment xsde2svg can draw — busbars, generic wires, junction points, breakers (fixed and withdrawable), load-break switches, disconnectors (fixed and withdrawable), ground switches, ground terminals, choke coils, current transformers, surge arresters, fuses, capacitors, and 2-winding power transformers. Anything else in the source SVG is left out of the extracted diagram; `extract` reports what it skipped rather than guessing.

### Extract a diagram

```
go run ./cmd/svg-sld extract -in examples/sld/substation.svg -out diagrams/substation.xml -voltage-hints voltage-hints.csv
```

Whole directory (recurses, mirroring structure, `.svg` swapped for `.xml`):
```
go run ./cmd/svg-sld extract -in examples/sld -out diagrams -voltage-hints voltage-hints.csv
```

`-voltage-hints` is optional; it pre-fills a diagram's `<voltageClasses>` table with the real voltage-level name (`"10кВ"`, `"6кВ"`, ...) for colors already confirmed elsewhere in the corpus — see [`voltage-hints.csv`](voltage-hints.csv) at the repo root. Without it, a class still gets extracted, just named after its own color as a placeholder to fill in by hand.

Example output (trimmed):
```xml
<diagram width="2230" height="1600" source="RP_10kV_Vypolzovo.svg">
  <layers>
    <layer id="0" name="Base"></layer>
    <layer id="10" name="Контейнеры"></layer>
  </layers>
  <voltageClasses>
    <class id="v1" name="6кВ" color="#326400"></class>
    <class id="v2" name="10кВ" color="#962896"></class>
  </voltageClasses>
  <nodes>
    <node id="n1" x="810" y="240"></node>
    <node id="n2" x="900" y="270"></node>
    ...
  </nodes>
  <elements>
    <element id="148694372" class="BusBarSection" shape="24" name="СШ 6 кВ" voltage="v1" layer="0" x="810" y="240">
      <geometry>
        <point x="810" y="240"></point>
        <point x="1320" y="240"></point>
      </geometry>
    </element>
    <element id="2413" class="Breaker" shape="41" name="В-10 Л-22" voltage="v2" layer="0" x="900" y="660" state="1">
      <port name="1" node="n12"></port>
      <port name="2" node="n13"></port>
    </element>
    ...
  </elements>
  <connectors>
    <connector id="2405" kind="BusbarWire" voltage="v2" layer="0" from="n11" to="n12">
      <point x="900" y="590"></point>
      <point x="900" y="650"></point>
    </connector>
    ...
  </connectors>
  <labels>
    <label for="2413" layer="0" x="920" y="663" size="13" anchor="start">В-10 Л-22</label>
    ...
  </labels>
</diagram>
```

`class`/`shape` are independent: `class` is the equipment's real-world kind (`Breaker`, `Disconnector`, ...), while `shape` is the original xsde2svg numeric code (`"41"` vs `"43"` for a fixed vs. withdrawable breaker) — two shapes can share one class but need different render templates. `voltage` on an element/connector is a `<voltageClasses>` id, never a raw color. Every element/connector/label carries a `layer`, always resolvable against `<layers>` (an implicit `"0"`/`Base` layer is added even when the source SVG never wrote a `data-layer`), so a viewer can show/hide by layer.

### Render a diagram back into SVG

```
go run ./cmd/svg-sld render -in diagrams/substation.xml -symbols symbols.xml -out rendered/substation.svg
```

Whole directory:
```
go run ./cmd/svg-sld render -in diagrams -symbols symbols.xml -out rendered
```

[`symbols.xml`](symbols.xml) at the repo root is the tracked symbol library — one local-coordinate SVG template per `shape` code. `render` does not aim to byte-for-byte reproduce the original SVG (unlike `svg-text inject`); it draws a fresh document from the diagram model. If an element's `shape` has no template in the library, `render` still writes everything else and reports the missing shape(s) once the document is complete, instead of stopping at the first one.
