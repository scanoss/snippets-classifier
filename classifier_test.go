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
	"errors"
	"testing"
)

func TestEmbeddedModelIsValid(t *testing.T) {
	m := DefaultModel()
	if m.Version == "" || len(m.Features) == 0 {
		t.Fatalf("embedded model incomplete: %+v", m)
	}
}

func TestClassifyRequiresSpans(t *testing.T) {
	c, _ := New()
	if _, err := c.Classify(Match{Local: []byte("a"), OSS: []byte("a")}); !errors.Is(err, ErrNoSpans) {
		t.Fatalf("want ErrNoSpans, got %v", err)
	}
}

func TestIdenticalRangesAreReal(t *testing.T) {
	code := `func Sum(xs []int) int {
	total := 0
	for _, x := range xs {
		total += x
	}
	if total < 0 {
		panic("negative total")
	}
	return total
}

func Mean(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	return float64(Sum(xs)) / float64(len(xs))
}
`
	c, _ := New()
	r, err := c.Classify(Match{Local: []byte(code), OSS: []byte(code), Spans: []Span{{1, 17, 1, 17}}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Verdict != VerdictReal {
		t.Fatalf("identical ranges scored %v (%.3f)", r.Verdict, r.Probability)
	}
}

func TestUnrelatedRangesAreFalsePositive(t *testing.T) {
	local := "func handler(w http.ResponseWriter, r *http.Request) {\n\tuser := loadUser(r)\n\tif user == nil {\n\t\thttp.Error(w, \"forbidden\", 403)\n\t\treturn\n\t}\n\trender(w, user.Profile)\n}\n"
	oss := "def parse_config(path):\n    with open(path) as fh:\n        data = yaml.safe_load(fh)\n    validate(data)\n    return Config(**data)\n\n\nclass Config:\n    pass\n"
	c, _ := New()
	r, _ := c.Classify(Match{Local: []byte(local), OSS: []byte(oss), Spans: []Span{{1, 8, 1, 8}}})
	if r.Verdict != VerdictFalsePositive {
		t.Fatalf("unrelated ranges scored %v (%.3f)", r.Verdict, r.Probability)
	}
}

func TestOptions(t *testing.T) {
	if _, err := New(WithThreshold(0)); err == nil {
		t.Fatal("threshold 0 accepted")
	}
	if _, err := New(WithModel([]byte(`{"features":["nope"],"coef":[1]}`))); err == nil {
		t.Fatal("model with an unknown feature accepted")
	}
	c, err := New(WithThreshold(0.9), WithModel(defaultModel))
	if err != nil || c.Threshold() != 0.9 {
		t.Fatalf("options not applied: %v", err)
	}
}
