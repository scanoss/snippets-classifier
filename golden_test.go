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
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// The golden files are written from the Python reference implementation, which
// lives with the training code. Regenerate them there, never by hand.

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func TestMatchingBlocksMatchesPythonDifflib(t *testing.T) {
	var cases []struct {
		A, B   []string
		Blocks [][3]int
	}
	readJSON(t, "testdata/difflib.golden.json", &cases)
	for i, c := range cases {
		got := matchingBlocks(c.A, c.B)
		want := make([]block, len(c.Blocks))
		for k, b := range c.Blocks {
			want[k] = block{b[0], b[1], b[2]}
		}
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("case %d a=%v b=%v:\n got  %v\n want %v", i, c.A, c.B, got, want)
		}
	}
}

func TestNormalisationMatchesPython(t *testing.T) {
	var cases []struct {
		Line, Norm, Informative string
		Tokens, Abstract        []string
	}
	readJSON(t, "testdata/normalize.golden.json", &cases)
	for _, c := range cases {
		if got := norm(c.Line); got != c.Norm {
			t.Errorf("norm(%q) = %q, want %q", c.Line, got, c.Norm)
		}
		if got := informative(c.Line); got != c.Informative {
			t.Errorf("informative(%q) = %q, want %q", c.Line, got, c.Informative)
		}
		toks := tokens(c.Norm)
		if len(toks) == 0 && len(c.Tokens) == 0 {
			continue
		}
		if !reflect.DeepEqual(toks, c.Tokens) {
			t.Errorf("tokens(%q) = %q, want %q", c.Norm, toks, c.Tokens)
		}
		if got := abstractTokens(toks); !reflect.DeepEqual(got, c.Abstract) {
			t.Errorf("abstract(%q) = %q, want %q", c.Norm, got, c.Abstract)
		}
	}
}

func TestSplitLinesMatchesPython(t *testing.T) {
	var g struct {
		Input []byte `json:"-"`
		Raw   []int  `json:"input"`
		Lines []string
	}
	readJSON(t, "testdata/splitlines.golden.json", &g)
	in := make([]byte, len(g.Raw))
	for i, b := range g.Raw {
		in[i] = byte(b)
	}
	if got := SplitLines(in); !reflect.DeepEqual(got, g.Lines) {
		t.Fatalf("SplitLines:\n got  %q\n want %q", got, g.Lines)
	}
}
