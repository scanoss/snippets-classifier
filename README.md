# snippets-classifier

A Go library that decides whether a SCANOSS **snippet match** is a false
positive. It compares the local line range with the OSS line range the scanner
paired it with, and scores the comparison with a small logistic regression
trained on human-reviewed matches.

- **What it needs:** no network, no state and no model server. It is plain Go
  with the model embedded, and is safe for concurrent use.
- **How it compares:** by line and by token only. There is no semantic or
  syntactic analysis of the code.
- **Where it is used:** SCANOSS Earnie imports it to propose "not a real
  match" on snippet findings.

This repository holds the library, the embedded model and a small command-line
wrapper. The labelled data and the training code are not published; see
[The model](#the-model).

## What counts as a false positive

These are the review criteria, decided by the product owner. The classifier is
trained to apply them, and anyone reviewing new cases must apply them too.

1. **A false positive is a wrong candidate assignment.** The **reported** OSS
   range does not correspond to the **reported** local range. Only scattered
   generic lines coincide (braces, imports, `return`, framework idioms,
   getters), while the surrounding code does something different.
2. **The reported ranges are what is judged.** Suppose the local code does
   exist in the candidate file, but the reported OSS range points somewhere
   else in it. That match is a **false positive**: the match as reported is
   wrong. This happens: SCANOSS sometimes reports an OSS range that is offset
   inside the right file, and the offset is already present in the raw
   scanoss-go result.
3. **If the reported lines correspond as a block, the match is real,** whatever
   the code is. Generated code, framework boilerplate, license headers,
   constant tables and scaffolding are all real when their lines line up.
4. **The component does not matter.** A fork, a vendored copy, or another
   project carrying the same code is still a real match. Attribution is a
   separate question, handled by Earnie's triage (`replace_component`), not by
   this classifier.
5. **Reformatted or renamed code counts only if the reported lines still line
   up.** Formatting noise does not break correspondence: whitespace, case,
   trailing `;`/`,` and a leading `_` on identifiers are ignored. Code that was
   rewritten so the lines no longer correspond is a false positive.

A human can usually see the answer in the side-by-side view within seconds.
When they cannot, the case is labelled `unsure` and left out of training.

## Usage

```go
import snippetclassifier "github.com/scanoss/snippets-classifier"

c, err := snippetclassifier.New() // embedded model, threshold 0.7
if err != nil { ... }

res, err := c.Classify(snippetclassifier.Match{
	Local: localFileBytes,
	OSS:   ossFileBytes,
	Spans: []snippetclassifier.Span{{LocalStart: 10, LocalEnd: 42, OSSStart: 118, OSSEnd: 150}},
})
// res.Probability   P(false positive), 0..1
// res.Verdict       "false_positive" | "real" (Probability >= threshold)
// res.Contributions per-feature push towards FP (+) or real (−), largest first
// res.Features      every computed feature
// res.ModelVersion  e.g. "lr-d51f2e100211" (content hash of the model)
```

- `Span` lines are 1-based and inclusive, exactly as SCANOSS reports them. A
  match with several spans passes them all.
- `WithThreshold(t)` changes the decision threshold. `WithModel(json)` loads
  another model in the same format, for example a per-organisation model.
- `SplitLines` / `ClassifyLines` are available when the files are already
  split into lines.

A command-line wrapper prints the result as JSON:

```bash
go run ./cmd/snippet-classify --local a.java --oss b.java --span 5-44:12-51 [--span …] [--threshold 0.9]
```

### Choosing the threshold

These figures are leave-one-project-out on the 263 reviewed cases (120 false
positive, 143 real). Every case is scored by a model that never saw its project.

| Threshold | FP precision | FP recall | Real matches flagged |
|---|---|---|---|
| 0.5 | 0.93 | 0.92 | 8 |
| **0.7 (default)** | **0.95** | **0.89** | 6 |
| 0.9 | 0.98 | 0.72 | 2 |

The ranking quality is AUC 0.970. Use 0.7 to *propose* false positives for a
human to confirm. Anything that dismisses matches without a human should use a
higher threshold: dismissing a real match is the expensive error.

### Performance

Median 3.4 ms per match. The worst reviewed case takes 0.9 s, against OSS files of
up to 216,000 lines. Most of the cost is the in-order alignment of the ranges and of a window
around the OSS range. The rest of the OSS file is read once, in linear time.

## How it works

Both files are split into lines the way Python's `str.splitlines()` does, and
each line is normalised:

- whitespace is collapsed and case is lowered;
- trailing `;`/`,` and leading `_` on identifiers are dropped;
- lines that carry no information are marked trivial: blank lines, punctuation
  only, comments, and import/include/package lines.

The features compare the **informative** lines:

| Feature | What it measures |
|---|---|
| `log_range_longest_block` | Longest run of identical informative lines inside the reported ranges, log-scaled. This is the strongest signal. |
| `range_containment` | Share of the local range's 5-token shingles present in the reported OSS range. |
| `range_aligned_ratio` | Share of informative local lines aligned, in order, with the OSS range (difflib matching blocks). |
| `range_abstract_aligned_ratio` | The same after replacing identifiers and literals with `I`/`S`/`N`. It survives renames. |
| `tok_aligned_ratio` | In-order token alignment, blind to line breaks, against a window around the OSS range. Only runs of 8 or more tokens count. |
| `log_inf_lines` | Size of the local range. Short ranges coincide more easily. |
| `abstract_line_diversity` | Distinct line templates. It is low for tables, accessors and field lists, where abstraction makes every row look alike. |
| `matched_inf_ratio` | Share of informative local lines found anywhere in the OSS file. |
| `log_longest_run` | Longest identical run anywhere in the OSS file. |

A standardised logistic regression turns the features into P(false positive).
Each coefficient is readable. The largest are: a long identical block and high
range containment push towards *real*, and small, low-diversity ranges push
towards *false positive*.

Things that were tried and are not in the model:
- the SCANOSS `matched` percentage, which reaches 86–87 % on clear false
  positives;
- simhash;
- IDF-weighted containment;
- boosted trees, which did not beat the regression on the human labels
  (AUC 0.961–0.966 vs 0.970).

### Known limits

- **Idioms.** Ubiquitous idioms that match line for line (a gin `cors.Config`
  block, a JWT middleware from a tutorial) score as *real*, because their lines
  do correspond. Reviewers split on these cases.
- **Translations.** A translated resource file with the same keys and different
  values has no line in common and scores *false positive*.
- **Offset ranges.** A reported range that is offset inside the right file
  scores *false positive* by design (criterion 2).

## The model

`model/model.json` is a standardised logistic regression over the nine
features above: feature names, standardisation statistics, coefficients,
intercept, and the evaluation figures of the version. Its `version` is a
content hash and is reported as `ModelVersion` in every result.

The model is trained in Python on human-reviewed SCANOSS snippet matches. The
Go code here is a port of the Python feature code, and the golden files in
`testdata/` pin the building blocks (`difflib` matching blocks, line
normalisation, tokenisation and `splitlines`) to the Python values. They are
generated from the Python reference, never edited by hand. The training data
and code are kept in a private repository, where a parity test checks every
reviewed case against this library to 1e-9.

**A new model is a new library version.** Each retrain is released as a new
tag, and `ModelVersion` identifies the model that produced each score.

## Reporting a misclassified match

Open an issue with the result JSON (`ModelVersion`, `Probability`, the
features) and the reported ranges. If you can share them, include both code
ranges; otherwise describe what the two ranges contain. Judge the match by the
criteria above: only whether the reported ranges correspond.

## Layout

```
.
├── classifier.go         public API: New, Classify, options, Result
├── features.go           feature extraction (port of the Python features)
├── normalize.go          line normalisation, tokens, abstraction
├── difflib.go            port of difflib.SequenceMatcher matching blocks
├── lines.go              Python-compatible splitlines + UTF-8 replacement
├── model.go              logistic regression, embedded model
├── model/model.json      the trained model (embedded)
├── cmd/snippet-classify  command-line wrapper
└── testdata/             golden files for the building blocks
```

## Development

```bash
make test      # Go tests
make vet       # go vet + gofmt
make help      # every target
```

The library uses the Go standard library only.

## License

MIT; see [LICENSE](LICENSE). `difflib.go` ports part of CPython's `difflib`
and also carries the Python Software Foundation License Version 2; see
[LICENSES/PSF-2.0.txt](LICENSES/PSF-2.0.txt).
