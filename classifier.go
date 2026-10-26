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

// Package snippetclassifier decides whether a SCANOSS snippet match is a false
// positive: a match whose reported OSS line range does not correspond to the
// reported local line range (a wrong candidate assignment).
//
// It compares the two ranges line by line and token by token — no semantic
// analysis — and scores the result with a small logistic regression trained on
// human-reviewed matches. It runs locally, needs no network and
// no state, and is safe for concurrent use.
//
// What it judges is correspondence only. The component the match is
// attributed to, and whether the code is generated, boilerplate or a table,
// do not matter: if the reported lines correspond, the match is real.
package snippetclassifier

import "errors"

// DefaultThreshold is the P(false positive) at or above which Classify reports
// a false positive. On the reviewed corpus it gives FP precision 0.95 and
// recall 0.89 (leave-one-project-out).
const DefaultThreshold = 0.7

// Verdict is the thresholded decision.
type Verdict string

const (
	// VerdictFalsePositive means the reported ranges do not correspond.
	VerdictFalsePositive Verdict = "false_positive"
	// VerdictReal means the reported ranges correspond.
	VerdictReal Verdict = "real"
)

// Match is one snippet match: the full contents of the local file and of the
// matched OSS file, and the ranges the scanner paired between them.
type Match struct {
	Local []byte
	OSS   []byte
	Spans []Span
}

// Result is the classifier's answer for one match.
type Result struct {
	// Probability is P(false positive), in [0, 1].
	Probability float64 `json:"probability"`
	// Verdict is Probability compared with the classifier's threshold.
	Verdict Verdict `json:"verdict"`
	// Contributions explain Probability, largest effect first.
	Contributions []Contribution `json:"contributions"`
	// Features holds every computed feature, including ones the model does not use.
	Features map[string]float64 `json:"features"`
	// ModelVersion identifies the model that produced the score.
	ModelVersion string `json:"model_version"`
}

// Classifier scores snippet matches. The zero value is not usable; call New.
type Classifier struct {
	model     *Model
	threshold float64
}

// Option configures a Classifier.
type Option func(*Classifier) error

// WithModel replaces the embedded model with one in the JSON shape
// the training code writes (see Model) — for example a per-organisation model.
func WithModel(data []byte) Option {
	return func(c *Classifier) error {
		m, err := ParseModel(data)
		if err != nil {
			return err
		}
		c.model = m
		return nil
	}
}

// WithThreshold sets the P(false positive) at or above which the verdict is
// VerdictFalsePositive.
func WithThreshold(t float64) Option {
	return func(c *Classifier) error {
		if t <= 0 || t > 1 {
			return errors.New("snippetclassifier: threshold must be in (0, 1]")
		}
		c.threshold = t
		return nil
	}
}

// New returns a Classifier using the embedded model and DefaultThreshold
// unless options say otherwise.
func New(opts ...Option) (*Classifier, error) {
	c := &Classifier{model: DefaultModel(), threshold: DefaultThreshold}
	for _, o := range opts {
		if err := o(c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// ModelVersion identifies the model in use.
func (c *Classifier) ModelVersion() string { return c.model.Version }

// Threshold is the configured decision threshold.
func (c *Classifier) Threshold() float64 { return c.threshold }

// ErrNoSpans is returned for a match without any line range.
var ErrNoSpans = errors.New("snippetclassifier: match has no spans")

// Classify scores one match.
func (c *Classifier) Classify(m Match) (Result, error) {
	if len(m.Spans) == 0 {
		return Result{}, ErrNoSpans
	}
	return c.ClassifyLines(SplitLines(m.Local), SplitLines(m.OSS), m.Spans), nil
}

// ClassifyLines is Classify for files already split with SplitLines.
func (c *Classifier) ClassifyLines(local, oss []string, spans []Span) Result {
	f := ComputeFeatures(local, oss, spans)
	p, contrib := c.model.Probability(f)
	v := VerdictReal
	if p >= c.threshold {
		v = VerdictFalsePositive
	}
	return Result{Probability: p, Verdict: v, Contributions: contrib, Features: f, ModelVersion: c.model.Version}
}
