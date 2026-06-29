package optparse

import "regexp"

var reGetopts = regexp.MustCompile(`([a-zA-Z0-9][a-zA-Z0-9_-]*)(:)?`)

// GetoptsResult is the value of one getopts entry: a boolean flag, or a string
// argument (nil Str + TakesArg means the arg-taking option was absent).
type GetoptsResult struct {
	// TakesArg is true for "name:" specs (the option takes an argument).
	TakesArg bool
	// Flag is the boolean value for a no-argument option (false if absent).
	Flag bool
	// Str is the supplied argument for an arg-taking option, or nil if absent.
	Str *string
}

// Getopts implements MRI #getopts: it parses each single-letter/word from the
// spec strings (a trailing ":" means the option takes an argument), registers the
// corresponding switches, parses argv with ParseBang, and returns a map from
// option name to its result. It mirrors the pure-Ruby OptionParser#getopts,
// including its scan grammar (so "ab" is one name "ab", not two flags).
//
// On a parse error the partially-built result and the error are returned.
func (p *Parser) Getopts(argv []string, specs ...string) (map[string]*GetoptsResult, error) {
	result := map[string]*GetoptsResult{}
	type binding struct {
		name     string
		takesArg bool
	}
	var bindings []binding
	for _, spec := range specs {
		for _, m := range reGetopts.FindAllStringSubmatch(spec, -1) {
			name := m[1]
			takesArg := m[2] == ":"
			flag := "-" + name
			if len(name) != 1 {
				flag = "--" + name
			}
			if takesArg {
				result[name] = &GetoptsResult{TakesArg: true}
				p.Define([]string{flag + " VALUE"}, CoerceNone, nil, nil)
			} else {
				result[name] = &GetoptsResult{}
				p.Define([]string{flag}, CoerceNone, nil, nil)
			}
			bindings = append(bindings, binding{name, takesArg})
		}
	}
	matches, _, err := p.ParseBang(argv)
	if err != nil {
		return result, err
	}
	// The Define calls above were appended in binding order, so spec index N maps
	// to bindings[N - base], where base is the first index we registered.
	base := len(p.specs) - len(bindings)
	for _, mt := range matches {
		b := bindings[mt.SpecIndex-base]
		r := result[b.name]
		if b.takesArg {
			s, _ := mt.Value.(string)
			v := s
			r.Str = &v
		} else {
			r.Flag = true
		}
	}
	return result, nil
}
