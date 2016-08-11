package util

import "regexp"

func EscapeMarkdown(s string) string {
	// TODO a regex to escape links as well
	return regexp.MustCompile("[\\*_`]").ReplaceAllStringFunc(s,
		func(match string) string {
			return "\\" + match
		})
}
