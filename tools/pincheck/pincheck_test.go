package pincheck

import "testing"

func TestCheckFile(t *testing.T) {
	sha := "3d3c42e5aac5ba805825da76410c181273ba90b1"
	cases := []struct {
		line string
		ok   bool
	}{
		{"      - uses: actions/checkout@" + sha + " # v7.0.1", true},
		{"      - uses: 'actions/checkout@" + sha + "'", true},
		{"    uses: org/repo/.github/workflows/ci.yml@" + sha, true},
		{"      - uses: ./.github/actions/setup", true},
		{"      - uses: docker://alpine@sha256:" + sha + sha[:24], true},
		{"      - uses: actions/checkout@v7", false},
		{"      - uses: actions/checkout@v7.0.1", false},
		{"      - uses: actions/checkout@main", false},
		{"      - uses: actions/checkout@3d3c42e", false},
		{"      - uses: actions/checkout@" + sha[:39] + "Z", false},
		{"      - uses: docker://alpine:3.20", false},
		{"      - uses: actions/checkout", false},
		{"      - uses:   \"actions/checkout@v7\"   # tag", false},
	}
	for _, c := range cases {
		vs := CheckFile("w.yml", []byte("steps:\n"+c.line+"\n"))
		if got := len(vs) == 0; got != c.ok {
			t.Errorf("%q: pinned = %v, want %v (%v)", c.line, got, c.ok, vs)
		}
		if !c.ok && len(vs) == 1 && vs[0].Line != 2 {
			t.Errorf("%q: line %d, want 2", c.line, vs[0].Line)
		}
	}
}

func TestOtherKeysIgnored(t *testing.T) {
	if vs := CheckFile("w.yml", []byte("name: uses: x@v1\nrun: echo uses: a@v1\n")); len(vs) != 0 {
		t.Errorf("unexpected violations: %v", vs)
	}
}
