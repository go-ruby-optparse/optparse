<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-optparse/brand/main/social/go-ruby-optparse-optparse.png" alt="go-ruby-optparse/optparse" width="720"></p>

# optparse — go-ruby-optparse

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-optparse.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of the deterministic, interpreter-independent
core of Ruby's [OptionParser](https://docs.ruby-lang.org/en/master/OptionParser.html)
(stdlib `optparse`)** — the argv-parsing *engine*. It models the option
specifications declared with `on(...)`, tokenizes an argv, performs
long/short/abbreviation matching, handles `=`/bundled/`--[no-]`/optional-argument
forms, coerces values, and reports the full `OptionParser::ParseError` taxonomy —
matching MRI 4.0.5 byte-for-byte.

It is the OptionParser backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but is a
**standalone, reusable** module with no dependency on the Ruby runtime.

> **What it is — and isn't.** Modeling the option specs, tokenizing argv, matching
> long/short/abbreviated names, coercing arguments (Integer/Float/Array/custom
> lists) and computing the error taxonomy is fully deterministic and needs **no
> interpreter**, so it lives here as pure Go. The per-match Ruby *blocks* that
> `on(...)` registers **do** need a Ruby interpreter and stay in the consumer
> (e.g. rbgo): this library parses argv and returns the ordered matches + coerced
> values + leftover operands + MRI-exact errors, and the host dispatches the
> blocks over those matches.

## Features

Faithful port of MRI's `lib/optparse.rb` parsing engine, validated against the
`ruby` binary on every supported platform:

- **All flag forms** — long `--name`, short `-n`, both `-n, --name`.
- **Argument styles** — required `--name VALUE` / `-n VALUE`, optional
  `--name [VALUE]`, `=`-joined `--name=VALUE` / `-n=VALUE`, glued short `-nVALUE`.
- **Bundled shorts** — `-xvf`, with the last switch in a bundle taking its argument.
- **Negation** — `--[no-]flag` matches both `--flag` (true) and `--no-flag` (false).
- **Abbreviation & completion** — unique-prefix long abbreviation (`--verb` →
  `--verbose`), short→long completion (`-n` → `--name`), with `AmbiguousOption`
  on a tie.
- **Coercion** — `Integer` (decimal + `0x`/`0b`/`0o`, `_` separators, signed,
  arbitrary precision), `Float`, `Array` (comma-split), `String`, and custom
  candidate lists / value maps with prefix completion.
- **Parse modes** — `parse!` / `permute!` (options anywhere), `order!` (stop at
  the first operand, or a callback per operand), and `getopts`.
- **Terminators** — `--` ends option parsing; a bare `-` is an operand.
- **Full error tree** — `InvalidOption`, `MissingArgument`, `InvalidArgument`,
  `AmbiguousOption`, `AmbiguousArgument`, `NeedlessArgument`, each with MRI-exact
  `message` / `reason` / `args` and `recover`.
- **Help layout** — `help` / `to_s` / `summarize` with MRI's column alignment,
  separators, and `on_head` ordering.

CGO-free, dependency-free, **100% test coverage**, `gofmt` + `go vet` clean, and
green across the six 64-bit Go targets (amd64, arm64, riscv64, loong64, ppc64le,
s390x).

## Usage

```go
package main

import (
	"fmt"

	"github.com/go-ruby-optparse/optparse"
)

func main() {
	p := optparse.New()
	p.Banner = "Usage: tool [options] FILE..."

	// Declare options from MRI `on(...)` flag strings (+ optional coercion).
	verbose := p.Define([]string{"-v", "--verbose", "run verbosely"}, "", nil, nil)
	count := p.Define([]string{"--count N", "how many"}, optparse.CoerceInteger, nil, nil)
	mode := p.Define([]string{"--mode M", "fast|slow"},
		optparse.CoerceList, []string{"fast", "slow"}, nil)

	matches, rest, err := p.ParseBang([]string{"-v", "--count", "0x10", "--mode", "fa", "file.txt"})
	if err != nil {
		// err is a *optparse.ParseError with MRI-exact Error()/Class()/Args.
		fmt.Println(err) // e.g. "invalid argument: --count xx"
		return
	}

	// Dispatch: in rbgo each match's SpecIndex selects the Ruby block to call.
	for _, m := range matches {
		switch m.SpecIndex {
		case verbose:
			fmt.Println("verbose:", m.Value) // true
		case count:
			fmt.Println("count:", m.Value) // int64(16)
		case mode:
			fmt.Println("mode:", m.Value) // "fast"
		}
	}
	fmt.Println("operands:", rest) // ["file.txt"]
	fmt.Print(p.Help())
}
```

`MakeSpec` builds an `optparse.Spec` directly from `on(...)` strings; `On` /
`OnHead` register a `Spec` and return its index; `Order`, `Permute` and `Getopts`
cover the remaining MRI parse entry points.

## Tests &amp; coverage

The test suite is **differential**: every case is parsed by both this engine and
the same pure-Ruby `OptionParser` that go-embedded-ruby ships (extracted into
`testdata/optionparser.rb`), and the serialized result — matches, coerced values,
leftovers, or the error class/reason/args/message — is compared byte-for-byte. The
Ruby oracle self-skips where `ruby` is absent (and binds `$stdout` to binary so
Windows never injects CRLF); a parallel set of deterministic, ruby-free tests
locks the same expectations and reaches **100% coverage** on its own, so the
no-ruby and qemu CI lanes stay green.

```sh
go test ./...                                   # full suite (uses ruby if present)
go test -race -coverprofile=cover.out ./...     # race + coverage
go tool cover -func=cover.out | tail -1         # total: 100.0%
```

## License

BSD-3-Clause. See [LICENSE](LICENSE).
