package rtprompt

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/coxley/keyboard"
	"golang.org/x/term"
)

// Prompt lets a user type input that is sent to a callback in realtime
//
// Callback is only invoked when the input changes or <TAB> is pressed. The
// return string is shown below the prompt and rewritten when changed.
//
// This allows for a reactive CLI that isn't full-on curses/TUI. It should feel
// like a normal tool.
type Prompt struct {
	// What the user sees in front of their input
	Prefix string

	// Receives the input value and whether a tab was just pressed
	//
	// If callback returns a value after input has changed again, it'll be
	// dropped.
	Callback func(s string, tab bool) string

	// Lines between the prompt and output from callback (default: 2)
	Padding int

	// Keep the value of text and pos variables on screen
	Debug bool

	text string
	pos  int  // position of cursor
	tab  bool // Did the user just press tab?
}

// New prompt instance w/ sane defaults
func New(pfx string, callback func(string, bool) string) *Prompt {
	return &Prompt{
		Prefix:   pfx,
		Callback: callback,
		Padding:  2,
		Debug:    false,
	}
}

// callbacks might want to do some action on TAB, like cycle through options
type textResult struct {
	text string
	tab  bool
}

// Start begins the prompt and feeds updates into the callback until Enter is pressed.
//
// From when this starts, until it's finished, the terminal is put into raw
// mode. If you write to stdout/stderr during that time, it will throw off the
// formatting; badly.
//
// Most of the logic making it feel like a normal prompt is from careful
// repositioning of the ANSI cursor.
//
// Return channel given a single value, and closed, on Enter.
func (p *Prompt) Start() chan struct{} {
	if p.Callback == nil {
		p.Callback = func(string, bool) string { return "" }
	}

	textCh := make(chan textResult)
	go p.readInput(textCh)

	// No need to expose internal text channel. Sync single value channel with
	// the cancellation of textCh.
	done := make(chan struct{}, 1)
	go func() {
		for _ = range textCh {
			continue
		}
		done <- struct{}{}
		close(done)
	}()
	return done
}

// Set terminal mode to raw, set up goroutines, and start reading keypresses
func (p *Prompt) readInput(textCh chan textResult) {

	// Terminal must be set to raw mode
	oldState, err := term.MakeRaw(0)
	if err != nil {
		panic(err)
	}

	// Restore terminal *before* closing the channel. Can fuck up prompt
	// otherwise. (defers are LIFO)
	defer close(textCh)
	defer term.Restore(0, oldState)

	keyCh, err := keyboard.GetKeys(10)
	if err != nil {
		panic(err)
	}
	defer keyboard.Close()

	// Wrap callback in a goroutine so we don't throttle user typing on its
	// performance.
	type cbResult struct {
		// This lets us know if the result is expired or not
		ts     time.Time
		output string
	}
	callbackCh := make(chan cbResult)
	go func() {
		for res := range textCh {
			callbackCh <- cbResult{time.Now(), p.Callback(res.text, res.tab)}
		}
	}()

	fmt.Printf(p.Prefix) // Prompt statement
	var lastUpdate time.Time
	for {
		select {
		case e := <-keyCh:
			if e.Err != nil {
				p.print(fmt.Sprintf("error: %+v", e), 10)
			}

			oldText := p.text
			if !p.handleKey(e, func() { term.Restore(0, oldState) }) {
				return
			}

			// Don't update callback for text navigation. (arrow keys, etc)
			if p.text == oldText && e.Key != keyboard.KeyTab {
				continue
			}

			lastUpdate = time.Now()
			textCh <- textResult{p.text, e.Key == keyboard.KeyTab}
		case res := <-callbackCh:
			// New text entered before we could read the result
			if res.ts.Before(lastUpdate) {
				continue
			}
			p.print(res.output, p.Padding)
		}
	}
}

// Print text with 'padding' lines beneath the prompt. Returns cursor to original position.
//
// If we don't have enough room below the prompt, newlines are printed until we
// do. Whereas curses and other terminal UIs will clear the entire screen, this
// tries to be inconspicuous.
//
// Padded lines are not cleared. This allows us to print some output 10 lines
// below (eg: debug info) and some other at 2 lines below.
func (p *Prompt) print(s string, padding int) {
	if s == "" {
		return
	}
	// Create padding lines, but don't clear as there's no content.
	for i := 0; i < padding; i++ {
		fmt.Printf("\n")
	}
	// Create enough space for the output.
	//
	// If there are more lines to print than available between the prompt and
	// terminal bottom, we won't be able to get the cursor back to where the
	// user is typing.
	//
	// For each line we create, clear it to allow new text to replace it
	// entirely.
	linecnt := strings.Count(s, "\n")
	clearLine() // otherwise the first line of 's' won't be a clean slate
	for i := 0; i < linecnt; i++ {
		fmt.Printf("\n")
		clearLine()
	}

	// Go back to where we started, and save it.
	p.cursorUp(linecnt + padding)
	saveCursor()

	// \r in ANSI moves cursor to beginning of current line. Default goes down
	// a row without changing column position.
	//
	// \n without \r will make paragraphs look like a waterfall. Not a typo for
	// CRLF
	fmt.Print(strings.Repeat("\n\r", padding))
	fmt.Print(strings.ReplaceAll(s, "\n", "\n\r"))
	restoreCursor()
}

// Handles a single key press, and calls cleanup+SIGINT on ^C
//
// This tries to simulate most of the keybindings from Linux's line discipline.
// Becuase we're in raw mode, we have to do it ourselves. And because the user
// is typing text, these are important.
//
// A false return means we should exit the prompt.
func (p *Prompt) handleKey(key keyboard.KeyEvent, cleanup func()) bool {

	switch key.Key {
	case keyboard.KeyEsc:
		// Raw escape
		if key.Rune == 0 {
			return false
		}
		// Alt combos show up as ESC<letter>
		switch key.Rune {
		case 'b':
			// back a word
			p.cursorLeft(p.pos - p.lastWordIndex() - 1)
		case 'f':
			// forward a word
			p.print(fmt.Sprintf("next word: %d", p.nextWordIndex()), 4)
			p.cursorRight(p.nextWordIndex() - p.pos + 1)
		}
	case keyboard.KeyEnter:
		return false
	case keyboard.KeyCtrlC:
		cleanup()
		// always succeeds on UNIX systems
		p, _ := os.FindProcess(os.Getpid())
		p.Signal(os.Interrupt)
	case keyboard.KeyArrowLeft, keyboard.KeyCtrlB:
		p.cursorLeft(1)
	case keyboard.KeyArrowRight, keyboard.KeyCtrlF:
		p.cursorRight(1)
	case keyboard.KeyBackspace, keyboard.KeyBackspace2:
		p.backspace(1)
	case keyboard.KeyDelete, keyboard.KeyCtrlD:
		p.del(1)
	case keyboard.KeyCtrlA, keyboard.KeyHome:
		// Beginning of line
		p.cursorLeft(p.pos)
	case keyboard.KeyCtrlE, keyboard.KeyEnd:
		// End of line
		p.cursorRight(len(p.text) - p.pos)
	case keyboard.KeyCtrlU:
		// Remove text before cursor
		p.backspace(p.pos)
	case keyboard.KeyCtrlK:
		// Remove text from cursor to EOL
		p.del(len(p.text) - p.pos)
	case keyboard.KeyCtrlW:
		p.backspace(p.pos - p.lastWordIndex() - 1)
	case keyboard.KeySpace:
		p.advance(" ")
	default:
		// Do nothing without letter/digit
		if key.Rune == 0 {
			break
		}
		p.advance(string(key.Rune))
	}

	if p.Debug {
		debugText := fmt.Sprintf("text=%v\npos=%v\n", p.text, p.pos)
		p.print(debugText, 10)
	}
	return true
}

// move and update position of cursor
func (p *Prompt) cursorLeft(n int) {
	// At bounds, no-op
	if p.pos == 0 {
		return
	}
	fmt.Printf("\033[%dD", n)
	p.pos -= n
}

// move and update position of cursor
func (p *Prompt) cursorRight(n int) {
	// At bounds, no-op
	if p.pos == len(p.text) {
		return
	}
	fmt.Printf("\033[%dC", n)
	p.pos += n
}

// move cursor up in the same column
func (p *Prompt) cursorUp(n int) {
	fmt.Printf("\033[%dA", n)
}

// delete text behind the cursor
func (p *Prompt) backspace(n int) {
	if n == 0 {
		return
	}

	// nothing to delete
	if p.pos == 0 {
		return
	}

	oldPos := p.pos
	newPos := oldPos - n
	start := p.text[:newPos]
	end := p.text[oldPos:]

	// Clear all text from new position to old position
	p.cursorLeft(n)
	saveCursor()
	eraseFromCursor()

	fmt.Printf(end)
	p.text = start + end
	p.pos = newPos
	restoreCursor()
}

// delete text in front of the cursor
func (p *Prompt) del(n int) {
	if n == 0 {
		return
	}

	// nothing to delete
	if p.pos == len(p.text) {
		return
	}

	start := p.text[:p.pos]
	end := p.text[p.pos+n:]
	saveCursor()

	// Clear all text from new position to old position
	eraseFromCursor()
	fmt.Printf(end)
	p.text = start + end

	restoreCursor()
}

// add text in front of the prompt and advance the cursor's position
func (p *Prompt) advance(s string) {
	if s == "" {
		return
	}

	// Prompt is adding text to the end
	if p.pos == len(p.text) {
		p.text += s
		p.pos += len(s)
		fmt.Printf(s)
		return
	}

	// Cursor is in the middle of the string. Divide into two parts.
	before := p.text[:p.pos]
	after := p.text[p.pos:]

	// Clear all text from cursor onward, replace with modified text, and
	// advance cursor.
	saveCursor()
	eraseFromCursor()
	fmt.Printf(s + after)
	p.text = before + s + after
	restoreCursor()
	p.cursorRight(1)
}

// erase everything on the line in front of the cursor
func eraseFromCursor() {
	fmt.Printf("\033[K")
}

// clear entire line without changing cursor position
func clearLine() {
	fmt.Printf("\033[2K")
}

func saveCursor() {
	fmt.Printf("\033[s")
}

func restoreCursor() {
	fmt.Printf("\033[u")
}

// Look for closest space before current position (or start)
//
// This is used for move/remove back a word actions.
func (p *Prompt) lastWordIndex() int {
	// Trim right-space so that "this is a test " -> "this is a "
	return strings.LastIndex(strings.TrimRight(p.text[:p.pos], " "), " ")
}

// Look for closest space after the current position (or end)
//
// This is used for move/remove forward a word actions.
func (p *Prompt) nextWordIndex() int {
	// Split at position, but add index to the final value
	beforeLen := len(p.text[:p.pos])
	fwd := p.text[p.pos:]

	if i := strings.Index(fwd, " "); i != 0 {
		return i + beforeLen
	} else if i == -1 {
		return 0
	}

	// Cursor is at a space. Trim so we can find the closest word after.
	trimmed := strings.TrimLeft(fwd, " ")
	spaceCnt := len(fwd) - len(trimmed)
	return strings.Index(fwd, " ") + spaceCnt + beforeLen
}
