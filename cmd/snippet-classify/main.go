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

// Command snippet-classify scores one SCANOSS snippet match from the command
// line and prints the result as JSON.
//
//	snippet-classify --local src/util.c --oss oss/util.c --span 10-42:118-150 [--span ...]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	snippetclassifier "github.com/scanoss/snippets-classifier"
)

type spanFlags []snippetclassifier.Span

func (s *spanFlags) String() string { return fmt.Sprint(*s) }

// Set parses LOCAL_START-LOCAL_END:OSS_START-OSS_END (1-based, inclusive).
func (s *spanFlags) Set(v string) error {
	local, oss, ok := strings.Cut(v, ":")
	if !ok {
		return fmt.Errorf("span %q: want LOCAL_START-LOCAL_END:OSS_START-OSS_END", v)
	}
	ls, le, err := parseRange(local)
	if err != nil {
		return err
	}
	os_, oe, err := parseRange(oss)
	if err != nil {
		return err
	}
	*s = append(*s, snippetclassifier.Span{LocalStart: ls, LocalEnd: le, OSSStart: os_, OSSEnd: oe})
	return nil
}

func parseRange(r string) (int, int, error) {
	a, b, ok := strings.Cut(r, "-")
	if !ok {
		b = a
	}
	x, err := strconv.Atoi(a)
	if err != nil {
		return 0, 0, fmt.Errorf("range %q: %w", r, err)
	}
	y, err := strconv.Atoi(b)
	if err != nil {
		return 0, 0, fmt.Errorf("range %q: %w", r, err)
	}
	return x, y, nil
}

func main() {
	var spans spanFlags
	local := flag.String("local", "", "local (scanned) file")
	oss := flag.String("oss", "", "matched OSS file")
	model := flag.String("model", "", "model JSON (default: the embedded model)")
	threshold := flag.Float64("threshold", snippetclassifier.DefaultThreshold, "P(false positive) at or above which the verdict is false_positive")
	flag.Var(&spans, "span", "LOCAL_START-LOCAL_END:OSS_START-OSS_END; repeat for several spans")
	flag.Parse()
	if *local == "" || *oss == "" || len(spans) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*local, *oss, *model, *threshold, spans); err != nil {
		fmt.Fprintln(os.Stderr, "snippet-classify:", err)
		os.Exit(1)
	}
}

func run(localPath, ossPath, modelPath string, threshold float64, spans []snippetclassifier.Span) error {
	opts := []snippetclassifier.Option{snippetclassifier.WithThreshold(threshold)}
	if modelPath != "" {
		data, err := os.ReadFile(modelPath)
		if err != nil {
			return err
		}
		opts = append(opts, snippetclassifier.WithModel(data))
	}
	c, err := snippetclassifier.New(opts...)
	if err != nil {
		return err
	}
	l, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}
	o, err := os.ReadFile(ossPath)
	if err != nil {
		return err
	}
	res, err := c.Classify(snippetclassifier.Match{Local: l, OSS: o, Spans: spans})
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}
