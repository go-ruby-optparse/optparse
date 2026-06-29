package optparse

import "regexp"

var (
	reNoLong = regexp.MustCompile(`\A--\[no-\]([^\s=]+)(.*)?\z`)
	reLong   = regexp.MustCompile(`\A--([^\s=\[]+)(.*)?\z`)
	reShort  = regexp.MustCompile(`\A-(.)(.*)?\z`)
	reArgLb  = regexp.MustCompile(`\A\[(.*)\]\z`)
	reArgPfx = regexp.MustCompile(`\A[=\s]+`)
)

// MakeSpec builds a Spec from the string arguments of an MRI `on(...)` call (the
// option flag strings such as "-v", "--verbose", "--name VALUE", "--[no-]color",
// "--name [VALUE]", and any trailing description strings). The optional coerce
// argument supplies a type/list converter that MRI would pass as a Class, Array,
// or Hash positional; pass "" for none.
//
// For a candidate list, set coerce to CoerceList and provide list (and optionally
// values); for a built-in type pass CoerceInteger/CoerceFloat/CoerceArray/
// CoerceString. rbgo maps the Ruby `on(*args)` positional arguments onto these.
func MakeSpec(opts []string, coerce string, list, values []string) Spec {
	s := Spec{Coerce: coerce, List: list, Values: values}
	for _, o := range opts {
		switch {
		case reNoLong.MatchString(o):
			m := reNoLong.FindStringSubmatch(o)
			s.Long = append(s.Long, "--"+m[1])
			s.Negatable = true
			if m[2] != "" {
				applyArgSpec(&s, m[2])
			}
		case reLong.MatchString(o):
			m := reLong.FindStringSubmatch(o)
			s.Long = append(s.Long, "--"+m[1])
			if m[2] != "" {
				applyArgSpec(&s, m[2])
			}
		case reShort.MatchString(o):
			m := reShort.FindStringSubmatch(o)
			s.Short = append(s.Short, "-"+m[1])
			if m[2] != "" {
				applyArgSpec(&s, m[2])
			}
		default:
			s.Desc = append(s.Desc, o)
		}
	}
	return s
}

// applyArgSpec parses the trailing argument descriptor of a flag string (MRI's
// parse_argspec): "[VAL]" → optional, "VAL" → required, "" → none.
func applyArgSpec(s *Spec, rest string) {
	rest = reArgPfx.ReplaceAllString(rest, "")
	switch {
	case reArgLb.MatchString(rest):
		s.ArgStyle = ArgOptional
		s.ArgName = reArgLb.FindStringSubmatch(rest)[1]
	case rest != "":
		s.ArgStyle = ArgRequired
		s.ArgName = rest
	default:
		s.ArgStyle = ArgNone
		s.ArgName = ""
	}
}

// Define registers an option from raw MRI `on(...)` flag/description strings,
// returning the assigned spec index. It is sugar over MakeSpec + On.
func (p *Parser) Define(opts []string, coerce string, list, values []string) int {
	return p.On(MakeSpec(opts, coerce, list, values))
}
