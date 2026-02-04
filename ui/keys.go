package ui

import (
	"os"
	"time"
)

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

// escTimeout is how long to wait for more bytes after receiving ESC.
// Terminal escape sequences may arrive in split reads; this timeout
// distinguishes a bare Escape press from an incomplete sequence.
var escTimeout = 50 * time.Millisecond

// readKey reads a single key event from a raw-mode file descriptor.
// It detects escape sequences for arrow keys and returns keyRune + the
// rune for printable characters. On read errors it returns keyEsc so
// callers exit cleanly instead of spinning.
//
// When a lone ESC byte is read, readKey waits briefly (escTimeout) for
// additional bytes that would form an escape sequence (e.g. ESC [ A for
// arrow up). This handles the common case where the OS delivers the
// multi-byte sequence across split reads.
func readKey(f *os.File) (keyEvent, rune) {
	var buf [3]byte
	n, err := f.Read(buf[:])
	if err != nil || n == 0 {
		return keyEsc, 0
	}

	// If all 3 bytes arrived at once, check for escape sequence first.
	// Handles both CSI (ESC [) and SS3 (ESC O) forms — the latter is sent
	// when the terminal is in application cursor mode (DECCKM set).
	if n == 3 && buf[0] == 27 && (buf[1] == '[' || buf[1] == 'O') {
		return escSeqKey(buf[2])
	}

	// If 2 bytes arrived starting with ESC + '[' or ESC + 'O', read one more.
	if n == 2 && buf[0] == 27 && (buf[1] == '[' || buf[1] == 'O') {
		var third [1]byte
		f.SetReadDeadline(time.Now().Add(escTimeout))
		n2, err2 := f.Read(third[:])
		f.SetReadDeadline(time.Time{})
		if err2 == nil && n2 == 1 {
			return escSeqKey(third[0])
		}
		return keyNone, 0
	}

	if n >= 1 {
		switch {
		case buf[0] == 3: // ctrl+c
			return keyCtrlC, 0
		case buf[0] == 13 || buf[0] == 10: // enter
			return keyEnter, 0
		case buf[0] == 27: // ESC — might be start of a sequence
			// Wait briefly for follow-up bytes.
			var seq [2]byte
			f.SetReadDeadline(time.Now().Add(escTimeout))
			n2, err2 := f.Read(seq[:])
			f.SetReadDeadline(time.Time{})
			if err2 != nil || n2 == 0 {
				return keyEsc, 0 // bare escape
			}
			if n2 >= 2 && (seq[0] == '[' || seq[0] == 'O') {
				return escSeqKey(seq[1])
			}
			if n2 == 1 && (seq[0] == '[' || seq[0] == 'O') {
				// Got ESC + '[' or ESC + 'O', need one more byte.
				var third [1]byte
				f.SetReadDeadline(time.Now().Add(escTimeout))
				n3, err3 := f.Read(third[:])
				f.SetReadDeadline(time.Time{})
				if err3 == nil && n3 == 1 {
					return escSeqKey(third[0])
				}
				return keyNone, 0
			}
			return keyNone, 0
		case buf[0] >= 32 && buf[0] < 127:
			return keyRune, rune(buf[0])
		}
	}
	return keyNone, 0
}

// escSeqKey maps the final byte of an arrow key sequence to a key event.
// Works for both CSI (ESC [ X) and SS3 (ESC O X) forms.
func escSeqKey(b byte) (keyEvent, rune) {
	switch b {
	case 'A':
		return keyUp, 0
	case 'B':
		return keyDown, 0
	}
	return keyNone, 0
}
