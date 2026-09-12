package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pterm/pterm"
	"golang.org/x/term"
)

// ── Core reader ─────────────────────────────────────────────────────────────

func deleteWord(buf []byte, hidden bool) []byte {
	if len(buf) == 0 {
		return buf
	}
	i := len(buf) - 1
	// skip trailing spaces
	for i >= 0 && buf[i] == ' ' {
		i--
	}
	// skip non-spaces
	for i >= 0 && buf[i] != ' ' {
		i--
	}
	
	delCount := len(buf) - (i + 1)
	if !hidden {
		for j := 0; j < delCount; j++ {
			fmt.Print("\b \b")
		}
	}
	return buf[:i+1]
}

func deleteLine(buf []byte, hidden bool) []byte {
	delCount := len(buf)
	if !hidden {
		for j := 0; j < delCount; j++ {
			fmt.Print("\b \b")
		}
	}
	return []byte{}
}

// readLine reads one line from stdin in raw mode, handling backspace and Ctrl+C natively.
// It returns (text, cancelled). Ctrl+C cancels and returns ("", true).
func readLine() (string, bool) {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		var text string
		fmt.Scanln(&text)
		return text, false
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	var buf []byte
	b := make([]byte, 1)
	inEsc, inCSI, inSS3 := false, false, false

	for {
		n, err := os.Stdin.Read(b)
		if err != nil || n == 0 {
			break
		}
		c := b[0]

		if inCSI {
			if c >= 0x40 && c <= 0x7E {
				inCSI = false
			}
			continue
		}
		if inSS3 {
			inSS3 = false
			continue
		}
		if inEsc {
			inEsc = false
			if c == 127 || c == 8 {
				// Alt+Backspace
				buf = deleteWord(buf, false)
			} else if c == '[' {
				inCSI = true // Arrow keys, etc.
			} else if c == 'O' {
				inSS3 = true // F1-F4, etc.
			}
			// other Alt+keys are swallowed
			continue
		}
		
		switch c {
		case 3: // Ctrl+C
			fmt.Print("^C\r\n")
			return "", true
		case 4: // Ctrl+D
			fmt.Print("\r\n")
			return string(buf), false
		case '\r', '\n': // Enter
			fmt.Print("\r\n")
			return string(buf), false
		case 127, 8: // Backspace (DEL or BS)
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				fmt.Print("\b \b")
			}
		case 23: // Ctrl+W
			buf = deleteWord(buf, false)
		case 21: // Ctrl+U
			buf = deleteLine(buf, false)
		case 27: // ESC
			inEsc = true
		default:
			// Only accept printable characters
			if c >= 32 && c <= 126 {
				buf = append(buf, c)
				fmt.Print(string(c))
			}
		}
	}
	return string(buf), false
}

// readLineHidden is identical to readLine but does not echo characters.
func readLineHidden() (string, bool) {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		var text string
		fmt.Scanln(&text)
		return text, false
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	var buf []byte
	b := make([]byte, 1)
	inEsc, inCSI, inSS3 := false, false, false

	for {
		n, err := os.Stdin.Read(b)
		if err != nil || n == 0 {
			break
		}
		c := b[0]

		if inCSI {
			if c >= 0x40 && c <= 0x7E {
				inCSI = false
			}
			continue
		}
		if inSS3 {
			inSS3 = false
			continue
		}
		if inEsc {
			inEsc = false
			if c == 127 || c == 8 {
				buf = deleteWord(buf, true)
			} else if c == '[' {
				inCSI = true
			} else if c == 'O' {
				inSS3 = true
			}
			continue
		}
		
		switch c {
		case 3:
			fmt.Print("^C\r\n")
			return "", true
		case 4, '\r', '\n':
			fmt.Print("\r\n")
			return string(buf), false
		case 127, 8:
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
			}
		case 23:
			buf = deleteWord(buf, true)
		case 21:
			buf = deleteLine(buf, true)
		case 27:
			inEsc = true
		default:
			if c >= 32 && c <= 126 {
				buf = append(buf, c)
			}
		}
	}
	return string(buf), false
}

// ── Prompt helpers ───────────────────────────────────────────────────────────

// ask prompts for a required string field.
func ask(label string) (string, bool) {
	fmt.Printf("  %s %s: ", pterm.LightYellow("▸"), pterm.Bold.Sprint(label))
	return readLine()
}

// askDefault prompts for an optional field; returns defaultVal if user presses Enter.
func askDefault(label, defaultVal string) (string, bool) {
	fmt.Printf("  %s %s %s: ",
		pterm.LightYellow("▸"),
		pterm.Bold.Sprint(label),
		pterm.FgGray.Sprintf("[default: %s]", defaultVal),
	)
	text, cancelled := readLine()
	if cancelled {
		return "", true
	}
	if text == "" {
		return defaultVal, false
	}
	return text, false
}

// askOptional prompts for an optional field.
func askOptional(label string) (string, bool) {
	fmt.Printf("  %s %s %s: ",
		pterm.LightYellow("▸"),
		pterm.Bold.Sprint(label),
		pterm.FgGray.Sprint("(optional)"),
	)
	return readLine()
}

// askSecretOptional prompts for an optional secret field.
func askSecretOptional(label string) (string, bool) {
	fmt.Printf("  %s %s %s: ",
		pterm.LightYellow("▸"),
		pterm.Bold.Sprint(label),
		pterm.FgGray.Sprint("(optional, hidden)"),
	)
	return readLineHidden()
}

// askInt prompts for a required integer field.
func askInt(label string) (int, bool) {
	raw, cancelled := ask(label)
	if cancelled {
		return 0, true
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		pterm.Error.Printf("'%s' is not a valid number.\n", raw)
		return 0, false
	}
	return val, false
}

// askIntDefault prompts for an optional integer field with a default value.
func askIntDefault(label string, defaultVal int) (int, bool) {
	raw, cancelled := askDefault(label, strconv.Itoa(defaultVal))
	if cancelled {
		return 0, true
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal, false
	}
	return val, false
}

// askConfirm asks a yes/no question. Returns (true, false) for "yes".
func askConfirm(question string) (bool, bool) {
	fmt.Printf("  %s %s %s: ",
		pterm.FgRed.Sprint("⚠"),
		pterm.Bold.Sprint(question),
		pterm.FgGray.Sprint("[y/N]"),
	)
	text, cancelled := readLine()
	if cancelled {
		return false, true
	}
	return strings.ToLower(text) == "y" || strings.ToLower(text) == "yes", false
}

// printFormHeader prints a stylized section header before an interactive form.
func printFormHeader(title string) {
	fmt.Println()
	pterm.DefaultSection.WithStyle(pterm.NewStyle(pterm.FgLightYellow)).Println(title)
	pterm.FgGray.Println("  Press Ctrl+C to cancel at any time.\n")
}
