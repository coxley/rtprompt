package rtprompt

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/schollz/closestmatch"
)

// ClosestMatch configures a callback to let users select from a set of options
//
// Data's keys are used as titles, and shown on-screen. The values can be used
// to give additional ranking. For example, you might want to show ticket names
// but use their summaries for more accurate matching.
//
// onSelect will be called when user presses Enter. If they made a selection,
// the string arg will be equal to one of the keys in Data. Otherwise it's the
// raw value of the prompt.
type ClosestMatch struct {
	Data     map[string]string
	OnSelect func(string)
	MaxShown int

	// default: "Use <TAB> and <ENTER> to select from below. Otherwise press <ENTER> when ready"
	Instructions     string
	ShowInstructions bool

	// default: FgBlue
	SelectedColor *color.Color
	// default: FgHiBlack
	InstructionColor *color.Color
}

// CB returns a configured callback to use with Prompt
func (c *ClosestMatch) CB() Callback {
	// We want to add context to closestmatch, but separate the titles back
	// later
	delim := "::CBDELIM::"

	if c.SelectedColor == nil {
		c.SelectedColor = color.New(color.FgBlue)
	}
	if c.InstructionColor == nil {
		c.InstructionColor = color.New(color.FgHiBlack)
	}
	if c.Instructions == "" {
		c.Instructions = "Use <TAB> and <ENTER> to select from below. Otherwise press <ENTER> when ready"
	}

	var content []string
	for k, v := range c.Data {
		content = append(content, strings.Join([]string{k, v}, delim))
	}

	bagSizes := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	cm := closestmatch.New(content, bagSizes)

	// Only recompute when needed instead of every callback invocation
	var topN []string
	lastSelected := -1
	return func(inp string, tab bool, enter bool) string {
		preproc := func(s string) string { return strings.Split(s, delim)[0] }

		if enter {
			if lastSelected != -1 {
				inp = preproc(topN[lastSelected])
			}
			c.OnSelect(inp)
			return ""
		}

		if !tab {
			lastSelected = -1
		}

		// Tab control. Support cycling thru
		if tab {
			if lastSelected < min(len(content), c.MaxShown)-1 {
				lastSelected++
			} else {
				lastSelected = 0
			}
		}

		// When content is empty, don't show anything but the prompt
		if len(content) == 0 {
			return ""
		}

		if inp == "" { // first load or backspaced text
			topN = content[:min(len(content), c.MaxShown)]
		} else if !tab {
			// Only repopulate topN on non-tab entries
			topN = cm.ClosestN(strings.ToLower(inp), c.MaxShown)
		}
		return c.joinLines(
			topN,
			preproc,
			lastSelected,
		)
	}
}

func (c *ClosestMatch) joinLines(lines []string, preproc func(string) string, selected int) string {
	var output string
	if c.ShowInstructions {
		output += c.InstructionColor.Sprintln(c.Instructions)
	}
	for i, line := range lines {
		line = preproc(line)

		// Nothing has been selected yet
		if i == 0 && selected == -1 {
			output += fmt.Sprintln(line)
			continue
		}

		// Color and note current selection
		if selected > -1 && i == selected {
			output += fmt.Sprintf("%s (selected)\n", c.SelectedColor.Sprint(line))
			continue
		}

		// Plain jane text
		output += fmt.Sprintln(line)
	}
	return output
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
