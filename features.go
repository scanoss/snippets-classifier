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
	"math"
	"strings"
)

// shingleK is the shingle length in tokens (metrics.K).
const shingleK = 5

// minTokenBlock is the shortest in-order token run counted by
// tok_aligned_ratio; shorter coincidences are noise.
const minTokenBlock = 8

// Span pairs a local line range with the OSS line range the scanner matched
// it to. Lines are 1-based and inclusive, as SCANOSS reports them.
type Span struct {
	LocalStart, LocalEnd int
	OSSStart, OSSEnd     int
}

// ComputeFeatures returns every feature the classifier knows, keyed by the
// names the training code uses. local and oss are
// the lines of the two files (see SplitLines).
func ComputeFeatures(local, oss []string, spans []Span) map[string]float64 {
	lr := make([][2]int, len(spans))
	or := make([][2]int, len(spans))
	for i, s := range spans {
		lr[i] = [2]int{s.LocalStart, s.LocalEnd}
		or[i] = [2]int{s.OSSStart, s.OSSEnd}
	}

	// File-level metrics (metrics.features_from_lines).
	inf := informativeLines(take(local, lr))
	ossInf := informativeLines(oss)
	ossSet := make(map[string]struct{}, len(ossInf))
	for _, l := range ossInf {
		ossSet[l] = struct{}{}
	}
	matched := 0
	for _, l := range inf {
		if _, ok := ossSet[l]; ok {
			matched++
		}
	}
	templates := map[string]struct{}{}
	for _, l := range inf {
		templates[abstractLine(l)] = struct{}{}
	}
	f := map[string]float64{
		"inf_lines":               float64(len(inf)),
		"matched_inf_lines":       float64(matched),
		"matched_inf_ratio":       ratio(matched, len(inf)),
		"longest_run":             float64(longestCommonRun(inf, ossInf)),
		"abstract_line_diversity": ratio(len(templates), len(inf)),
	}

	for k, v := range rangeFeatures(local, oss, lr, or) {
		f[k] = v
	}
	for _, k := range []string{"matched_inf_lines", "longest_run", "inf_lines", "range_longest_block"} {
		f["log_"+k] = math.Log1p(f[k])
	}
	return f
}

// rangeFeatures measures range-to-range correspondence: does the reported OSS
// range line up with the reported local range? A wrong candidate assignment
// fails exactly this (model.range_features).
func rangeFeatures(local, oss []string, lr, or [][2]int) map[string]float64 {
	li := informativeLines(take(local, lr))
	oi := informativeLines(take(oss, or))
	if len(li) == 0 || len(oi) == 0 {
		return map[string]float64{
			"range_aligned_ratio": 0, "range_longest_block": 0, "range_abstract_aligned_ratio": 0,
			"range_containment": 0, "tok_aligned_ratio": 0, "tok_abstract_aligned_ratio": 0,
		}
	}
	blocks := matchingBlocks(li, oi)
	longest, aligned := 0, 0
	for _, b := range blocks {
		aligned += b.Size
		if b.Size > longest {
			longest = b.Size
		}
	}
	abstractAligned := 0
	for _, b := range matchingBlocks(abstractLines(li), abstractLines(oi)) {
		abstractAligned += b.Size
	}
	lt := lineTokens(li)
	ot := lineTokens(oi)
	ls := shingles(lt)
	common := 0
	os := shingles(ot)
	for s := range ls {
		if _, ok := os[s]; ok {
			common++
		}
	}

	// Token level, in order, blind to line breaks; the OSS side is a window
	// around the reported range widened by the local range's length, because
	// reported ranges can be off.
	spanLen := 0
	for _, r := range lr {
		spanLen += r[1] - r[0] + 1
	}
	window := make([][2]int, len(or))
	for i, r := range or {
		window[i] = [2]int{max(1, r[0]-spanLen), r[1] + spanLen}
	}
	owt := lineTokens(informativeLines(take(oss, window)))
	tokAligned := 0
	for _, b := range matchingBlocks(lt, owt) {
		if b.Size >= minTokenBlock {
			tokAligned += b.Size
		}
	}
	at := abstractTokens(lt)
	tokAbstract := 0
	for _, b := range matchingBlocks(at, abstractTokens(owt)) {
		if b.Size >= minTokenBlock {
			tokAbstract += b.Size
		}
	}
	return map[string]float64{
		"range_aligned_ratio":          ratio(aligned, len(li)),
		"range_longest_block":          float64(longest),
		"range_abstract_aligned_ratio": ratio(abstractAligned, len(li)),
		"range_containment":            ratio(common, len(ls)),
		"tok_aligned_ratio":            ratio(tokAligned, len(lt)),
		"tok_abstract_aligned_ratio":   ratio(tokAbstract, len(at)),
	}
}

// take returns the lines inside the 1-based inclusive ranges, in range order;
// out-of-file line numbers are skipped (metrics.take).
func take(lines []string, ranges [][2]int) []string {
	var out []string
	for _, r := range ranges {
		for i := r[0]; i <= r[1]; i++ {
			if i > 0 && i <= len(lines) {
				out = append(out, lines[i-1])
			}
		}
	}
	return out
}

func informativeLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if n := informative(l); n != "" {
			out = append(out, n)
		}
	}
	return out
}

func lineTokens(lines []string) []string {
	var out []string
	for _, l := range lines {
		out = append(out, tokens(l)...)
	}
	return out
}

func abstractTokens(toks []string) []string {
	out := make([]string, len(toks))
	for i, t := range toks {
		out[i] = abstract(t)
	}
	return out
}

func abstractLine(l string) string { return strings.Join(abstractTokens(tokens(l)), " ") }

func abstractLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = abstractLine(l)
	}
	return out
}

// shingles is the set of k-token windows; a sequence shorter than k is one
// shingle, and an empty one has none (metrics.shingles).
func shingles(toks []string) map[string]struct{} {
	out := map[string]struct{}{}
	if len(toks) == 0 {
		return out
	}
	if len(toks) < shingleK {
		out[strings.Join(toks, "\x1f")] = struct{}{}
		return out
	}
	for i := 0; i+shingleK <= len(toks); i++ {
		out[strings.Join(toks[i:i+shingleK], "\x1f")] = struct{}{}
	}
	return out
}

// longestCommonRun is the longest run of consecutive lines of a that also
// appears consecutively in b (metrics.longest_common_run).
func longestCommonRun(a, b []string) int {
	pos := map[string][]int{}
	for j, x := range b {
		pos[x] = append(pos[x], j)
	}
	best := 0
	prev := map[int]int{}
	for _, x := range a {
		cur := map[int]int{}
		for _, j := range pos[x] {
			cur[j] = prev[j-1] + 1
			if cur[j] > best {
				best = cur[j]
			}
		}
		prev = cur
	}
	return best
}

func ratio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}
