package cli

import (
	"reflect"
	"strings"
	"testing"
)

func TestDecodeKey(t *testing.T) {
	tests := []struct {
		in       []byte
		want     key
		digit    int
		consumed int
	}{
		{[]byte{'\r'}, keyEnter, 0, 1},
		{[]byte{'\n'}, keyEnter, 0, 1},
		{[]byte{27, '[', 'A'}, keyUp, 0, 3},
		{[]byte{27, '[', 'B'}, keyDown, 0, 3},
		{[]byte{'k'}, keyUp, 0, 1},
		{[]byte{'j'}, keyDown, 0, 1},
		{[]byte{27}, keyCancel, 0, 1},
		{[]byte{3}, keyCancel, 0, 1},
		{[]byte{'q'}, keyCancel, 0, 1},
		{[]byte{'1'}, keyDigit, 0, 1},
		{[]byte{'3'}, keyDigit, 2, 1},
		{[]byte{'x'}, keyNone, 0, 1},
		{[]byte{27, '[', 'C'}, keyNone, 0, 3}, // right arrow: ignored
		// One read carrying several keys: only the first is decoded, and
		// consumed tells the caller where the next one starts.
		{[]byte{27, '[', 'B', '\r'}, keyDown, 0, 3},
		{nil, keyNone, 0, 0},
	}
	for _, tt := range tests {
		k, d, c := decodeKey(tt.in)
		if k != tt.want || d != tt.digit || c != tt.consumed {
			t.Errorf("decodeKey(%v) = %v, %d, %d; want %v, %d, %d", tt.in, k, d, c, tt.want, tt.digit, tt.consumed)
		}
	}
}

func TestSelectFallsBackToNumbered(t *testing.T) {
	// Non-file stdin (as in tests and pipes) must use the numbered prompt.
	app, out, _, _ := newTestApp(t, "2\n")
	app.Interactive = true
	idx, err := app.selectFrom("Pick one", []string{"alpha", "beta"})
	if err != nil || idx != 1 {
		t.Fatalf("selectFrom = %d, %v; want 1, nil", idx, err)
	}
	if !strings.Contains(out.String(), "1) alpha") || !strings.Contains(out.String(), "2) beta") {
		t.Fatalf("numbered fallback not rendered:\n%s", out)
	}
}

func TestSelectFallbackCancelAndInvalid(t *testing.T) {
	app, _, _, _ := newTestApp(t, "\n")
	app.Interactive = true
	if idx, err := app.selectFrom("Pick", []string{"a"}); err != nil || idx != -1 {
		t.Fatalf("empty input should cancel, got %d, %v", idx, err)
	}

	app2, _, _, _ := newTestApp(t, "7\n")
	app2.Interactive = true
	if _, err := app2.selectFrom("Pick", []string{"a"}); err == nil {
		t.Fatal("out-of-range choice should error")
	}
}

func TestOfferCopySourceStartClean(t *testing.T) {
	app, out, _, _ := newTestApp(t, "1\n")
	app.Run([]string{"create", "existing", "--no-alias"})
	app.Interactive = true

	if src := app.offerCopySource("new"); src != "" {
		t.Fatalf("option 1 must mean start clean, got %q", src)
	}
	if !strings.Contains(out.String(), "Start clean") {
		t.Fatalf("start-clean option not offered:\n%s", out)
	}
}

func TestOfferCopySourcePicksProfile(t *testing.T) {
	app, _, _, _ := newTestApp(t, "2\n")
	app.Run([]string{"create", "existing", "--no-alias"})
	app.Interactive = true

	if src := app.offerCopySource("new"); src != "existing" {
		t.Fatalf("option 2 should pick the profile, got %q", src)
	}
}

func TestParseMultiChoice(t *testing.T) {
	cases := []struct {
		in   string
		want []int
		err  bool
	}{
		{"", nil, false},
		{"a", []int{0, 1, 2, 3}, false},
		{"A", []int{0, 1, 2, 3}, false},
		{"2", []int{1}, false},
		{"1,3", []int{0, 2}, false},
		{" 1 , 3 ", []int{0, 2}, false},
		{"3,1,3", []int{0, 2}, false},
		{"0", nil, true},
		{"5", nil, true},
		{"x", nil, true},
		{"1,x", nil, true},
	}
	for _, c := range cases {
		got, err := parseMultiChoice(c.in, 4)
		if c.err != (err != nil) {
			t.Errorf("parseMultiChoice(%q) error = %v, want error=%v", c.in, err, c.err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseMultiChoice(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestDecodeKeySpace(t *testing.T) {
	k, _, consumed := decodeKey([]byte{' '})
	if k != keySpace || consumed != 1 {
		t.Errorf("decodeKey(space) = (%v, %d), want (keySpace, 1)", k, consumed)
	}
}

func TestMultiSelectNumberedFallback(t *testing.T) {
	app, out, _, _ := newTestApp(t, "1,3\n")
	app.Interactive = true // strings.Reader stdin still forces the fallback
	got, err := app.multiSelect("Pick", []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []int{0, 2}) {
		t.Fatalf("multiSelect = %v, want [0 2]", got)
	}
	if !strings.Contains(out.String(), "1) a") {
		t.Fatalf("numbered menu not rendered:\n%s", out)
	}
}
