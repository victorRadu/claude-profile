package cli

import "os"

// palette renders ANSI styles. The zero value has styling disabled, so
// tests and piped output stay plain automatically.
type palette struct{ on bool }

// newPalette enables color only for real terminals, honoring NO_COLOR
// (https://no-color.org) and TERM=dumb. On Windows it also switches the
// console into VT processing mode.
func newPalette(f *os.File) palette {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return palette{}
	}
	return palette{on: isTerminal(f) && enableVT(f)}
}

func (p palette) wrap(code, s string) string {
	if !p.on {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (p palette) bold(s string) string   { return p.wrap("1", s) }
func (p palette) dim(s string) string    { return p.wrap("2", s) }
func (p palette) cyan(s string) string   { return p.wrap("36", s) }
func (p palette) green(s string) string  { return p.wrap("32", s) }
func (p palette) yellow(s string) string { return p.wrap("33", s) }
