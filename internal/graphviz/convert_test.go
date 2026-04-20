package graphviz

import (
	"strings"
	"testing"
)

func TestDotValue_BareIDsUnquoted(t *testing.T) {
	cases := map[string]string{
		"TB":         "TB",
		"true":       "true",
		"false":      "false",
		"spline":     "spline",
		"my_value":   "my_value",
		"123":        "123",
		"-3.14":      "-3.14",
		".5":         ".5", // Graphviz numerals accept leading dot per spec
		"7.3,10.2":   `"7.3,10.2"`,
		"":           `""`,
		"a.b":        `"a.b"`,
		"with space": `"with space"`,
		`has"quote`:  `"has\"quote"`,
	}
	for in, want := range cases {
		got := dotValue(in)
		if got != want {
			t.Errorf("dotValue(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsBareNumeral(t *testing.T) {
	cases := map[string]bool{
		"0":     true,
		"1":     true,
		"-1":    true,
		"3.14":  true,
		"-3.14": true,
		".5":    true, // "-?(\.[0-9]+|[0-9]+...)" covers it
		"1.":    true,
		"":      false,
		"-":     false,
		"abc":   false,
		"1a":    false,
		"1.2.3": false,
	}
	for in, want := range cases {
		if got := isBareNumeral(in); got != want {
			t.Errorf("isBareNumeral(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsBareID(t *testing.T) {
	cases := map[string]bool{
		"foo":       true,
		"_foo":      true,
		"foo_bar":   true,
		"foo123":    true,
		"cluster_x": true,
		"1foo":      false,
		"foo.bar":   false,
		"foo-bar":   false,
		"":          false,
	}
	for in, want := range cases {
		if got := isBareID(in); got != want {
			t.Errorf("isBareID(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestDotQuote_EscapesSpecials(t *testing.T) {
	got := dotQuote(`he said "hi"`)
	want := `"he said \"hi\""`
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	got = dotQuote("line1\nline2")
	if !strings.Contains(got, `\n`) {
		t.Errorf("expected \\n escape, got %q", got)
	}
}

func TestRootDirectionToRankdir(t *testing.T) {
	cases := map[string]string{
		"":      "TB",
		"down":  "TB",
		"up":    "BT",
		"left":  "RL",
		"right": "LR",
		"junk":  "TB",
	}
	for in, want := range cases {
		if got := rootDirectionToRankdir(in); got != want {
			t.Errorf("rootDirectionToRankdir(%q) = %q want %q", in, got, want)
		}
	}
}
