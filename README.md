# Realtime Prompt (`rtprompt`)

[![GoDoc](https://img.shields.io/badge/pkg.go.dev-doc-blue)](http://pkg.go.dev/github.com/coxley/rtprompt)

A command-line prompt that looks normal, but can show results as users type.

![Basic Example](https://raw.githubusercontent.com/coxley/rtprompt/master/examples/basic2.gif)

# Why?

I wanted a way to have auto-updated results in the terminal as a user typed --
without the experience that curses brings. I'm not a fan of how terminal UIs
feel like opening a new window.

Turns out this didn't exist, and wasn't nearly as straight forward as I had
hoped. We have to put the terminal into raw mode to get unbuffered input, but
because it's a text prompt we need to recreate the keybindings that Linux's
terminal discipline uses.

# Example (Basic)

Here's the code for the example above. It's a silly thing that counts the words
that have been typed. But it demonstrates that we can have output update in
real-time without doing a full-screen terminal UI.

Navigation shortcuts are demonstrated too.

```go
package main

import (
  "fmt"

  "github.com/coxley/rtprompt"
)

func callback(inp string, tab bool, enter bool) string {
  return fmt.Sprintf("Words typed: %d", len(strings.Fields(inp)))
}

func main() {
  prompt := rtprompt.New("Input: ", callback)
  prompt.Wait()
  fmt.Println("Glad we're done with that")
}
```

# Example (Closest Match)

![Closest Match Example](https://raw.githubusercontent.com/coxley/rtprompt/master/examples/closestmatch.gif)

This one is more involved. It's also why I wrote this lib. I wanted users, when
reporting a bug, have a list of open tasks shown that get updated as they type.

We use https://github.com/schollz/closestmatch for ranking, support tab to
select items, colors for selected items, and a function to invoke when the user
presses enter.

```go
package main

import (
  "fmt"

  "github.com/coxley/rtprompt"
)

func main() {
  issues := map[string]string{
    "[#1011] LSPs too optimized":                               "",
    "[#1112] Dry runs too dry":                                 "",
    "[#1213] All hands on deck":                                "",
    "[#1314] Leak in the Enterprise":                           "",
    "[#1415] Exception thrown during cardio":                   "",
    "[#1516] Panic when frying eggs":                           "",
    "[#1617] Frying eggs when panicking":                       "",
    "[#1618] Nothing to report, just lonely":                   "",
    "[#1619] Bloody onions when expecting uncontaminated ones": "",
  }

  var selected string
  cm := rtprompt.ClosestMatch{
    Data:     issues,
    OnSelect: func(s string) { selected = s },
    MaxShown: 7,
  }
  prompt := rtprompt.New("Summary: ", cm.CB())
  prompt.Wait()
  fmt.Printf("Woohoo! You selected: %s\n", selected)
}
```


## Supported Keybinds

These should cover most use-cases but happy to expand them.

* CTRL+A, HOME: Beginning of line
* CTRL+B, <-: Move left
* CTRL-C: SIGINT
* CTRL+F, ->: Move right
* CTRL+D: Delete forward
* CTRL+E, END: End of line
* CTRL+U: Remove all text before cursor
* CTRL+K: Remove all text after cursor
* CTRL+W: Remove the previous word
* ALT+B: Go back a word
* ALT+F: Go forward a word
* Enter: Finish input
