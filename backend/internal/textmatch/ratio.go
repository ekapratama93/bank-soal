package textmatch

// ratio reimplements Python difflib.SequenceMatcher(None, a, b).ratio():
// find the longest matching block, recurse on the parts before/after it,
// and score as 2*M / T where M is the total length of all matching blocks
// and T is the combined length of both strings. Junk/autojunk handling is
// intentionally omitted — Python only applies autojunk when len(b) >= 200,
// which never happens for these short answer strings, so the plain
// algorithm below produces identical results.
func ratio(a, b string) float64 {
	ra := []rune(a)
	rb := []rune(b)
	if len(ra) == 0 && len(rb) == 0 {
		return 1.0
	}
	matches := matchingBlockSize(ra, rb, 0, len(ra), 0, len(rb))
	return 2.0 * float64(matches) / float64(len(ra)+len(rb))
}

func matchingBlockSize(a, b []rune, alo, ahi, blo, bhi int) int {
	i, j, size := longestMatch(a, b, alo, ahi, blo, bhi)
	if size == 0 {
		return 0
	}
	total := size
	if alo < i && blo < j {
		total += matchingBlockSize(a, b, alo, i, blo, j)
	}
	if i+size < ahi && j+size < bhi {
		total += matchingBlockSize(a, b, i+size, ahi, j+size, bhi)
	}
	return total
}

// longestMatch finds the longest common substring of a[alo:ahi] and
// b[blo:bhi] via a rolling dynamic-programming table (O(n*m), fine for
// short answer strings), returning its start indices and length.
func longestMatch(a, b []rune, alo, ahi, blo, bhi int) (besti, bestj, bestsize int) {
	width := bhi - blo
	if width <= 0 || ahi <= alo {
		return alo, blo, 0
	}
	prev := make([]int, width)
	for i := alo; i < ahi; i++ {
		cur := make([]int, width)
		for j := blo; j < bhi; j++ {
			if a[i] == b[j] {
				v := 1
				if j > blo {
					v = prev[j-blo-1] + 1
				}
				cur[j-blo] = v
				if v > bestsize {
					bestsize = v
					besti = i - v + 1
					bestj = j - v + 1
				}
			}
		}
		prev = cur
	}
	if bestsize == 0 {
		besti, bestj = alo, blo
	}
	return besti, bestj, bestsize
}
