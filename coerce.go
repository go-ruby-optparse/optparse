package optparse

import (
	"math/big"
	"strconv"
	"strings"
)

// invalidArg is the sentinel a converter returns when the value is unacceptable.
// convert() turns it into a properly-tokenized InvalidArgument (split_invalid),
// so the converter need not know the original option token.
type invalidArg struct{}

func (invalidArg) Error() string { return "invalid argument" }

// ambiguousArg is the sentinel for an ambiguous candidate-list match.
type ambiguousArg struct{}

func (ambiguousArg) Error() string { return "ambiguous argument" }

// converterFor returns the converter for a Spec's Coerce/List configuration, or
// nil for no coercion (CoerceNone).
func converterFor(s Spec) converter {
	switch s.Coerce {
	case CoerceNone:
		return nil
	case CoerceString:
		return convString
	case CoerceInteger:
		return convInteger
	case CoerceFloat:
		return convFloat
	case CoerceArray:
		return convArray
	case CoerceList:
		return listConverter(s.List, s.Values)
	default:
		// Unknown coercion name behaves like String (the Object/NilClass/String
		// accept defaults, which return the value unchanged).
		return convString
	}
}

// convString returns the value unchanged (MRI's String accept), passing nil
// (an absent optional argument) through.
func convString(s string, present bool) (any, error) {
	if !present {
		return nil, nil
	}
	return s, nil
}

// convInteger parses MRI's Integer accept: decimal, and 0x/0b/0o prefixes, with
// "_" digit separators, signed.
func convInteger(s string, present bool) (any, error) {
	if !present {
		return nil, invalidArg{}
	}
	clean := strings.ReplaceAll(s, "_", "")
	switch {
	case reHex.MatchString(s):
		return parseBase(clean, 16)
	case reBin.MatchString(s):
		return parseBase(clean, 2)
	case reOct.MatchString(s):
		return parseBase(clean, 8)
	case reDec.MatchString(s):
		return parseBase(clean, 10)
	default:
		return nil, invalidArg{}
	}
}

// parseBase parses a (possibly 0x/0b/0o-prefixed, signed) integer literal in the
// given base, returning an int64 when it fits and a *big.Int otherwise so very
// large literals round-trip like Ruby's arbitrary-precision Integer.
func parseBase(s string, base int) (any, error) {
	sign := ""
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		if s[0] == '-' {
			sign = "-"
		}
		s = s[1:]
	}
	if base != 10 && len(s) >= 2 && s[0] == '0' {
		s = s[2:]
	}
	if v, err := strconv.ParseInt(sign+s, base, 64); err == nil {
		return v, nil
	}
	if bi, ok := new(big.Int).SetString(sign+s, base); ok {
		return bi, nil
	}
	return nil, invalidArg{}
}

// convFloat parses MRI's Float accept.
func convFloat(s string, present bool) (any, error) {
	if !present || !reFlt.MatchString(s) {
		return nil, invalidArg{}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, invalidArg{}
	}
	return f, nil
}

// convArray splits on "," like MRI's Array accept; an absent value passes through.
func convArray(s string, present bool) (any, error) {
	if !present {
		return nil, nil
	}
	return strings.Split(s, ","), nil
}

// listConverter builds MRI's candidate-list converter: exact match wins; else a
// unique start_with prefix; empty → InvalidArgument; many → AmbiguousArgument.
// When values is non-nil it returns the parallel value; otherwise the candidate.
func listConverter(keys, values []string) converter {
	return func(s string, present bool) (any, error) {
		if !present {
			return nil, invalidArg{}
		}
		var cands []int
		for i, k := range keys {
			if k == s {
				cands = []int{i}
				break
			}
		}
		if len(cands) == 0 {
			for i, k := range keys {
				if strings.HasPrefix(k, s) {
					cands = append(cands, i)
				}
			}
		}
		switch len(cands) {
		case 1:
			i := cands[0]
			if values != nil {
				return values[i], nil
			}
			return keys[i], nil
		case 0:
			return nil, invalidArg{}
		default:
			return nil, ambiguousArg{}
		}
	}
}
