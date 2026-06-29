package optparse

import (
	"regexp"
	"strings"
)

// Match is one resolved option occurrence. The consumer dispatches the Ruby block
// registered for SpecIndex, passing Value.
type Match struct {
	// SpecIndex is the index (registration order) of the matched Spec.
	SpecIndex int
	// Value is the coerced argument. For a flag it is a bool: true, or false for
	// the negated form of a "--[no-]flag". For an optional argument that was not
	// supplied it is nil. Otherwise it is the coerced argument (string, int64,
	// *big.Int, float64, []string, or a candidate-list value).
	Value any
	// Raw is the argument string as it appeared (before coercion), or "" for a flag.
	Raw string
}

// Parser holds the registered option specs plus the help/summary layout. The zero
// value is not ready; use New.
type Parser struct {
	specs []Spec

	// list preserves declaration order for help; each item is a switch or separator.
	list []listItem

	switchesLong  map[string]*switchDef
	switchesShort map[string]*switchDef

	// Banner is the first help line; empty means "Usage: <ProgramName> [options]".
	Banner string
	// ProgramName is used in the default banner; empty means "optparse".
	ProgramName string
	// SummaryWidth is the option-column width (MRI default 32).
	SummaryWidth int
	// SummaryIndent is the left indent (MRI default "    ").
	SummaryIndent string

	matches []Match
}

type listItem struct {
	isSep bool
	sep   string
	sw    *switchDef
}

// New returns a Parser with MRI's default summary layout.
func New() *Parser {
	return &Parser{
		switchesLong:  map[string]*switchDef{},
		switchesShort: map[string]*switchDef{},
		SummaryWidth:  32,
		SummaryIndent: "    ",
	}
}

// Separator appends a literal line to the help output (MRI #separator).
func (p *Parser) Separator(text string) *Parser {
	p.list = append(p.list, listItem{isSep: true, sep: text})
	return p
}

// On registers a Spec (MRI #on / #on_tail) and returns the assigned spec index.
func (p *Parser) On(spec Spec) int {
	idx := len(p.specs)
	p.specs = append(p.specs, spec)
	sw := &switchDef{
		short:     spec.Short,
		long:      spec.Long,
		arg:       spec.ArgName,
		mandatory: spec.ArgStyle == ArgRequired,
		optional:  spec.ArgStyle == ArgOptional,
		negatable: spec.Negatable,
		conv:      converterFor(spec),
		desc:      spec.Desc,
		specIndex: idx,
	}
	p.register(sw, false)
	return idx
}

// OnHead registers a Spec at the head of the help list (MRI #on_head).
func (p *Parser) OnHead(spec Spec) int {
	idx := len(p.specs)
	p.specs = append(p.specs, spec)
	sw := &switchDef{
		short:     spec.Short,
		long:      spec.Long,
		arg:       spec.ArgName,
		mandatory: spec.ArgStyle == ArgRequired,
		optional:  spec.ArgStyle == ArgOptional,
		negatable: spec.Negatable,
		conv:      converterFor(spec),
		desc:      spec.Desc,
		specIndex: idx,
	}
	p.register(sw, true)
	return idx
}

func (p *Parser) register(sw *switchDef, head bool) {
	item := listItem{sw: sw}
	if head {
		p.list = append([]listItem{item}, p.list...)
	} else {
		p.list = append(p.list, item)
	}
	for _, s := range sw.short {
		p.switchesShort[s] = sw
	}
	for _, l := range sw.long {
		p.switchesLong[l] = sw
	}
	if sw.negatable {
		for _, l := range sw.long {
			p.switchesLong[strings.Replace(l, "--", "--no-", 1)] = sw
		}
	}
}

var reShortHead = regexp.MustCompile(`\A-.`)

// ParseBang consumes argv as MRI #parse! (== #permute!): options anywhere, with
// operands gathered and returned as rest. It returns the ordered matches (for the
// consumer to dispatch the blocks), the leftover operands, and the first
// *ParseError encountered.
func (p *Parser) ParseBang(argv []string) (matches []Match, rest []string, err error) {
	return p.parseInOrder(argv, true, nil)
}

// Permute is MRI #permute! (the same as ParseBang).
func (p *Parser) Permute(argv []string) (matches []Match, rest []string, err error) {
	return p.parseInOrder(argv, true, nil)
}

// Order is MRI #order!: parsing stops at the first non-option, which (with all the
// remaining argv) is returned as rest. If nonOpt is non-nil it is instead called
// for each leading non-option token and parsing continues (MRI's block form).
func (p *Parser) Order(argv []string, nonOpt func(string)) (matches []Match, rest []string, err error) {
	return p.parseInOrder(argv, false, nonOpt)
}

func (p *Parser) parseInOrder(argv []string, permute bool, nonopt func(string)) ([]Match, []string, error) {
	p.matches = nil
	work := append([]string(nil), argv...)
	operands := []string{}
	for len(work) > 0 {
		arg := work[0]
		switch {
		case arg == "--":
			work = work[1:]
			operands = append(operands, work...)
			work = nil
		case arg == "-" || !reShortHead.MatchString(arg):
			if permute {
				operands = append(operands, work[0])
				work = work[1:]
			} else if nonopt != nil {
				nonopt(work[0])
				work = work[1:]
			} else {
				return p.matches, joinRest(work, operands), nil
			}
		case strings.HasPrefix(arg, "--"):
			work = work[1:]
			var err error
			if work, err = p.parseLong(arg, work); err != nil {
				return p.matches, joinRest(work, operands), err
			}
		default:
			work = work[1:]
			var err error
			if work, err = p.parseShort(arg, work); err != nil {
				return p.matches, joinRest(work, operands), err
			}
		}
	}
	return p.matches, joinRest(work, operands), nil
}

// joinRest concatenates the remaining argv and gathered operands into a single,
// always-non-nil slice (MRI returns an Array even when empty).
func joinRest(work, operands []string) []string {
	rest := make([]string, 0, len(work)+len(operands))
	rest = append(rest, work...)
	rest = append(rest, operands...)
	return rest
}

func (p *Parser) parseLong(arg string, argv []string) ([]string, error) {
	body := arg[2:]
	name, val, hasEq := strings.Cut(body, "=")
	opt := "--" + name
	sw, negated, err := p.lookupLong(opt)
	if err != nil {
		return argv, err
	}
	if sw == nil {
		return argv, newErr(KindInvalidOption, arg)
	}
	var valp *string
	if hasEq {
		v := val
		valp = &v
	}
	return p.consume(sw, opt, arg, valp, hasEq, argv, negated)
}

func (p *Parser) lookupLong(opt string) (*switchDef, bool, error) {
	if sw, ok := p.switchesLong[opt]; ok {
		return sw, p.negatedName(opt, sw), nil
	}
	var cands []string
	for k := range p.switchesLong {
		if strings.HasPrefix(k, opt) {
			cands = append(cands, k)
		}
	}
	if len(cands) == 0 {
		return nil, false, nil
	}
	uniq := map[*switchDef]bool{}
	for _, k := range cands {
		uniq[p.switchesLong[k]] = true
	}
	if len(uniq) == 1 {
		sw := p.switchesLong[cands[0]]
		key := opt
		if len(cands) == 1 {
			key = cands[0]
		}
		return sw, p.negatedName(key, sw), nil
	}
	return nil, false, newErr(KindAmbiguousOption, opt)
}

func (p *Parser) negatedName(key string, sw *switchDef) bool {
	if !sw.negatable || !strings.HasPrefix(key, "--no-") {
		return false
	}
	for _, l := range sw.long {
		if l == key {
			return false
		}
	}
	return true
}

func (p *Parser) parseShort(arg string, argv []string) ([]string, error) {
	chars := []rune(arg[1:])
	i := 0
	for i < len(chars) {
		c := "-" + string(chars[i])
		sw := p.switchesShort[c]
		if sw == nil {
			var err error
			if sw, err = p.completeShort(chars[i]); err != nil {
				return argv, err
			}
		}
		if sw == nil {
			return argv, newErr(KindInvalidOption, c)
		}
		rest := string(chars[i+1:])
		if sw.mandatory || sw.optional {
			var valp *string
			if rest != "" {
				v := rest
				if strings.HasPrefix(rest, "=") {
					v = rest[1:]
				}
				valp = &v
			}
			return p.consume(sw, c, arg, valp, false, argv, false)
		}
		// A plain (no-argument) short flag never errors in consume (the only
		// consume error for a flag is a "=value" form, which a bundled short can
		// never carry), so the returned argv is unchanged and the error is nil.
		argv, _ = p.consume(sw, c, arg, nil, false, argv, false)
		i++
	}
	return argv, nil
}

// completeShort resolves an unregistered short char against the long options by
// first letter (MRI's "-n" → "--name" completion). A tie is AmbiguousOption.
func (p *Parser) completeShort(ch rune) (*switchDef, error) {
	uniq := map[*switchDef]bool{}
	for name, sw := range p.switchesLong {
		if strings.HasPrefix(name, "--no-") {
			isReal := false
			for _, l := range sw.long {
				if l == name {
					isReal = true
				}
			}
			if !isReal {
				continue
			}
		}
		r := []rune(name)
		if len(r) > 2 && r[2] == ch {
			uniq[sw] = true
		}
	}
	if len(uniq) > 1 {
		return nil, newErr(KindAmbiguousOption, "-"+string(ch))
	}
	for sw := range uniq {
		return sw, nil
	}
	return nil, nil
}

func (p *Parser) consume(sw *switchDef, opt, orig string, val *string, hasEq bool, argv []string, negated bool) ([]string, error) {
	switch {
	case sw.mandatory:
		if val == nil {
			if len(argv) == 0 {
				return argv, newErr(KindMissingArgument, opt)
			}
			v := argv[0]
			argv = argv[1:]
			val = &v
		}
		cv, err := p.convert(sw, *val, orig)
		if err != nil {
			return argv, err
		}
		p.emit(sw, cv, *val)
	case sw.optional:
		if val == nil && len(argv) > 0 && !reShortHead.MatchString(argv[0]) {
			v := argv[0]
			argv = argv[1:]
			val = &v
		}
		if val == nil {
			cv, err := p.convertAbsent(sw, orig)
			if err != nil {
				return argv, err
			}
			p.emit(sw, cv, "")
		} else {
			cv, err := p.convert(sw, *val, orig)
			if err != nil {
				return argv, err
			}
			p.emit(sw, cv, *val)
		}
	default:
		if hasEq && val != nil {
			return argv, newErr(KindNeedlessArgument, orig)
		}
		if sw.negatable {
			p.emit(sw, !negated, "")
		} else {
			p.emit(sw, true, "")
		}
	}
	return argv, nil
}

func (p *Parser) emit(sw *switchDef, value any, raw string) {
	p.matches = append(p.matches, Match{SpecIndex: sw.specIndex, Value: value, Raw: raw})
}

// convert applies the switch's converter, translating a sentinel failure into a
// properly-tokenized InvalidArgument/AmbiguousArgument (MRI's split_invalid).
func (p *Parser) convert(sw *switchDef, val, orig string) (any, error) {
	if sw.conv == nil {
		return val, nil
	}
	cv, err := sw.conv(val, true)
	if err != nil {
		return nil, convError(orig, val)
	}
	return cv, nil
}

// convertAbsent runs the converter for an absent optional argument (present=false).
func (p *Parser) convertAbsent(sw *switchDef, orig string) (any, error) {
	if sw.conv == nil {
		return nil, nil
	}
	cv, err := sw.conv("", false)
	if err != nil {
		return nil, convError(orig, "")
	}
	return cv, nil
}

func convError(orig, val string) error {
	// MRI's #convert rescues InvalidArgument (the superclass of AmbiguousArgument)
	// and re-raises a plain InvalidArgument with the split tokens, so a converter's
	// AmbiguousArgument is downgraded to InvalidArgument here. The standalone
	// AmbiguousArgument (KindAmbiguousArgument) is reserved for callers that raise
	// it directly outside the convert path.
	return newErr(KindInvalidArgument, splitInvalid(orig, val)...)
}

var reLongBare = regexp.MustCompile(`\A(--[^\s=]+)\z`)

// splitInvalid reproduces MRI's split_invalid: a bare "--name" original keeps the
// value as a second token; anything else (short, or "--name=val") is single-token.
func splitInvalid(orig, val string) []string {
	if reLongBare.MatchString(orig) {
		return []string{orig, val}
	}
	return []string{orig}
}
