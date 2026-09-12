package cmd

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// pickProfile is a minimal fzf-style interactive picker over profile names:
// type to filter (substring, case-insensitive), Up/Down to move, Enter to
// select, Esc/Ctrl-C to cancel.
func pickProfile(names []string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("profile switch requires an interactive terminal")
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", fmt.Errorf("enter raw terminal mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	query := ""
	selected := 0
	filtered := filterProfiles(names, query)
	linesDrawn := 0

	redraw := func() {
		// Clear previously drawn lines.
		for i := 0; i < linesDrawn; i++ {
			fmt.Fprint(os.Stderr, "\r\x1b[K")
			if i < linesDrawn-1 {
				fmt.Fprint(os.Stderr, "\x1b[1A")
			}
		}
		if linesDrawn > 0 {
			fmt.Fprint(os.Stderr, "\r")
		}

		fmt.Fprintf(os.Stderr, "Switch profile: %s\r\n", query)
		max := len(filtered)
		if max > 15 {
			max = 15
		}
		for i := 0; i < max; i++ {
			prefix := "  "
			if i == selected {
				prefix = "> "
			}
			fmt.Fprintf(os.Stderr, "%s%s\r\n", prefix, filtered[i])
		}
		if len(filtered) == 0 {
			fmt.Fprintf(os.Stderr, "  (no matches)\r\n")
			max = 1
		}
		linesDrawn = max + 1
	}

	redraw()

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return "", err
		}

		switch {
		case n == 1 && (buf[0] == 3 || buf[0] == 27): // Ctrl-C or Esc
			fmt.Fprintln(os.Stderr)
			return "", fmt.Errorf("cancelled")
		case n == 1 && (buf[0] == '\r' || buf[0] == '\n'): // Enter
			fmt.Fprintln(os.Stderr)
			if len(filtered) == 0 {
				return "", fmt.Errorf("no matching profile")
			}
			return filtered[selected], nil
		case n == 1 && (buf[0] == 127 || buf[0] == 8): // Backspace
			if len(query) > 0 {
				query = query[:len(query)-1]
			}
		case n == 3 && buf[0] == 27 && buf[1] == '[' && buf[2] == 'A': // Up
			if selected > 0 {
				selected--
			}
		case n == 3 && buf[0] == 27 && buf[1] == '[' && buf[2] == 'B': // Down
			if selected < len(filtered)-1 {
				selected++
			}
		case n >= 1 && buf[0] >= 32 && buf[0] < 127: // printable ASCII
			query += string(buf[0])
		}

		filtered = filterProfiles(names, query)
		if selected >= len(filtered) {
			selected = len(filtered) - 1
		}
		if selected < 0 {
			selected = 0
		}
		redraw()
	}
}

func filterProfiles(names []string, query string) []string {
	if query == "" {
		return names
	}
	q := strings.ToLower(query)
	var out []string
	for _, n := range names {
		if strings.Contains(strings.ToLower(n), q) {
			out = append(out, n)
		}
	}
	return out
}
