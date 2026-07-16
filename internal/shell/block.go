package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Managed block markers. Everything between them is owned by claude-profile;
// user content outside the block is never touched.
const (
	BlockStart = "# >>> claude-profile >>>"
	BlockEnd   = "# <<< claude-profile <<<"
)

// SetLine inserts or replaces one line inside the managed block of file.
// Lines are identified by prefix key, so updating is idempotent.
// The file and block are created if missing.
func SetLine(file, key, line string) error {
	content, err := readFileOrEmpty(file)
	if err != nil {
		return err
	}
	head, block, tail, found := splitBlock(content)
	if !found {
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		head, block, tail = content+"\n", nil, ""
	}
	kept := block[:0:0]
	for _, l := range block {
		if !strings.HasPrefix(strings.TrimSpace(l), key) {
			kept = append(kept, l)
		}
	}
	kept = append(kept, line)
	return writeBlock(file, head, kept, tail)
}

// RemoveLine deletes any line starting with key from the managed block.
// If the block becomes empty it is removed entirely. Missing files are a no-op.
func RemoveLine(file, key string) error {
	content, err := readFileOrEmpty(file)
	if err != nil || content == "" {
		return err
	}
	head, block, tail, found := splitBlock(content)
	if !found {
		return nil
	}
	kept := block[:0:0]
	for _, l := range block {
		if !strings.HasPrefix(strings.TrimSpace(l), key) {
			kept = append(kept, l)
		}
	}
	if len(kept) == 0 {
		return os.WriteFile(file, []byte(strings.TrimLeft(head+tail, "\n")), 0o600)
	}
	return writeBlock(file, head, kept, tail)
}

// SetTaggedLine inserts or replaces the line containing tag inside the
// managed block. Unlike SetLine, the match is on a marker substring rather
// than a prefix, so the line's leading content (e.g. an alias name chosen
// by the user) can change between updates.
func SetTaggedLine(file, tag, line string) error {
	content, err := readFileOrEmpty(file)
	if err != nil {
		return err
	}
	head, block, tail, found := splitBlock(content)
	if !found {
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		head, block, tail = content+"\n", nil, ""
	}
	kept := block[:0:0]
	for _, l := range block {
		if !strings.Contains(l, tag) {
			kept = append(kept, l)
		}
	}
	kept = append(kept, line)
	return writeBlock(file, head, kept, tail)
}

// RemoveTaggedLine deletes any line containing tag from the managed block.
func RemoveTaggedLine(file, tag string) error {
	content, err := readFileOrEmpty(file)
	if err != nil || content == "" {
		return err
	}
	head, block, tail, found := splitBlock(content)
	if !found {
		return nil
	}
	kept := block[:0:0]
	for _, l := range block {
		if !strings.Contains(l, tag) {
			kept = append(kept, l)
		}
	}
	if len(kept) == 0 {
		return os.WriteFile(file, []byte(strings.TrimLeft(head+tail, "\n")), 0o600)
	}
	return writeBlock(file, head, kept, tail)
}

// FindTaggedLine returns the managed-block line containing tag, if any.
func FindTaggedLine(file, tag string) (string, bool, error) {
	content, err := readFileOrEmpty(file)
	if err != nil {
		return "", false, err
	}
	_, block, _, found := splitBlock(content)
	if !found {
		return "", false, nil
	}
	for _, l := range block {
		if strings.Contains(l, tag) {
			return l, true, nil
		}
	}
	return "", false, nil
}

// HasLine reports whether any line in file (inside or outside the managed
// block) begins with key, ignoring leading whitespace.
func HasLine(file, key string) (bool, error) {
	content, err := readFileOrEmpty(file)
	if err != nil {
		return false, err
	}
	for _, l := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), key) {
			return true, nil
		}
	}
	return false, nil
}

func readFileOrEmpty(file string) (string, error) {
	b, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", file, err)
	}
	return string(b), nil
}

// splitBlock separates content into text before the block, the lines inside
// it, and text after it.
func splitBlock(content string) (head string, block []string, tail string, found bool) {
	lines := strings.Split(content, "\n")
	start, end := -1, -1
	for i, l := range lines {
		switch strings.TrimSpace(l) {
		case BlockStart:
			if start == -1 {
				start = i
			}
		case BlockEnd:
			if start != -1 && end == -1 {
				end = i
			}
		}
	}
	if start == -1 || end == -1 {
		return content, nil, "", false
	}
	head = strings.Join(lines[:start], "\n")
	if head != "" {
		head += "\n"
	}
	block = lines[start+1 : end]
	tail = strings.Join(lines[end+1:], "\n")
	if tail != "" && !strings.HasSuffix(tail, "\n") {
		tail += "\n"
	}
	return head, block, tail, true
}

func writeBlock(file, head string, block []string, tail string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString(head)
	b.WriteString(BlockStart + "\n")
	for _, l := range block {
		b.WriteString(l + "\n")
	}
	b.WriteString(BlockEnd + "\n")
	b.WriteString(tail)
	return os.WriteFile(file, []byte(b.String()), 0o600)
}
