package optparse

import "testing"

// Direct unit tests for branches that the end-to-end parse path does not
// naturally reach: the converter sentinels, the absent-argument converter cases,
// and the rare --no- name collisions.

func TestSentinelErrors(t *testing.T) {
	if (invalidArg{}).Error() != "invalid argument" {
		t.Fatal("invalidArg.Error")
	}
	if (ambiguousArg{}).Error() != "ambiguous argument" {
		t.Fatal("ambiguousArg.Error")
	}
}

func TestConvertersAbsent(t *testing.T) {
	if _, err := convInteger("", false); err == nil {
		t.Fatal("convInteger absent should error")
	}
	if _, err := convFloat("", false); err == nil {
		t.Fatal("convFloat absent should error")
	}
	if v, err := convString("", false); err != nil || v != nil {
		t.Fatalf("convString absent: %v / %v", v, err)
	}
	if v, err := convArray("", false); err != nil || v != nil {
		t.Fatalf("convArray absent: %v / %v", v, err)
	}
	lc := listConverter([]string{"a"}, nil)
	if _, err := lc("", false); err == nil {
		t.Fatal("listConverter absent should error")
	}
}

func TestParseBaseMalformed(t *testing.T) {
	// "0x_" matches the hex regex (underscore is allowed) but reduces to an empty
	// digit string, so both ParseInt and big.Int.SetString fail → invalidArg.
	if _, err := convInteger("0x_", true); err == nil {
		t.Fatal("0x_ should be invalid")
	}
}

func TestApplyArgSpecEmptyDescriptor(t *testing.T) {
	// A trailing "=" with nothing after it yields ArgNone (the default branch).
	s := MakeSpec([]string{"--flag="}, "", nil, nil)
	if s.ArgStyle != ArgNone || s.ArgName != "" {
		t.Fatalf("empty descriptor: %+v", s)
	}
}

func TestOptionalArgConvertError(t *testing.T) {
	// An optional argument that IS supplied but fails coercion exercises the
	// optional-branch convert error path.
	p := New()
	p.On(MakeSpec([]string{"--n [N]"}, CoerceInteger, nil, nil))
	if _, _, err := p.ParseBang([]string{"--n=xx"}); err == nil {
		t.Fatal("optional bad int should error")
	}
}

func TestHelpOptionalArgLeft(t *testing.T) {
	// buildLeft's optional-argument branch (" [ARG]").
	p := New()
	p.On(MakeSpec([]string{"--opt [VALUE]", "desc"}, "", nil, nil))
	if got := p.Help(); got == "" {
		t.Fatal("empty")
	} else if want := "--opt [VALUE]"; !contains(got, want) {
		t.Fatalf("missing %q in %q", want, got)
	}
}

func TestNegatableNoCollision(t *testing.T) {
	// A negatable switch whose affirmative long name itself begins with "--no-":
	// "--[no-]no-x" registers "--no-x" (real) and "--no-no-x" (synthetic).
	// Looking up the real "--no-x" must not be treated as a negation.
	p := New()
	p.On(MakeSpec([]string{"--[no-]no-x"}, "", nil, nil))
	m, _, err := p.ParseBang([]string{"--no-x"})
	if err != nil {
		t.Fatal(err)
	}
	if m[0].Value != true {
		t.Fatalf("real --no-x should be affirmative true, got %v", m[0].Value)
	}
	// the synthetic negation still negates
	m, _, _ = p.ParseBang([]string{"--no-no-x"})
	if m[0].Value != false {
		t.Fatalf("--no-no-x should negate, got %v", m[0].Value)
	}
	// completeShort sees the real "--no-x" via its 'n' first letter without being
	// excluded as a synthetic negation.
	p2 := New()
	p2.On(MakeSpec([]string{"--[no-]no-x"}, "", nil, nil))
	if _, _, err := p2.ParseBang([]string{"-n"}); err != nil {
		t.Fatalf("short completion over real --no-x: %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
