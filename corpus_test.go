package optparse

// corpus is the differential corpus: each entry is parsed by both this engine and
// the reference OptionParser and the results compared. It exercises long/short/
// both forms, required/optional/= /bundled/--[no-] args, abbreviation, short→long
// completion, every coercion, parse!/order!/permute!/getopts, the -- terminator,
// the "-" operand, and the full error tree.
var corpus = []oracleInput{
	// --- plain flags ---------------------------------------------------------
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-v"}}}, Argv: []string{"-v"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--verbose"}}}, Argv: []string{"--verbose"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-v", "--verbose"}}}, Argv: []string{"-v", "--verbose"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-v"}}}, Argv: []string{}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-v"}}}, Argv: []string{"a", "b"}},

	// --- required argument ---------------------------------------------------
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--name VALUE"}}}, Argv: []string{"--name", "bob"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--name VALUE"}}}, Argv: []string{"--name=bob"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-n VALUE"}}}, Argv: []string{"-n", "bob"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-n VALUE"}}}, Argv: []string{"-nbob"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-n VALUE"}}}, Argv: []string{"-n=bob"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--name VALUE"}}}, Argv: []string{"--name"}}, // MissingArgument
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-n VALUE"}}}, Argv: []string{"-n"}},         // MissingArgument

	// --- optional argument ---------------------------------------------------
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--opt [VALUE]"}}}, Argv: []string{"--opt", "x"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--opt [VALUE]"}}}, Argv: []string{"--opt"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--opt [VALUE]"}}}, Argv: []string{"--opt=z"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--opt [VALUE]"}}, {Opts: []string{"-v"}}}, Argv: []string{"--opt", "-v"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-o [VALUE]"}}}, Argv: []string{"-ox"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-o [VALUE]"}}}, Argv: []string{"-o"}},

	// --- bundled shorts ------------------------------------------------------
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-x"}}, {Opts: []string{"-v"}}, {Opts: []string{"-f"}}}, Argv: []string{"-xvf"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-x"}}, {Opts: []string{"-f VALUE"}}}, Argv: []string{"-xfhello"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-x"}}, {Opts: []string{"-f VALUE"}}}, Argv: []string{"-xf", "hello"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-x"}}, {Opts: []string{"-y"}}}, Argv: []string{"-xz"}}, // InvalidOption -z

	// --- --[no-] negation ----------------------------------------------------
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--[no-]color"}}}, Argv: []string{"--color"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--[no-]color"}}}, Argv: []string{"--no-color"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--[no-]color"}}}, Argv: []string{"--no-color=1"}}, // NeedlessArgument

	// --- abbreviation / completion ------------------------------------------
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--verbose"}}}, Argv: []string{"--verb"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--verbose"}}, {Opts: []string{"--version"}}}, Argv: []string{"--ver"}}, // Ambiguous
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--name VALUE"}}}, Argv: []string{"-n", "bob"}},                         // short completes long
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--name"}}, {Opts: []string{"--node"}}}, Argv: []string{"-n"}},          // AmbiguousOption -n

	// --- coercion: Integer ---------------------------------------------------
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--n N"}, Coerce: "Integer"}}, Argv: []string{"--n", "42"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--n N"}, Coerce: "Integer"}}, Argv: []string{"--n", "0xff"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--n N"}, Coerce: "Integer"}}, Argv: []string{"--n", "0b1010"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--n N"}, Coerce: "Integer"}}, Argv: []string{"--n", "0o17"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--n N"}, Coerce: "Integer"}}, Argv: []string{"--n", "-42"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--n N"}, Coerce: "Integer"}}, Argv: []string{"--n", "1_000"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--n N"}, Coerce: "Integer"}}, Argv: []string{"--n", "99999999999999999999999999"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--n N"}, Coerce: "Integer"}}, Argv: []string{"--n", "xx"}}, // InvalidArgument long
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--n N"}, Coerce: "Integer"}}, Argv: []string{"--n=xx"}},    // InvalidArgument long=
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-n N"}, Coerce: "Integer"}}, Argv: []string{"-n", "xx"}},   // InvalidArgument short

	// --- coercion: Float -----------------------------------------------------
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--f F"}, Coerce: "Float"}}, Argv: []string{"--f", "3.14"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--f F"}, Coerce: "Float"}}, Argv: []string{"--f", "-2.5e3"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--f F"}, Coerce: "Float"}}, Argv: []string{"--f", ".5"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--f F"}, Coerce: "Float"}}, Argv: []string{"--f", "nope"}}, // InvalidArgument

	// --- coercion: Array -----------------------------------------------------
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--list L"}, Coerce: "Array"}}, Argv: []string{"--list", "a,b,c"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--list L"}, Coerce: "Array"}}, Argv: []string{"--list", "single"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--list [L]"}, Coerce: "Array"}}, Argv: []string{"--list"}}, // nil through

	// --- coercion: String ----------------------------------------------------
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--s S"}, Coerce: "String"}}, Argv: []string{"--s", "hi"}},

	// --- coercion: candidate list -------------------------------------------
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--mode M"}, Coerce: "List", List: []string{"fast", "slow"}}}, Argv: []string{"--mode", "fast"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--mode M"}, Coerce: "List", List: []string{"fast", "slow"}}}, Argv: []string{"--mode", "fa"}},   // prefix
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--mode M"}, Coerce: "List", List: []string{"fast", "fail"}}}, Argv: []string{"--mode", "fa"}},   // AmbiguousArgument
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--mode M"}, Coerce: "List", List: []string{"fast", "slow"}}}, Argv: []string{"--mode", "nope"}}, // InvalidArgument
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"--mode M"}, Coerce: "List", List: []string{"a", "b"}, Values: []string{"AA", "BB"}}}, Argv: []string{"--mode", "a"}},

	// --- parse!/order!/permute! ---------------------------------------------
	{Mode: "permute", Specs: []oracleSpec{{Opts: []string{"-v"}}}, Argv: []string{"a", "-v", "b"}},
	{Mode: "order", Specs: []oracleSpec{{Opts: []string{"-v"}}}, Argv: []string{"a", "-v", "b"}},
	{Mode: "order", Specs: []oracleSpec{{Opts: []string{"-v"}}}, Argv: []string{"-v", "a", "b"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-v"}}, {Opts: []string{"-x VALUE"}}}, Argv: []string{"-v", "op1", "-x", "y", "op2"}},

	// --- -- terminator and "-" operand --------------------------------------
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-v"}}}, Argv: []string{"-v", "--", "-x", "a"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-v"}}}, Argv: []string{"-", "-v"}},
	{Mode: "order", Specs: []oracleSpec{{Opts: []string{"-v"}}}, Argv: []string{"-", "-v"}},

	// --- invalid option ------------------------------------------------------
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-v"}}}, Argv: []string{"-x"}},
	{Mode: "parse", Specs: []oracleSpec{{Opts: []string{"-v"}}}, Argv: []string{"--bogus"}},

	// --- getopts -------------------------------------------------------------
	{Mode: "getopts", Argv: []string{"-a", "-b"}, GetoptsSpecs: []string{"a", "b"}},
	{Mode: "getopts", Argv: []string{"--name=bob"}, GetoptsSpecs: []string{"name:"}},
	{Mode: "getopts", Argv: []string{"-x", "val"}, GetoptsSpecs: []string{"x:"}},
	{Mode: "getopts", Argv: []string{}, GetoptsSpecs: []string{"a", "name:"}},
	{Mode: "getopts", Argv: []string{"--verbose"}, GetoptsSpecs: []string{"verbose"}},

	// --- help / summarize layout --------------------------------------------
	{Mode: "parse", Banner: "Usage: tool [opts]", Help: true,
		Specs: []oracleSpec{
			{Opts: []string{"-v", "--verbose", "run verbosely"}},
			{Opts: []string{"--name NAME", "the name to use"}},
			{Opts: []string{"--[no-]color", "toggle color"}},
		},
		Argv: []string{}},
}
