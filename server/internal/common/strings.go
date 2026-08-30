package common

import "strings"

func TruncateString(str string, n int) string {
	words := strings.Fields(str)

	if len(words) <= n {
		return str
	}

	return strings.Join(words[:n], " ")
}
