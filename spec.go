package optparse

import "regexp"

// ArgStyle describes whether a switch takes an argument and how.
type ArgStyle int

const (
	// ArgNone is a flag with no argument.
	ArgNone ArgStyle = iota
	// ArgRequired is a switch with a mandatory argument ("--name VALUE").
	ArgRequired
	// ArgOptional is a switch with an optional argument ("--name [VALUE]").
	ArgOptional
)

// Coercion names the value converter applied to a matched argument before it is
// reported in a Match. The empty string means "no coercion" (the raw string is
// reported). Built-in names mirror MRI's accept defaults; CoerceList carries an
// explicit candidate set instead (see Spec.List).
const (
	CoerceNone    = ""
	CoerceString  = "String"
	CoerceInteger = "Integer"
	CoerceFloat   = "Float"
	CoerceArray   = "Array"
	// CoerceList selects the candidate-list converter built from Spec.List.
	CoerceList = "List"
)

// Spec is the parsed form of one MRI `on(...)` declaration. A consumer builds it
// from the Ruby call (rbgo maps the `on(*args, &block)` arguments onto these
// fields) and registers it with Parser.On; the matching Ruby block is held by the
// consumer and dispatched against the resulting Match.
type Spec struct {
	// Short are the short forms, each including the leading dash (e.g. "-v").
	Short []string
	// Long are the long forms, each including the leading dashes (e.g. "--verbose").
	// For a negatable switch this holds the affirmative form only ("--color"); the
	// "--no-color" alias is synthesized at registration time.
	Long []string
	// ArgStyle is none/required/optional.
	ArgStyle ArgStyle
	// ArgName is the placeholder shown in help (e.g. "VALUE"), or "" if none.
	ArgName string
	// Negatable marks a "--[no-]flag" switch.
	Negatable bool
	// Coerce names the value converter (CoerceInteger, CoerceFloat, ...). Empty
	// means no coercion.
	Coerce string
	// List is the ordered candidate set for CoerceList. For a value→value map use
	// Keys with parallel Values; for a plain list leave Values nil and the key is
	// returned as itself.
	List []string
	// Values, when non-nil and the same length as List, supplies the value each
	// candidate maps to (MRI's Hash form of `on(...)`). Stored as the raw matched
	// candidate string; the consumer maps it back to a Ruby object.
	Values []string
	// Desc are the description lines shown in help.
	Desc []string
}

// switchDef is the internal registered representation (the MRI Switch struct).
type switchDef struct {
	short     []string
	long      []string
	arg       string
	mandatory bool
	optional  bool
	negatable bool
	conv      converter
	desc      []string
	specIndex int
}

// converter maps a raw argument string to a coerced value, or returns a
// *ParseError (InvalidArgument / AmbiguousArgument) on failure.
type converter func(s string, present bool) (any, error)

var (
	reHex = regexp.MustCompile(`\A[-+]?0[xX][0-9a-fA-F_]+\z`)
	reBin = regexp.MustCompile(`\A[-+]?0[bB][01_]+\z`)
	reOct = regexp.MustCompile(`\A[-+]?0[oO][0-7_]+\z`)
	reDec = regexp.MustCompile(`\A[-+]?\d[\d_]*\z`)
	reFlt = regexp.MustCompile(`\A[-+]?(\d+\.?\d*|\.\d+)([eE][-+]?\d+)?\z`)
)
