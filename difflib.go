// SPDX-License-Identifier: MIT AND PSF-2.0
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
 *
 * This file ports the matching-block algorithm of difflib.SequenceMatcher
 * (find_longest_match and get_matching_blocks) from CPython's Lib/difflib.py:
 *
 *   Copyright (c) 2001-2024 Python Software Foundation; All Rights Reserved
 *
 * That part is used under the Python Software Foundation License Version 2;
 * see LICENSES/PSF-2.0.txt. Changes from the original: translated to Go,
 * restricted to no junk and autojunk=False, elements interned to ints, and
 * the dynamic-programming rows kept in slices cleared by the indices they
 * touched instead of a dict per row. The blocks returned are the same.
 */

package snippetclassifier

import "sort"

// block is a matching block of two sequences: a[A:A+Size] == b[B:B+Size].
type block struct{ A, B, Size int }

// matchingBlocks ports Python's difflib.SequenceMatcher(None, a, b,
// autojunk=False).get_matching_blocks(), without the final sentinel. It must
// return exactly the blocks Python returns — including its tie-breaking and the
// merge of adjacent blocks — because features are computed from them and the
// model was trained on the Python values.
//
// The algorithm is Python's; the data structures are not. Elements are interned
// to ints and the dynamic-programming rows are slices cleared by the indices
// they touched, which keeps small-alphabet inputs (abstracted tokens, where
// every symbol occurs thousands of times) fast without changing any result.
func matchingBlocks(a, b []string) []block {
	ids := make(map[string]int, len(b))
	bi := make([]int, len(b))
	for j, x := range b {
		id, ok := ids[x]
		if !ok {
			id = len(ids)
			ids[x] = id
		}
		bi[j] = id
	}
	ai := make([]int, len(a))
	for i, x := range a {
		if id, ok := ids[x]; ok {
			ai[i] = id
		} else {
			ai[i] = -1 // absent from b: never matches
		}
	}
	b2j := make([][]int, len(ids))
	for j, id := range bi {
		b2j[id] = append(b2j[id], j)
	}
	m := &matcher{a: ai, b: bi, b2j: b2j, prev: make([]int, len(b)+1), cur: make([]int, len(b)+1)}

	type span struct{ alo, ahi, blo, bhi int }
	queue := []span{{0, len(a), 0, len(b)}}
	var blocks []block
	for len(queue) > 0 {
		q := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		i, j, k := m.findLongestMatch(q.alo, q.ahi, q.blo, q.bhi)
		if k == 0 {
			continue
		}
		blocks = append(blocks, block{i, j, k})
		if q.alo < i && q.blo < j {
			queue = append(queue, span{q.alo, i, q.blo, j})
		}
		if i+k < q.ahi && j+k < q.bhi {
			queue = append(queue, span{i + k, q.ahi, j + k, q.bhi})
		}
	}
	sort.Slice(blocks, func(x, y int) bool {
		if blocks[x].A != blocks[y].A {
			return blocks[x].A < blocks[y].A
		}
		if blocks[x].B != blocks[y].B {
			return blocks[x].B < blocks[y].B
		}
		return blocks[x].Size < blocks[y].Size
	})
	var out []block
	var cur block
	for _, bl := range blocks {
		if cur.A+cur.Size == bl.A && cur.B+cur.Size == bl.B {
			cur.Size += bl.Size
			continue
		}
		if cur.Size > 0 {
			out = append(out, cur)
		}
		cur = bl
	}
	if cur.Size > 0 {
		out = append(out, cur)
	}
	return out
}

type matcher struct {
	a, b      []int
	b2j       [][]int
	prev, cur []int // run length ending at b[j-1] is stored at index j; slot 0 is always 0
	prevTouch []int
	curTouch  []int
}

// findLongestMatch ports SequenceMatcher.find_longest_match with no junk: the
// longest a[i:i+k] == b[j:j+k] inside the window, earliest in a, then earliest
// in b.
func (m *matcher) findLongestMatch(alo, ahi, blo, bhi int) (int, int, int) {
	besti, bestj, bestsize := alo, blo, 0
	for i := alo; i < ahi; i++ {
		if id := m.a[i]; id >= 0 {
			js := m.b2j[id]
			for _, j := range js[sort.SearchInts(js, blo):] {
				if j >= bhi {
					break
				}
				k := m.prev[j] + 1 // j2len[j-1] + 1
				m.cur[j+1] = k
				m.curTouch = append(m.curTouch, j+1)
				if k > bestsize {
					besti, bestj, bestsize = i-k+1, j-k+1, k
				}
			}
		}
		for _, x := range m.prevTouch {
			m.prev[x] = 0
		}
		m.prev, m.cur = m.cur, m.prev
		m.prevTouch, m.curTouch = m.curTouch, m.prevTouch[:0]
	}
	for _, x := range m.prevTouch {
		m.prev[x] = 0
	}
	m.prevTouch = m.prevTouch[:0]
	// Python extends the match over equal neighbours; with no junk the loop
	// above is already maximal, but the extension is kept for fidelity.
	for besti > alo && bestj > blo && m.a[besti-1] == m.b[bestj-1] {
		besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
	}
	for besti+bestsize < ahi && bestj+bestsize < bhi && m.a[besti+bestsize] == m.b[bestj+bestsize] {
		bestsize++
	}
	return besti, bestj, bestsize
}
