package cli

import (
	"strings"
	"testing"
)

func TestEveryCommandHasHelp(t *testing.T) {
	for _, name := range commandOrder {
		h, ok := helpTopics[name]
		if !ok {
			t.Errorf("command %q missing from helpTopics", name)
			continue
		}
		if h.summary == "" || h.usage == "" {
			t.Errorf("command %q help lacks summary or usage", name)
		}
		if !strings.Contains(h.usage, "claude-profile "+name) {
			t.Errorf("command %q usage does not mention the command: %q", name, h.usage)
		}
	}
}

func TestHelpCommandTopic(t *testing.T) {
	app, out, _, _ := newTestApp(t, "")
	if code := app.Run([]string{"help", "create"}); code != 0 {
		t.Fatal("help create failed")
	}
	got := out.String()
	for _, want := range []string{"claude-profile create", "Usage:", "--from", "Examples:"} {
		if !strings.Contains(got, want) {
			t.Errorf("help create missing %q:\n%s", want, got)
		}
	}
}

func TestHelpFlagOnCommand(t *testing.T) {
	app, out, _, _ := newTestApp(t, "")
	if code := app.Run([]string{"remove", "--help"}); code != 0 {
		t.Fatal("remove --help failed")
	}
	if !strings.Contains(out.String(), "--force") {
		t.Fatalf("remove --help missing flag docs:\n%s", out)
	}
	if app.Store.Exists("--help") {
		t.Fatal("--help must not be treated as a profile name")
	}
}

func TestHelpUnknownTopic(t *testing.T) {
	app, _, errBuf, _ := newTestApp(t, "")
	if code := app.Run([]string{"help", "frobnicate"}); code == 0 {
		t.Fatal("help for unknown command should fail")
	}
	if !strings.Contains(errBuf.String(), "frobnicate") {
		t.Fatalf("error should name the topic: %s", errBuf)
	}
}

func TestOverviewListsAllCommands(t *testing.T) {
	app, out, _, _ := newTestApp(t, "")
	app.Run([]string{"help"})
	for _, name := range commandOrder {
		if !strings.Contains(out.String(), name) {
			t.Errorf("overview missing command %q", name)
		}
	}
}

func TestPaletteDisabledProducesPlainOutput(t *testing.T) {
	app, out, _, _ := newTestApp(t, "")
	app.Run([]string{"help"})
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatal("ANSI codes emitted with styling disabled")
	}
}

func TestPaletteEnabledStyles(t *testing.T) {
	st := palette{on: true}
	if got := st.bold("x"); got != "\x1b[1mx\x1b[0m" {
		t.Fatalf("bold = %q", got)
	}
	if got := stripANSI(st.cyan(st.bold("name"))); got != "name" {
		t.Fatalf("stripANSI = %q, want name", got)
	}
	if got := pad(st.green("ab"), st, 5); stripANSI(got) != "ab   " {
		t.Fatalf("pad visible = %q, want 'ab   '", stripANSI(got))
	}
}
