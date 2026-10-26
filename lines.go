// SPDX-License-Identifier: MIT
/*
 * Copyright (c) 2026, SCANOSS
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 */

package snippetclassifier

import (
	"strings"
	"unicode/utf8"
)

// SplitLines decodes data as UTF-8, replacing invalid bytes with U+FFFD, and
// splits it the way Python's str.splitlines() does: on \n, \r, \r\n, \v, \f,
// \x1c, \x1d, \x1e, \x85, U+2028 and U+2029, without a trailing empty line.
// Line numbers in a Span index into this slice (1-based).
func SplitLines(data []byte) []string {
	var (
		out []string
		b   strings.Builder
	)
	flush := func() {
		out = append(out, b.String())
		b.Reset()
	}
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size <= 1 {
			b.WriteRune(utf8.RuneError)
			i += invalidRun(data[i:])
			continue
		}
		i += size
		switch r {
		case '\r':
			if i < len(data) && data[i] == '\n' {
				i++
			}
			flush()
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			flush()
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		flush()
	}
	return out
}

// invalidRun is how many bytes Python's "replace" error handler consumes for
// one U+FFFD: the maximal prefix of a well-formed sequence, at least one byte.
func invalidRun(p []byte) int {
	lead := p[0]
	var need int
	var lo, hi byte = 0x80, 0xbf
	switch {
	case lead >= 0xc2 && lead <= 0xdf:
		need = 1
	case lead == 0xe0:
		need, lo = 2, 0xa0
	case lead >= 0xe1 && lead <= 0xec, lead == 0xee, lead == 0xef:
		need = 2
	case lead == 0xed:
		need, hi = 2, 0x9f
	case lead == 0xf0:
		need, lo = 3, 0x90
	case lead >= 0xf1 && lead <= 0xf3:
		need = 3
	case lead == 0xf4:
		need, hi = 3, 0x8f
	default:
		return 1
	}
	n := 1
	for k := 0; k < need && n < len(p); k++ {
		c := p[n]
		if k == 0 {
			if c < lo || c > hi {
				break
			}
		} else if c < 0x80 || c > 0xbf {
			break
		}
		n++
	}
	return n
}
