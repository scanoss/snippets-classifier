# AGENTS — snippets-classifier

Read [README.md](README.md) first: it defines what a false positive is, and that
definition is the product owner's decision, not an implementation detail.

## Invariants (do NOT break)

1. **The false-positive criteria in the README are fixed.** A false positive
   means the *reported* ranges do not correspond. The component does not matter,
   and neither does the kind of code (generated code, boilerplate, tables).
   Never change features or thresholds towards another definition. If a case
   seems to call for one, ask.
2. **Python is the reference and Go must match it.** The model is trained on
   features computed by the Python training code, kept in a private repository
   (`scanoss/snippets-classifier-training`). The golden files in `testdata/` are
   generated there and are never edited by hand. A feature change is made on
   both sides together, followed by a retrain and the parity test in that
   repository.
3. **A model change is a version change.** Each retrain is released as a new tag.
   `ModelVersion` must identify the model that produced every score.
4. **Nothing third-party, nothing private.** This repository is public and MIT
   licensed. Never add training data, review labels, corpus code or matched OSS
   files to it, and never add code copied from other projects. A port of
   third-party code keeps its notice and license, as `difflib.go` does.
5. **Standard library only.** Keep the package free of dependencies beyond the
   Go standard library.

## Conventions

- Everything is in English: code, comments, docs, commit messages.
- Every Go file starts with the SPDX identifier and the MIT notice of SCANOSS.
- Go: gofmt and the standard library only. Comments explain *why*, especially
  wherever the code mirrors Python behaviour.
- The Go commands use `GOWORK=off`, because the parent directory may hold a
  `go.work` that does not include this module.
- Every behaviour change needs tests: unit tests for the building blocks and
  the public API tests in `classifier_test.go`.
