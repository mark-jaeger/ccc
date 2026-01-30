package ui

import "os"

// keyEvent represents a key press from raw terminal input.
type keyEvent int

const (
	keyNone  keyEvent = iota
	keyUp             // arrow up
	keyDown           // arrow down
	keyEnter          // enter / return
	keyEsc            // bare escape (no sequence following)
	keyCtrlC          // ctrl+c
	keyRune           // printable character (see accompanying rune)
)

// readKey reads a single key event from a raw-mode file descriptor.
// It detects escape sequences for arrow keys and returns keyRune + the
// rune for printable characters. On read errors it returns keyEsc so
// callers exit cleanly instead of spinning.
func readKey(f *os.File) (keyEvent, rune) {
	var buf [3]byte
	n, err := f.Read(buf[:])
	if err != nil || n == 0 {
		return keyEsc, 0
	}

	switch {
	case n == 1 && buf[0] == 3: // ctrl+c
		return keyCtrlC, 0
	case n == 1 && (buf[0] == 13 || buf[0] == 10): // enter
		return keyEnter, 0
	case n == 1 && buf[0] == 27: // bare escape
		return keyEsc, 0
	case n == 3 && buf[0] == 27 && buf[1] == '[':
		switch buf[2] {
		case 'A':
			return keyUp, 0
		case 'B':
			return keyDown, 0
		}
		return keyNone, 0
	case n == 1 && buf[0] >= 32 && buf[0] < 127:
		return keyRune, rune(buf[0])
	}
	return keyNone, 0
}
