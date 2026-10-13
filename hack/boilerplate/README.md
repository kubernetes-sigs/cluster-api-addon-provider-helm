# Boilerplate utilities

The boilerplate validation utilities have been copied and adapted from upstream Kubernetes, see https://github.com/kubernetes/kubernetes/blob/4dfd73940396730caf331e35cbb28235d233f2a0/hack/boilerplate.

## Copyright headers

Following [Kubernetes PR #134835](https://github.com/kubernetes/kubernetes/pull/134835),
new files use `Copyright The Kubernetes Authors.` without a year. Existing
copyright notices dated 2014-2025 remain valid and should be preserved. Headers
with a year of 2026 or later, or an unexpanded `YEAR` placeholder, are rejected.
Generated files continue to use yearless headers.

Both this verifier and the `goheader` rule in `.golangci.yaml` enforce this policy.
The four 2026-dated source headers already present in this repository were
converted to yearless headers when adopting the policy; their author attribution
is unchanged.

Run `make verify-boilerplate` to execute the Python 3 regression tests and verify
all repository headers. The tests cover yearless and historical headers, the
2025/2026 boundary, invalid headers, and every non-generated boilerplate format.
