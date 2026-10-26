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
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// defaultModel is the model the training pipeline writes. Training data and
// code are kept in a separate, private repository.
//
//go:embed model/model.json
var defaultModel []byte

// Model is a standardised logistic regression over named features, in the
// JSON shape the training code writes. Its positive class is
// "false positive": Probability returns P(false positive).
type Model struct {
	Version   string                `json:"version"`
	Features  []string              `json:"features"`
	Stats     map[string][2]float64 `json:"stats"` // feature -> [mean, standard deviation]
	Coef      []float64             `json:"coef"`
	Intercept float64               `json:"intercept"`
	// TrainedOn and the evaluations are informational; nothing reads them.
	TrainedOn json.RawMessage `json:"trained_on,omitempty"`
	EvalHuman json.RawMessage `json:"eval_human,omitempty"`
}

// ParseModel decodes and validates a model.
func ParseModel(data []byte) (*Model, error) {
	var m Model
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("snippetclassifier: decode model: %w", err)
	}
	if len(m.Features) == 0 || len(m.Features) != len(m.Coef) {
		return nil, fmt.Errorf("snippetclassifier: model has %d features and %d coefficients", len(m.Features), len(m.Coef))
	}
	known := ComputeFeatures(nil, nil, nil)
	for _, f := range m.Features {
		if _, ok := known[f]; !ok {
			return nil, fmt.Errorf("snippetclassifier: model uses unknown feature %q", f)
		}
		if _, ok := m.Stats[f]; !ok {
			return nil, fmt.Errorf("snippetclassifier: model has no standardisation stats for %q", f)
		}
	}
	return &m, nil
}

// DefaultModel returns the embedded model.
func DefaultModel() *Model {
	m, err := ParseModel(defaultModel)
	if err != nil {
		panic(err) // the embedded model is validated by the test suite
	}
	return m
}

// Contribution is how much one feature pushed the score, in logit units:
// positive towards "false positive", negative towards "real".
type Contribution struct {
	Feature string  `json:"feature"`
	Value   float64 `json:"value"`
	Logit   float64 `json:"logit"`
}

// Probability returns P(false positive) and the per-feature contributions,
// largest first.
func (m *Model) Probability(features map[string]float64) (float64, []Contribution) {
	z := m.Intercept
	contrib := make([]Contribution, len(m.Features))
	for i, f := range m.Features {
		st := m.Stats[f]
		sd := st[1]
		if sd == 0 {
			sd = 1
		}
		c := m.Coef[i] * (features[f] - st[0]) / sd
		z += c
		contrib[i] = Contribution{Feature: f, Value: features[f], Logit: c}
	}
	sort.SliceStable(contrib, func(a, b int) bool { return math.Abs(contrib[a].Logit) > math.Abs(contrib[b].Logit) })
	return sigmoid(z), contrib
}

func sigmoid(z float64) float64 {
	z = math.Max(-30, math.Min(30, z))
	return 1 / (1 + math.Exp(-z))
}
