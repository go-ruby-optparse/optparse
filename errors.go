// Package optparse is a pure-Go (no cgo) reimplementation of the deterministic,
// interpreter-independent core of Ruby's OptionParser (stdlib `optparse`).
//
// It models option *specifications* (the `on(...)` definitions), tokenizes an
// argv, performs long/short/abbreviation matching, handles `=`/bundled/`--[no-]`/
// optional-argument forms, coerces values (Integer/Float/Array/custom lists), and
// reports the full OptionParser::ParseError taxonomy with MRI-exact
// message/reason/args/recover. It is the parsing engine; the per-match Ruby blocks
// registered by `on(...)` are *not* run here — Parse returns the ordered matches
// and coerced values, and the consumer (e.g. rbgo) dispatches the blocks.
//
// The behavior mirrors MRI 4.0.5 / go-embedded-ruby's pure-Ruby OptionParser
// byte-for-byte across a broad differential corpus.
package optparse

import "strings"

// ErrorKind identifies which node of the OptionParser::ParseError tree an error
// belongs to. It maps 1:1 onto MRI's exception classes.
type ErrorKind int

const (
	// KindParseError is the base OptionParser::ParseError.
	KindParseError ErrorKind = iota
	// KindInvalidOption is OptionParser::InvalidOption.
	KindInvalidOption
	// KindMissingArgument is OptionParser::MissingArgument.
	KindMissingArgument
	// KindInvalidArgument is OptionParser::InvalidArgument.
	KindInvalidArgument
	// KindAmbiguousOption is OptionParser::AmbiguousOption.
	KindAmbiguousOption
	// KindAmbiguousArgument is OptionParser::AmbiguousArgument (an InvalidArgument).
	KindAmbiguousArgument
	// KindNeedlessArgument is OptionParser::NeedlessArgument.
	KindNeedlessArgument
)

// reasons holds the MRI `reason` string for each kind (the class-level `reason`).
var reasons = map[ErrorKind]string{
	KindParseError:        "parse error",
	KindInvalidOption:     "invalid option",
	KindMissingArgument:   "missing argument",
	KindInvalidArgument:   "invalid argument",
	KindAmbiguousOption:   "ambiguous option",
	KindAmbiguousArgument: "ambiguous argument",
	KindNeedlessArgument:  "needless argument",
}

// rubyClass holds the MRI exception class name for each kind, so a consumer can
// re-raise the corresponding Ruby exception.
var rubyClass = map[ErrorKind]string{
	KindParseError:        "OptionParser::ParseError",
	KindInvalidOption:     "OptionParser::InvalidOption",
	KindMissingArgument:   "OptionParser::MissingArgument",
	KindInvalidArgument:   "OptionParser::InvalidArgument",
	KindAmbiguousOption:   "OptionParser::AmbiguousOption",
	KindAmbiguousArgument: "OptionParser::AmbiguousArgument",
	KindNeedlessArgument:  "OptionParser::NeedlessArgument",
}

// ParseError is the Go analogue of OptionParser::ParseError. Its Error string,
// Reason, Args, Class and Recover all match MRI byte-for-byte.
type ParseError struct {
	// Kind selects the ParseError subtype.
	Kind ErrorKind
	// Args are the offending tokens, in MRI order. Error() renders them as
	// "<reason>: <args joined by space>".
	Args []string
}

// Reason returns the MRI class-level reason string (e.g. "invalid option").
func (e *ParseError) Reason() string { return reasons[e.Kind] }

// Class returns the MRI exception class name (e.g. "OptionParser::InvalidOption"),
// for a consumer that re-raises the matching Ruby exception.
func (e *ParseError) Class() string { return rubyClass[e.Kind] }

// Error renders the message exactly as MRI's #message: "reason: arg arg".
func (e *ParseError) Error() string {
	return e.Reason() + ": " + strings.Join(e.Args, " ")
}

// Recover prepends this error's offending Args to argv and returns it, mirroring
// MRI's ParseError#recover (argv[0,0] = @args).
func (e *ParseError) Recover(argv []string) []string {
	return append(append([]string(nil), e.Args...), argv...)
}

func newErr(kind ErrorKind, args ...string) *ParseError {
	return &ParseError{Kind: kind, Args: args}
}
