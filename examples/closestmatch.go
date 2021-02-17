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
