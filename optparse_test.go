package optparse

import (
	"math/big"
	"reflect"
	"testing"
)

// These deterministic, ruby-free tests lock the same expectations the
// differential corpus verifies against MRI, plus every error/help/edge branch.
// They reach 100% coverage on their own so the no-ruby and qemu CI lanes pass.

func mustParse(t *testing.T, p *Parser, argv []string) ([]Match, []string) {
	t.Helper()
	m, rest, err := p.ParseBang(argv)
	if err != nil {
		t.Fatalf("ParseBang(%v): %v", argv, err)
	}
	return m, rest
}

func TestFlagsLongShortBoth(t *testing.T) {
	p := New()
	p.On(MakeSpec([]string{"-v", "--verbose"}, "", nil, nil))
	m, rest := mustParse(t, p, []string{"-v"})
	if len(m) != 1 || m[0].SpecIndex != 0 || m[0].Value != true {
		t.Fatalf("short: %+v", m)
	}
	if len(rest) != 0 {
		t.Fatalf("rest: %v", rest)
	}
	m, _ = mustParse(t, p, []string{"--verbose"})
	if m[0].Value != true {
		t.Fatalf("long: %+v", m)
	}
}

func TestRequiredArgForms(t *testing.T) {
	for _, argv := range [][]string{
		{"--name", "bob"}, {"--name=bob"}, {"-n", "bob"}, {"-nbob"}, {"-n=bob"},
	} {
		p := New()
		p.On(MakeSpec([]string{"-n", "--name", "VALUE"}, "", nil, nil))
		// arg style comes from the flag string; redefine explicitly:
		p = New()
		p.On(MakeSpec([]string{"--name VALUE"}, "", nil, nil))
		p.On(MakeSpec([]string{"-n VALUE"}, "", nil, nil))
		m, _ := mustParse(t, p, argv)
		if m[0].Value != "bob" {
			t.Fatalf("argv=%v got %+v", argv, m)
		}
	}
}

func TestMissingArgument(t *testing.T) {
	p := New()
	p.On(MakeSpec([]string{"--name VALUE"}, "", nil, nil))
	_, _, err := p.ParseBang([]string{"--name"})
	pe := err.(*ParseError)
	if pe.Kind != KindMissingArgument || pe.Error() != "missing argument: --name" {
		t.Fatalf("got %v / %q", pe.Kind, pe.Error())
	}
	if pe.Class() != "OptionParser::MissingArgument" {
		t.Fatalf("class %q", pe.Class())
	}
	// short form too
	p = New()
	p.On(MakeSpec([]string{"-n VALUE"}, "", nil, nil))
	_, _, err = p.ParseBang([]string{"-n"})
	if err.(*ParseError).Error() != "missing argument: -n" {
		t.Fatalf("short missing: %v", err)
	}
}

func TestOptionalArgument(t *testing.T) {
	p := New()
	p.On(MakeSpec([]string{"--opt [VALUE]"}, "", nil, nil))
	p.On(MakeSpec([]string{"-v"}, "", nil, nil))
	m, _ := mustParse(t, p, []string{"--opt", "x"})
	if m[0].Value != "x" {
		t.Fatalf("present: %+v", m)
	}
	m, _ = mustParse(t, p, []string{"--opt"})
	if m[0].Value != nil {
		t.Fatalf("absent end: %+v", m)
	}
	m, _ = mustParse(t, p, []string{"--opt", "-v"})
	if m[0].Value != nil || len(m) != 2 || m[1].SpecIndex != 1 {
		t.Fatalf("absent before opt: %+v", m)
	}
	// short optional glued and bare
	p = New()
	p.On(MakeSpec([]string{"-o [VALUE]"}, "", nil, nil))
	m, _ = mustParse(t, p, []string{"-ox"})
	if m[0].Value != "x" {
		t.Fatalf("short glued: %+v", m)
	}
	m, _ = mustParse(t, p, []string{"-o"})
	if m[0].Value != nil {
		t.Fatalf("short bare: %+v", m)
	}
}

func TestBundledShorts(t *testing.T) {
	p := New()
	p.On(MakeSpec([]string{"-x"}, "", nil, nil))
	p.On(MakeSpec([]string{"-v"}, "", nil, nil))
	p.On(MakeSpec([]string{"-f"}, "", nil, nil))
	m, _ := mustParse(t, p, []string{"-xvf"})
	if len(m) != 3 || m[0].SpecIndex != 0 || m[2].SpecIndex != 2 {
		t.Fatalf("bundle: %+v", m)
	}
	// bundle terminating in arg-taking
	p = New()
	p.On(MakeSpec([]string{"-x"}, "", nil, nil))
	p.On(MakeSpec([]string{"-f VALUE"}, "", nil, nil))
	m, _ = mustParse(t, p, []string{"-xfhello"})
	if m[1].Value != "hello" {
		t.Fatalf("bundle arg: %+v", m)
	}
	// invalid in bundle
	p = New()
	p.On(MakeSpec([]string{"-x"}, "", nil, nil))
	_, _, err := p.ParseBang([]string{"-xz"})
	if err.(*ParseError).Error() != "invalid option: -z" {
		t.Fatalf("bundle invalid: %v", err)
	}
}

func TestNegatable(t *testing.T) {
	p := New()
	p.On(MakeSpec([]string{"--[no-]color"}, "", nil, nil))
	m, _ := mustParse(t, p, []string{"--color"})
	if m[0].Value != true {
		t.Fatalf("color: %+v", m)
	}
	m, _ = mustParse(t, p, []string{"--no-color"})
	if m[0].Value != false {
		t.Fatalf("no-color: %+v", m)
	}
	_, _, err := p.ParseBang([]string{"--no-color=1"})
	if err.(*ParseError).Kind != KindNeedlessArgument {
		t.Fatalf("needless: %v", err)
	}
	if err.(*ParseError).Error() != "needless argument: --no-color=1" {
		t.Fatalf("needless msg: %v", err)
	}
}

func TestAbbreviationAndCompletion(t *testing.T) {
	p := New()
	p.On(MakeSpec([]string{"--verbose"}, "", nil, nil))
	m, _ := mustParse(t, p, []string{"--verb"})
	if m[0].SpecIndex != 0 {
		t.Fatalf("abbrev: %+v", m)
	}
	// ambiguous
	p = New()
	p.On(MakeSpec([]string{"--verbose"}, "", nil, nil))
	p.On(MakeSpec([]string{"--version"}, "", nil, nil))
	_, _, err := p.ParseBang([]string{"--ver"})
	if err.(*ParseError).Kind != KindAmbiguousOption || err.(*ParseError).Error() != "ambiguous option: --ver" {
		t.Fatalf("ambiguous: %v", err)
	}
	// short completes long
	p = New()
	p.On(MakeSpec([]string{"--name VALUE"}, "", nil, nil))
	m, _ = mustParse(t, p, []string{"-n", "bob"})
	if m[0].Value != "bob" {
		t.Fatalf("short->long: %+v", m)
	}
	// ambiguous short completion
	p = New()
	p.On(MakeSpec([]string{"--name"}, "", nil, nil))
	p.On(MakeSpec([]string{"--node"}, "", nil, nil))
	_, _, err = p.ParseBang([]string{"-n"})
	if err.(*ParseError).Kind != KindAmbiguousOption {
		t.Fatalf("ambiguous short: %v", err)
	}
}

func TestCoercionInteger(t *testing.T) {
	cases := map[string]any{
		"42": int64(42), "0xff": int64(255), "0b1010": int64(10),
		"0o17": int64(15), "-42": int64(-42), "1_000": int64(1000),
		"0XfF": int64(255), "+7": int64(7),
	}
	for in, want := range cases {
		p := New()
		p.On(MakeSpec([]string{"--n N"}, CoerceInteger, nil, nil))
		m, _ := mustParse(t, p, []string{"--n", in})
		if m[0].Value != want {
			t.Fatalf("int %q: got %v want %v", in, m[0].Value, want)
		}
	}
	// big int overflow → *big.Int
	p := New()
	p.On(MakeSpec([]string{"--n N"}, CoerceInteger, nil, nil))
	m, _ := mustParse(t, p, []string{"--n", "99999999999999999999999999"})
	bi, ok := m[0].Value.(*big.Int)
	if !ok || bi.String() != "99999999999999999999999999" {
		t.Fatalf("bigint: %v", m[0].Value)
	}
	// big hex overflow
	m, _ = mustParse(t, p, []string{"--n", "0xffffffffffffffffffffff"})
	if _, ok := m[0].Value.(*big.Int); !ok {
		t.Fatalf("bighex: %v", m[0].Value)
	}
	// invalid
	_, _, err := p.ParseBang([]string{"--n", "xx"})
	if err.(*ParseError).Error() != "invalid argument: --n xx" {
		t.Fatalf("int invalid long: %v", err)
	}
	_, _, err = p.ParseBang([]string{"--n=xx"})
	if err.(*ParseError).Error() != "invalid argument: --n=xx" {
		t.Fatalf("int invalid long=: %v", err)
	}
	p = New()
	p.On(MakeSpec([]string{"-n N"}, CoerceInteger, nil, nil))
	_, _, err = p.ParseBang([]string{"-n", "xx"})
	if err.(*ParseError).Error() != "invalid argument: -n" {
		t.Fatalf("int invalid short: %v", err)
	}
}

func TestCoercionFloat(t *testing.T) {
	for in, want := range map[string]float64{"3.14": 3.14, "-2.5e3": -2500, ".5": 0.5, "10": 10} {
		p := New()
		p.On(MakeSpec([]string{"--f F"}, CoerceFloat, nil, nil))
		m, _ := mustParse(t, p, []string{"--f", in})
		if m[0].Value != want {
			t.Fatalf("float %q: got %v want %v", in, m[0].Value, want)
		}
	}
	p := New()
	p.On(MakeSpec([]string{"--f F"}, CoerceFloat, nil, nil))
	if _, _, err := p.ParseBang([]string{"--f", "nope"}); err == nil {
		t.Fatal("float should fail")
	}
	// a regex-passing but ParseFloat-overflowing value
	if _, _, err := p.ParseBang([]string{"--f", "1e99999"}); err == nil {
		t.Fatal("float overflow should fail")
	}
}

func TestCoercionArrayString(t *testing.T) {
	p := New()
	p.On(MakeSpec([]string{"--list L"}, CoerceArray, nil, nil))
	m, _ := mustParse(t, p, []string{"--list", "a,b,c"})
	if !reflect.DeepEqual(m[0].Value, []string{"a", "b", "c"}) {
		t.Fatalf("array: %v", m[0].Value)
	}
	// optional array absent → nil
	p = New()
	p.On(MakeSpec([]string{"--list [L]"}, CoerceArray, nil, nil))
	m, _ = mustParse(t, p, []string{"--list"})
	if m[0].Value != nil {
		t.Fatalf("array absent: %v", m[0].Value)
	}
	// String coercion + absent optional string
	p = New()
	p.On(MakeSpec([]string{"--s [S]"}, CoerceString, nil, nil))
	m, _ = mustParse(t, p, []string{"--s", "hi"})
	if m[0].Value != "hi" {
		t.Fatalf("string: %v", m[0].Value)
	}
	m, _ = mustParse(t, p, []string{"--s"})
	if m[0].Value != nil {
		t.Fatalf("string absent: %v", m[0].Value)
	}
}

func TestCoercionList(t *testing.T) {
	p := New()
	p.On(MakeSpec([]string{"--mode M"}, CoerceList, []string{"fast", "slow"}, nil))
	m, _ := mustParse(t, p, []string{"--mode", "fast"})
	if m[0].Value != "fast" {
		t.Fatalf("list exact: %v", m[0].Value)
	}
	m, _ = mustParse(t, p, []string{"--mode", "fa"})
	if m[0].Value != "fast" {
		t.Fatalf("list prefix: %v", m[0].Value)
	}
	// values mapping
	p = New()
	p.On(MakeSpec([]string{"--mode M"}, CoerceList, []string{"a", "b"}, []string{"AA", "BB"}))
	m, _ = mustParse(t, p, []string{"--mode", "a"})
	if m[0].Value != "AA" {
		t.Fatalf("list values: %v", m[0].Value)
	}
	// ambiguous (downgraded to InvalidArgument by convert)
	p = New()
	p.On(MakeSpec([]string{"--mode M"}, CoerceList, []string{"fast", "fail"}, nil))
	_, _, err := p.ParseBang([]string{"--mode", "fa"})
	if err.(*ParseError).Kind != KindInvalidArgument {
		t.Fatalf("list ambiguous: %v", err)
	}
	// unknown
	p = New()
	p.On(MakeSpec([]string{"--mode M"}, CoerceList, []string{"fast", "slow"}, nil))
	if _, _, err := p.ParseBang([]string{"--mode", "nope"}); err == nil {
		t.Fatal("list unknown should fail")
	}
	// optional list absent (present=false → InvalidArgument path covered)
	p = New()
	p.On(MakeSpec([]string{"--mode [M]"}, CoerceList, []string{"fast"}, nil))
	if _, _, err := p.ParseBang([]string{"--mode"}); err == nil {
		t.Fatal("optional list absent should fail like MRI Integer-style")
	}
}

func TestModesAndTerminators(t *testing.T) {
	p := New()
	p.On(MakeSpec([]string{"-v"}, "", nil, nil))
	_, rest := mustParse(t, p, []string{"a", "-v", "b"})
	if !reflect.DeepEqual(rest, []string{"a", "b"}) {
		t.Fatalf("permute rest: %v", rest)
	}
	// order! stops at first non-option
	p = New()
	p.On(MakeSpec([]string{"-v"}, "", nil, nil))
	m, rest, err := p.Order([]string{"a", "-v", "b"}, nil)
	if err != nil || len(m) != 0 || !reflect.DeepEqual(rest, []string{"a", "-v", "b"}) {
		t.Fatalf("order: %+v / %v", m, rest)
	}
	// order! with options first
	m, rest, _ = p.Order([]string{"-v", "a", "b"}, nil)
	if len(m) != 1 || !reflect.DeepEqual(rest, []string{"a", "b"}) {
		t.Fatalf("order opts first: %+v / %v", m, rest)
	}
	// order! with nonopt callback
	var seen []string
	p = New()
	p.On(MakeSpec([]string{"-v"}, "", nil, nil))
	m, rest, _ = p.Order([]string{"a", "-v", "b"}, func(s string) { seen = append(seen, s) })
	if !reflect.DeepEqual(seen, []string{"a", "b"}) || len(m) != 1 || len(rest) != 0 {
		t.Fatalf("order cb: seen=%v m=%+v rest=%v", seen, m, rest)
	}
	// Permute alias
	m, rest, _ = p.Permute([]string{"-v", "x"})
	if len(m) != 1 || !reflect.DeepEqual(rest, []string{"x"}) {
		t.Fatalf("permute alias: %+v / %v", m, rest)
	}
	// -- terminator
	p = New()
	p.On(MakeSpec([]string{"-v"}, "", nil, nil))
	_, rest = mustParse(t, p, []string{"-v", "--", "-x", "a"})
	if !reflect.DeepEqual(rest, []string{"-x", "a"}) {
		t.Fatalf("terminator: %v", rest)
	}
	// "-" operand
	_, rest = mustParse(t, p, []string{"-", "-v"})
	if !reflect.DeepEqual(rest, []string{"-"}) {
		t.Fatalf("dash operand: %v", rest)
	}
	// order! "-" stops
	m, rest, _ = p.Order([]string{"-", "-v"}, nil)
	if len(m) != 0 || !reflect.DeepEqual(rest, []string{"-", "-v"}) {
		t.Fatalf("order dash: %+v / %v", m, rest)
	}
}

func TestInvalidOption(t *testing.T) {
	p := New()
	p.On(MakeSpec([]string{"-v"}, "", nil, nil))
	_, _, err := p.ParseBang([]string{"-x"})
	if err.(*ParseError).Error() != "invalid option: -x" {
		t.Fatalf("short: %v", err)
	}
	if err.(*ParseError).Class() != "OptionParser::InvalidOption" {
		t.Fatalf("class: %v", err.(*ParseError).Class())
	}
	_, _, err = p.ParseBang([]string{"--bogus"})
	if err.(*ParseError).Error() != "invalid option: --bogus" {
		t.Fatalf("long: %v", err)
	}
}

func TestRecover(t *testing.T) {
	e := newErr(KindMissingArgument, "--name")
	got := e.Recover([]string{"rest1", "rest2"})
	if !reflect.DeepEqual(got, []string{"--name", "rest1", "rest2"}) {
		t.Fatalf("recover: %v", got)
	}
}

func TestParseErrorReasonAndBase(t *testing.T) {
	e := &ParseError{Kind: KindParseError, Args: []string{"x"}}
	if e.Reason() != "parse error" || e.Error() != "parse error: x" {
		t.Fatalf("base: %q / %q", e.Reason(), e.Error())
	}
	a := &ParseError{Kind: KindAmbiguousArgument, Args: []string{"--m", "fa"}}
	if a.Class() != "OptionParser::AmbiguousArgument" || a.Error() != "ambiguous argument: --m fa" {
		t.Fatalf("ambig arg: %q / %q", a.Class(), a.Error())
	}
}

func TestGetopts(t *testing.T) {
	p := New()
	res, err := p.Getopts([]string{"-a", "-b"}, "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if !res["a"].Flag || !res["b"].Flag {
		t.Fatalf("flags: %+v", res)
	}
	p = New()
	res, _ = p.Getopts([]string{"--name=bob"}, "name:")
	if res["name"].Str == nil || *res["name"].Str != "bob" {
		t.Fatalf("name: %+v", res["name"])
	}
	p = New()
	res, _ = p.Getopts([]string{}, "a", "name:")
	if res["a"].Flag || res["name"].Str != nil {
		t.Fatalf("absent: %+v", res)
	}
	// arg-taking short
	p = New()
	res, _ = p.Getopts([]string{"-x", "v"}, "x:")
	if *res["x"].Str != "v" {
		t.Fatalf("short arg: %+v", res["x"])
	}
	// error propagation
	p = New()
	_, err = p.Getopts([]string{"-z"}, "a")
	if err == nil {
		t.Fatal("getopts should error on -z")
	}
}

func TestHelpAndSummarize(t *testing.T) {
	p := New()
	p.Banner = "Usage: tool [opts]"
	p.On(MakeSpec([]string{"-v", "--verbose", "run verbosely"}, "", nil, nil))
	p.On(MakeSpec([]string{"--name NAME", "the name"}, "", nil, nil))
	p.Separator("")
	p.Separator("More:")
	p.OnHead(MakeSpec([]string{"-h", "--help", "show help"}, "", nil, nil))
	want := "Usage: tool [opts]\n" +
		"    -h, --help                       show help\n" +
		"    -v, --verbose                    run verbosely\n" +
		"        --name NAME                  the name\n" +
		"\n" +
		"More:\n"
	if got := p.Help(); got != want {
		t.Fatalf("help:\n got=%q\nwant=%q", got, want)
	}
	// Summarize returns the same lines minus banner.
	sum := p.Summarize()
	if len(sum) == 0 || sum[len(sum)-1] != "More:\n" {
		t.Fatalf("summarize: %q", sum)
	}
}

func TestHelpDefaults(t *testing.T) {
	// Default banner uses ProgramName (default "optparse"); long-only and
	// short-only switches, plus multi-line descriptions and the wide-left wrap.
	p := New()
	p.On(MakeSpec([]string{"-v", "first line", "second line"}, "", nil, nil))
	p.On(MakeSpec([]string{"--averylongoptionname-thatexceedswidth VALUE", "wrapped"}, "", nil, nil))
	p.On(MakeSpec([]string{"--[no-]flag"}, "", nil, nil))
	got := p.Help()
	wantBanner := "Usage: optparse [options]\n"
	if got[:len(wantBanner)] != wantBanner {
		t.Fatalf("default banner: %q", got[:len(wantBanner)])
	}
	// custom program name
	p.ProgramName = "mytool"
	if got := p.Help(); got[:26] != "Usage: mytool [options]\n  " {
		t.Fatalf("prog name banner: %q", got[:26])
	}
	// custom summary width/indent
	p = New()
	p.SummaryWidth = 10
	p.SummaryIndent = ">>"
	p.On(MakeSpec([]string{"-v", "desc"}, "", nil, nil))
	if p.Help() == "" {
		t.Fatal("empty help")
	}
}

func TestMakeSpecParsing(t *testing.T) {
	// no-arg long, required, optional, =-style descriptor, and pure desc strings.
	s := MakeSpec([]string{"--name=VALUE", "a description", "another"}, "", nil, nil)
	if s.ArgStyle != ArgRequired || s.ArgName != "VALUE" {
		t.Fatalf("=desc: %+v", s)
	}
	if len(s.Desc) != 2 {
		t.Fatalf("desc count: %v", s.Desc)
	}
	s = MakeSpec([]string{"--opt[VAL]"}, "", nil, nil)
	if s.ArgStyle != ArgOptional || s.ArgName != "VAL" {
		t.Fatalf("opt: %+v", s)
	}
	s = MakeSpec([]string{"--bare"}, "", nil, nil)
	if s.ArgStyle != ArgNone || s.ArgName != "" {
		t.Fatalf("bare: %+v", s)
	}
	s = MakeSpec([]string{"-x ARG"}, "", nil, nil)
	if s.ArgStyle != ArgRequired || s.ArgName != "ARG" {
		t.Fatalf("short arg: %+v", s)
	}
	s = MakeSpec([]string{"--[no-]color VALUE"}, "", nil, nil)
	if !s.Negatable || s.ArgStyle != ArgRequired || s.ArgName != "VALUE" {
		t.Fatalf("no- with arg: %+v", s)
	}
	// unknown coerce name falls back to String (no error, value passes through)
	p := New()
	p.On(MakeSpec([]string{"--x V"}, "Weird", nil, nil))
	m, _ := mustParse(t, p, []string{"--x", "hi"})
	if m[0].Value != "hi" {
		t.Fatalf("unknown coerce: %v", m[0].Value)
	}
}

func TestSerializeGoOther(t *testing.T) {
	// The "other" branch of the oracle serializer (uncommon value types).
	if got := serializeGo(struct{ A int }{1}); got.T != "other" {
		t.Fatalf("other: %+v", got)
	}
	if got := serializeGo(int64(5)); got.T != "int" || got.V != "5" {
		t.Fatalf("int: %+v", got)
	}
	if got := serializeGo(big.NewInt(7)); got.T != "int" {
		t.Fatalf("bigint: %+v", got)
	}
	if got := serializeGo(nil); got.T != "nil" {
		t.Fatalf("nil: %+v", got)
	}
	if got := serializeGo(false); got.V != false {
		t.Fatalf("false: %+v", got)
	}
}
