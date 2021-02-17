package main

import (
	"fmt"

	"github.com/coxley/rtprompt"
)

func main() {
	issues := map[string]string{}
	var selected string
	cb := rtprompt.ClosestMatch{
		Data:             issues,
		OnSelect:         func(s string) { selected = s },
		MaxShown:         7,
		ShowInstructions: true,
	}
	prompt := rtprompt.New("Summary: ", cb.CB())
	prompt.Wait()
	fmt.Printf("Woohoo! You selected: %s\n", selected)
}
