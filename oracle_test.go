package optparse

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os/exec"
	"strings"
	"testing"
)

// The differential oracle drives the *same* pure-Ruby OptionParser that
// go-embedded-ruby ships (testdata/optionparser.rb, extracted from rbgo's
// prelude). Each case is run by both this Go engine and the Ruby reference and
// the serialized result (matches + coerced values + leftovers, or the error
// class/reason/args/message) is compared byte-for-byte. This guarantees rbgo
// byte-compatibility; the deterministic ruby-free tests below independently lock
// the same expectations so the no-ruby / qemu lanes still reach 100% coverage.

// rubyHarness mirrors the Go engine via the reference OptionParser. It self-skips
// when ruby is absent and binds $stdout to binary so Windows CRLF never leaks.
const rubyHarness = `
$stdout.binmode
require File.expand_path('testdata/optionparser.rb')
require 'json'
def serialize(v)
  case v
  when true then {t:"bool", v:true}
  when false then {t:"bool", v:false}
  when nil then {t:"nil"}
  when Integer then {t:"int", v:v.to_s}
  when Float then {t:"float", v:v}
  when Array then {t:"array", v:v}
  when String then {t:"str", v:v}
  else {t:"other", v:v.to_s}
  end
end
def coerce_arg(c, list, values)
  case c
  when "Integer" then Integer
  when "Float" then Float
  when "Array" then Array
  when "String" then String
  when "List"
    if values && !values.empty?
      h = {}; list.each_with_index { |k,i| h[k] = values[i] }; h
    else
      list
    end
  else nil
  end
end
inp = JSON.parse($stdin.read)
op = OptionParser.new
op.banner = inp["banner"] if inp["banner"]
mode = inp["mode"]
begin
  if mode == "getopts"
    res = op.getopts(inp["argv"], *inp["getopts_specs"])
    puts JSON.generate({ok: true, getopts: res})
  else
    matches = []
    (inp["specs"] || []).each_with_index do |sp, idx|
      args = sp["opts"].dup
      conv = coerce_arg(sp["coerce"], sp["list"], sp["values"])
      args << conv if conv
      op.on(*args) { |v| matches << {spec: idx, value: serialize(v)} }
    end
    argv = inp["argv"].dup
    case mode
    when "order" then rest = op.order!(argv)
    when "permute" then rest = op.permute!(argv)
    else rest = op.parse!(argv)
    end
    if inp["help"]
      puts JSON.generate({ok: true, help: op.help})
    else
      puts JSON.generate({ok: true, matches: matches, rest: rest})
    end
  end
rescue OptionParser::ParseError => e
  puts JSON.generate({ok: false, klass: e.class.name, reason: e.reason, args: e.args, message: e.message})
end
`

type oracleSpec struct {
	Opts   []string `json:"opts"`
	Coerce string   `json:"coerce,omitempty"`
	List   []string `json:"list,omitempty"`
	Values []string `json:"values,omitempty"`
}

type oracleInput struct {
	Mode         string       `json:"mode"`
	Specs        []oracleSpec `json:"specs,omitempty"`
	Argv         []string     `json:"argv"`
	Banner       string       `json:"banner,omitempty"`
	Help         bool         `json:"help,omitempty"`
	GetoptsSpecs []string     `json:"getopts_specs,omitempty"`
}

type serValue struct {
	T string `json:"t"`
	V any    `json:"v,omitempty"`
}

type oracleOutput struct {
	OK      bool `json:"ok"`
	Matches []struct {
		Spec  int      `json:"spec"`
		Value serValue `json:"value"`
	} `json:"matches"`
	Rest    []string                   `json:"rest"`
	Help    string                     `json:"help"`
	Getopts map[string]json.RawMessage `json:"getopts"`
	Klass   string                     `json:"klass"`
	Reason  string                     `json:"reason"`
	Args    []string                   `json:"args"`
	Message string                     `json:"message"`
}

func runRuby(t *testing.T, in oracleInput) oracleOutput {
	t.Helper()
	if _, err := exec.LookPath("ruby"); err != nil {
		t.Skip("ruby not on PATH; skipping differential test")
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cmd := exec.Command("ruby", "-e", rubyHarness)
	cmd.Stdin = strings.NewReader(string(data))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ruby: %v\n%s", err, out)
	}
	var o oracleOutput
	if err := json.Unmarshal(out, &o); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	return o
}

// serializeGo renders a Go Match value the same way the Ruby harness serializes.
func serializeGo(v any) serValue {
	switch x := v.(type) {
	case bool:
		return serValue{T: "bool", V: x}
	case nil:
		return serValue{T: "nil"}
	case int64:
		return serValue{T: "int", V: fmt.Sprintf("%d", x)}
	case *big.Int:
		return serValue{T: "int", V: x.String()}
	case float64:
		return serValue{T: "float", V: x}
	case []string:
		arr := make([]any, len(x))
		for i, s := range x {
			arr[i] = s
		}
		return serValue{T: "array", V: arr}
	case string:
		return serValue{T: "str", V: x}
	default:
		return serValue{T: "other", V: fmt.Sprintf("%v", x)}
	}
}

// buildGo registers the input's specs on a fresh Parser and parses argv with the
// requested mode, returning the same shape the oracle compares.
func buildGo(t *testing.T, in oracleInput) oracleOutput {
	t.Helper()
	p := New()
	p.ProgramName = "optparse"
	if in.Banner != "" {
		p.Banner = in.Banner
	}
	if in.Mode == "getopts" {
		res, err := p.Getopts(in.Argv, in.GetoptsSpecs...)
		var o oracleOutput
		if err != nil {
			pe := err.(*ParseError)
			return oracleOutput{Klass: pe.Class(), Reason: pe.Reason(), Args: pe.Args, Message: pe.Error()}
		}
		o.OK = true
		o.Getopts = map[string]json.RawMessage{}
		for name, r := range res {
			var raw json.RawMessage
			switch {
			case !r.TakesArg:
				raw, _ = json.Marshal(r.Flag)
			case r.Str == nil:
				raw = json.RawMessage("null")
			default:
				raw, _ = json.Marshal(*r.Str)
			}
			o.Getopts[name] = raw
		}
		return o
	}
	for _, s := range in.Specs {
		p.Define(s.Opts, s.Coerce, s.List, s.Values)
	}
	var matches []Match
	var rest []string
	var err error
	switch in.Mode {
	case "order":
		matches, rest, err = p.Order(in.Argv, nil)
	case "permute":
		matches, rest, err = p.Permute(in.Argv)
	default:
		matches, rest, err = p.ParseBang(in.Argv)
	}
	if err != nil {
		pe := err.(*ParseError)
		return oracleOutput{Klass: pe.Class(), Reason: pe.Reason(), Args: pe.Args, Message: pe.Error()}
	}
	if in.Help {
		// The Ruby harness emits only the help text in this mode (no matches/rest).
		return oracleOutput{OK: true, Help: p.Help()}
	}
	o := oracleOutput{OK: true, Rest: rest}
	for _, m := range matches {
		o.Matches = append(o.Matches, struct {
			Spec  int      `json:"spec"`
			Value serValue `json:"value"`
		}{Spec: m.SpecIndex, Value: serializeGo(m.Value)})
	}
	return o
}

func canon(t *testing.T, o oracleOutput) string {
	t.Helper()
	// Normalize: drop the OK flag (we only compare meaningful fields) and re-encode
	// canonically so map ordering / nil-vs-empty don't cause spurious diffs.
	type norm struct {
		Matches []struct {
			Spec int
			T    string
			V    string
		}
		Rest    []string
		Help    string
		Getopts map[string]string
		Klass   string
		Reason  string
		Args    []string
		Message string
	}
	var n norm
	for _, m := range o.Matches {
		n.Matches = append(n.Matches, struct {
			Spec int
			T    string
			V    string
		}{m.Spec, m.Value.T, fmt.Sprintf("%v", m.Value.V)})
	}
	n.Rest = o.Rest
	n.Help = o.Help
	if o.Getopts != nil {
		n.Getopts = map[string]string{}
		for k, v := range o.Getopts {
			n.Getopts[k] = string(v)
		}
	}
	n.Klass, n.Reason, n.Args, n.Message = o.Klass, o.Reason, o.Args, o.Message
	b, _ := json.Marshal(n)
	return string(b)
}

func TestDifferentialAgainstReference(t *testing.T) {
	for i, c := range corpus {
		c := c
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			want := runRuby(t, c) // skips if no ruby
			got := buildGo(t, c)
			if g, w := canon(t, got), canon(t, want); g != w {
				t.Errorf("MISMATCH input=%+v\n got=%s\nwant=%s", c, g, w)
			}
		})
	}
}
