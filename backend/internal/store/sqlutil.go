package store

import "strconv"

// placeholder returns the Nth ($1, $2, ...) positional parameter marker,
// used when building dynamic UPDATE statements for partial-update endpoints.
func placeholder(n int) string {
	return "$" + strconv.Itoa(n)
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
