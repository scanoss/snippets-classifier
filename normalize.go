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
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// This file ports the line normalisation of the Python training code, the
// reference the model was trained on. The golden tests here and the corpus
// parity test in the training repository hold the two together, so change
// both or neither.

var (
	// Python `\w` is Unicode-aware; RE2's is ASCII only, hence the explicit
	// classes below.
	tokenRe      = regexp.MustCompile("[A-Za-z_$][\\p{L}\\p{N}_$]*|\\p{Nd}+(?:\\.\\p{Nd}+)?|\"(?:\\\\.|[^\"\\\\])*\"|'(?:\\\\.|[^'\\\\])*'|`[^`]*`|\\S")
	punctOnlyRe  = regexp.MustCompile(`^[\s{}()\[\];,.:<>]*$`)
	importishRe  = regexp.MustCompile(`^(import\b|package\b|from\s+\S+\s+import\b|#\s*include\b|using\s+[\p{L}\p{N}_.]+;|(const|let|var)\s+[\p{L}\p{N}_{},\s]+=\s*require\(|export\s+\*\s+from\b|export\s+\{[^}]*\}\s+from\b|"[\p{L}\p{N}_./\-]+"$|[\p{L}\p{N}_]+\s+"[\p{L}\p{N}_./\-]+"$)`)
	trailingSep  = regexp.MustCompile(`[;,]+$`)
	preprocessor = []string{"include", "define", "if", "else", "endif", "pragma", "undef"}
)

// abstractKeep are the tokens abstract() keeps verbatim; every other
// identifier becomes "I".
var abstractKeep = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`if else for while do switch case break continue return new throw try catch finally
function def class struct func var let const public private protected static final void int long char byte
bool boolean float double string null nil none true false this self await async yield in of is not and or`) {
		abstractKeep[w] = true
	}
}

// norm is the whitespace/case-insensitive key of a line. Trailing `;`/`,` and a
// leading `_` on identifiers are dropped too: formatters and linters add or
// remove them without changing the code.
func norm(line string) string {
	n := strings.Join(pyFields(pyLower(line)), " ")
	n = trailingSep.ReplaceAllString(n, "")
	return stripLeadingUnderscores(n)
}

// stripLeadingUnderscores removes a run of `_` that starts an identifier and is
// followed by an ASCII letter: Python's (?<![\w$])_+(?=[A-Za-z]), which RE2
// cannot express.
func stripLeadingUnderscores(s string) string {
	if !strings.Contains(s, "_") {
		return s
	}
	rs := []rune(s)
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(rs); {
		if rs[i] == '_' && (i == 0 || !(isWord(rs[i-1]) || rs[i-1] == '$')) {
			j := i
			for j < len(rs) && rs[j] == '_' {
				j++
			}
			if j < len(rs) && isASCIILetter(rs[j]) {
				i = j
				continue
			}
			for ; i < j; i++ {
				b.WriteRune('_')
			}
			continue
		}
		b.WriteRune(rs[i])
		i++
	}
	return b.String()
}

// informative returns the normalised line, or "" for a trivial one: blank,
// punctuation only, a comment, or an import/include/package line.
func informative(line string) string {
	n := norm(line)
	if n == "" || punctOnlyRe.MatchString(n) || isComment(n) || importishRe.MatchString(n) {
		return ""
	}
	return n
}

// isComment ports ^(//|/\*|\*|\*/|#(?!include|define|if|else|endif|pragma|undef)|--|<!--).
func isComment(n string) bool {
	switch {
	case strings.HasPrefix(n, "//"), strings.HasPrefix(n, "/*"), strings.HasPrefix(n, "*"),
		strings.HasPrefix(n, "--"), strings.HasPrefix(n, "<!--"):
		return true
	case strings.HasPrefix(n, "#"):
		rest := n[1:]
		for _, p := range preprocessor {
			if strings.HasPrefix(rest, p) {
				return false
			}
		}
		return true
	}
	return false
}

func tokens(nline string) []string {
	return tokenRe.FindAllString(nline, -1)
}

// abstract maps a token to its shape: string and number literals to S and N,
// identifiers other than keywords to I, punctuation unchanged.
func abstract(tok string) string {
	r, _ := utf8.DecodeRuneInString(tok)
	switch {
	case r == '"' || r == '\'' || r == '`':
		return "S"
	case pyIsDigit(r):
		return "N"
	case unicode.IsLetter(r) || r == '_' || r == '$':
		if abstractKeep[tok] {
			return tok
		}
		return "I"
	}
	return tok
}

func isWord(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) }

func isASCIILetter(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }

// pyIsDigit approximates str.isdigit(): decimal digits plus the superscript,
// subscript and circled digit forms Python also accepts.
func pyIsDigit(r rune) bool {
	switch {
	case unicode.IsDigit(r):
		return true
	case r == 0xb2, r == 0xb3, r == 0xb9, r >= 0x2070 && r <= 0x2079, r >= 0x2080 && r <= 0x2089,
		r >= 0x2460 && r <= 0x2468, r >= 0x2474 && r <= 0x247c, r >= 0x2488 && r <= 0x2490:
		return true
	}
	return false
}

// pyLower is str.lower().
func pyLower(s string) string { return strings.ToLower(s) }

// pyFields is str.split() with no argument: Python also treats the
// information separators \x1c-\x1f as whitespace.
func pyFields(s string) []string {
	return strings.FieldsFunc(s, pyIsSpace)
}

func pyIsSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}
