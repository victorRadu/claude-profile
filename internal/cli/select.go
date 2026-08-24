package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// key is a decoded keypress in the interactive select.
type key int

const (
	keyNone key = iota
	keyUp
	keyDown
	keyEnter
	keyCancel
	keyDigit
	keySpace
)

// decodeKey interprets the first key in a raw-mode input buffer and
// reports how many bytes it consumed. One read may carry several keys
// (a pty can deliver "ESC [ B \r" at once), so callers must keep decoding
// until the buffer is drained. Arrow keys arrive as ANSI escape sequences
// (ESC [ A / ESC [ B); a bare ESC is a cancel.
func decodeKey(b []byte) (k key, digit, consumed int) {
	if len(b) == 0 {
		return keyNone, 0, 0
	}
	switch b[0] {
	case '\r', '\n':
		return keyEnter, 0, 1
	case 3, 'q': // Ctrl-C, q
		return keyCancel, 0, 1
	case 'k':
		return keyUp, 0, 1
	case 'j':
		return keyDown, 0, 1
	case ' ':
		return keySpace, 0, 1
	case 27: // ESC
		if len(b) >= 3 && b[1] == '[' {
			switch b[2] {
			case 'A':
				return keyUp, 0, 3
			case 'B':
				return keyDown, 0, 3
			}
			return keyNone, 0, 3 // other CSI sequence: ignore
		}
		if len(b) >= 2 && b[1] == '[' {
			return keyNone, 0, 2 // truncated sequence: ignore
		}
		return keyCancel, 0, 1 // bare ESC
	}
	if b[0] >= '1' && b[0] <= '9' {
		return keyDigit, int(b[0] - '1'), 1
	}
	return keyNone, 0, 1
}

// selectFrom shows an interactive menu and returns the chosen index, or
// -1 when cancelled. On a real terminal it renders an arrow-key menu;
// otherwise it falls back to a numbered prompt, so pipes, tests and
// legacy consoles keep working.
func (a *App) selectFrom(title string, options []string) (int, error) {
	if f, ok := a.Stdin.(*os.File); ok && a.Interactive && isTerminal(f) {
		if idx, handled := a.selectRaw(f, title, options); handled {
			return idx, nil
		}
	}
	return a.selectNumbered(title, options)
}

// selectRaw is the arrow-key menu. handled is false when raw mode could
// not be enabled, signalling the caller to fall back.
func (a *App) selectRaw(f *os.File, title string, options []string) (idx int, handled bool) {
	restore, err := enableRaw(f)
	if err != nil {
		return 0, false
	}
	defer restore()

	out := a.Stdout
	st := a.Style
	fmt.Fprint(out, "\x1b[?25l")       // hide cursor
	defer fmt.Fprint(out, "\x1b[?25h") // show cursor

	sel := 0
	lines := len(options) + 2 // title + options + hint
	render := func() {
		fmt.Fprintf(out, "%s\x1b[K\r\n", st.bold(title))
		for i, opt := range options {
			if i == sel {
				fmt.Fprintf(out, "%s %s\x1b[K\r\n", st.cyan("❯"), st.bold(st.cyan(opt)))
			} else {
				fmt.Fprintf(out, "  %s\x1b[K\r\n", opt)
			}
		}
		fmt.Fprintf(out, "%s\x1b[K\r", st.dim("↑/↓ move · enter select · esc cancel"))
	}
	erase := func() {
		fmt.Fprintf(out, "\x1b[%dA\r\x1b[J", lines-1)
	}

	render()
	for {
		if !a.fillPending(f) {
			erase()
			return -1, true
		}
		k, d, consumed := decodeKey(a.rawPending)
		a.rawPending = a.rawPending[consumed:]
		switch k {
		case keyUp:
			sel = (sel + len(options) - 1) % len(options)
		case keyDown:
			sel = (sel + 1) % len(options)
		case keyEnter:
			erase()
			return sel, true
		case keyCancel:
			erase()
			return -1, true
		case keyDigit:
			if d < len(options) {
				erase()
				return d, true
			}
			continue
		case keyNone:
			continue
		}
		fmt.Fprintf(out, "\x1b[%dA\r", lines-1)
		render()
	}
}

// selectNumbered is the line-based fallback.
func (a *App) selectNumbered(title string, options []string) (int, error) {
	a.printf("%s\n", a.Style.bold(title))
	for i, opt := range options {
		a.printf("%3d) %s\n", i+1, opt)
	}
	choice := a.ask(fmt.Sprintf("Choose [1-%d, Enter to cancel] ", len(options)))
	if choice == "" {
		return -1, nil
	}
	n, err := parseChoice(choice, len(options))
	if err != nil {
		return -1, err
	}
	return n - 1, nil
}

// fillPending ensures at least one raw byte is buffered, reading from the
// terminal when needed. Returns false on EOF or read error.
func (a *App) fillPending(f *os.File) bool {
	if len(a.rawPending) > 0 {
		return true
	}
	buf := make([]byte, 64)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return false
	}
	a.rawPending = append(a.rawPending, buf[:n]...)
	return true
}

// confirmKey is a single-keypress yes/no on a real terminal, with a
// line-based fallback elsewhere. Enter and Escape mean "no" — the safe
// default for every confirmation in this tool.
func (a *App) confirmKey(prompt string) bool {
	f, ok := a.Stdin.(*os.File)
	if !ok || !a.Interactive || !isTerminal(f) {
		return a.confirmLine(prompt)
	}
	restore, err := enableRaw(f)
	if err != nil {
		return a.confirmLine(prompt)
	}
	defer restore()

	fmt.Fprintf(a.Stdout, "%s %s ", a.Style.bold(prompt), a.Style.dim("[y/N]"))
	yes := false
	if a.fillPending(f) {
		yes = a.rawPending[0] == 'y' || a.rawPending[0] == 'Y'
		a.rawPending = a.rawPending[1:]
	}
	if yes {
		fmt.Fprint(a.Stdout, "y\r\n")
	} else {
		fmt.Fprint(a.Stdout, "n\r\n")
	}
	return yes
}

// multiSelect shows a checklist and returns the chosen indexes, sorted.
// nil means nothing was chosen (or the user cancelled). On a real terminal
// it renders a space-to-toggle menu; otherwise a numbered prompt accepting
// comma-separated choices, so pipes and tests keep working.
func (a *App) multiSelect(title string, options []string) ([]int, error) {
	if f, ok := a.Stdin.(*os.File); ok && a.Interactive && isTerminal(f) {
		if picks, handled := a.multiSelectRaw(f, title, options); handled {
			return picks, nil
		}
	}
	return a.multiSelectNumbered(title, options)
}

func (a *App) multiSelectRaw(f *os.File, title string, options []string) (picks []int, handled bool) {
	restore, err := enableRaw(f)
	if err != nil {
		return nil, false
	}
	defer restore()

	out := a.Stdout
	st := a.Style
	fmt.Fprint(out, "\x1b[?25l")
	defer fmt.Fprint(out, "\x1b[?25h")

	sel := 0
	checked := make([]bool, len(options))
	lines := len(options) + 2
	render := func() {
		fmt.Fprintf(out, "%s\x1b[K\r\n", st.bold(title))
		for i, opt := range options {
			box := "◯"
			if checked[i] {
				box = st.green("◉")
			}
			if i == sel {
				fmt.Fprintf(out, "%s %s %s\x1b[K\r\n", st.cyan("❯"), box, st.bold(st.cyan(opt)))
			} else {
				fmt.Fprintf(out, "  %s %s\x1b[K\r\n", box, opt)
			}
		}
		fmt.Fprintf(out, "%s\x1b[K\r", st.dim("↑/↓ move · space toggle · enter confirm · esc cancel"))
	}
	erase := func() {
		fmt.Fprintf(out, "\x1b[%dA\r\x1b[J", lines-1)
	}

	render()
	for {
		if !a.fillPending(f) {
			erase()
			return nil, true
		}
		k, d, consumed := decodeKey(a.rawPending)
		a.rawPending = a.rawPending[consumed:]
		switch k {
		case keyUp:
			sel = (sel + len(options) - 1) % len(options)
		case keyDown:
			sel = (sel + 1) % len(options)
		case keySpace:
			checked[sel] = !checked[sel]
		case keyDigit:
			if d < len(options) {
				checked[d] = !checked[d]
			} else {
				continue
			}
		case keyEnter:
			erase()
			for i, c := range checked {
				if c {
					picks = append(picks, i)
				}
			}
			return picks, true
		case keyCancel:
			erase()
			return nil, true
		case keyNone:
			continue
		}
		fmt.Fprintf(out, "\x1b[%dA\r", lines-1)
		render()
	}
}

// multiSelectNumbered is the line-based fallback.
func (a *App) multiSelectNumbered(title string, options []string) ([]int, error) {
	a.printf("%s\n", a.Style.bold(title))
	for i, opt := range options {
		a.printf("%3d) %s\n", i+1, opt)
	}
	choice := a.ask("Choose [e.g. 1,3 · 'a' for all · Enter for none] ")
	return parseMultiChoice(choice, len(options))
}

// parseMultiChoice parses a comma-separated list of 1-based selections;
// "a" selects everything, an empty input selects nothing.
func parseMultiChoice(input string, limit int) ([]int, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}
	if input == "a" || input == "A" {
		all := make([]int, limit)
		for i := range all {
			all[i] = i
		}
		return all, nil
	}
	var out []int
	for _, part := range strings.Split(input, ",") {
		n, err := parseChoice(strings.TrimSpace(part), limit)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(out, n-1) {
			out = append(out, n-1)
		}
	}
	slices.Sort(out)
	return out, nil
}
