# Root R03 / R04 controlled migration registration verification

This is Root-authored registration-test verification only. It is not database
compatibility, application migration, release packaging, or deployment acceptance.
No test source, released migration, other manifest entry, or protected branch was
modified in either temporary worktree.

## Immutable input and workspace state

- Remote task branch at verification: `task/V030-013-definition-api` ->
  `22c256ed790d65c0a8cb2b4a7f58f5fc66a17c7f`; fetched that exact commit.
- Baseline: detached temporary worktree at that commit; clean. Its compatibility
  manifest SHA256 is `ed527eeaace14b4085c493994e73f38172a141c12305bfdb1c47c65f7f8f0b06`.
- Candidate: separate detached temporary worktree at the same commit. The only
  copied source file was the previously prepared `compatibility.json`; its
  SHA256 is Root-accepted `1b142ba7aaf774822b4db57ef6dcab8bf4c6177b14ac04b55586f66a600a507c`.
- Root test SHA256 in both worktrees:
  `2c93fbf0a76634848fb4eadc8a42fdfddc4bac87a038b4d6acd679f2a0f7a151`.
- Candidate test-source identity, the original nine path/hash entries, and the
  cold4-before-hot7-through-12 assertion all pass unchanged.

## Commands and exact results

1. In each worktree: `node --test tests/foundation/root-migration-registration.test.mjs`.
   Baseline exit 1: 3 cases, 1 pass / 2 fail / 0 skip. Digest-registration fails
   on missing cold4; ordering case fails on absent cold4. The prior reviewed
   registration preservation case passes.
   Candidate exit 0: 3 cases, 3 pass / 0 fail / 0 skip.
2. In candidate: `node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs`.
   Exit 0: 240 pass / 0 fail / 0 skip.
3. In candidate: `node scripts/check-tasks.mjs` exit 0 and
   `node scripts/verify-repo.mjs` exit 0.

Raw stdout/stderr is preserved in `baseline.log`, `candidate.log`, and
`governance-foundation.log`. The full candidate-only manifest diff is
`candidate-manifest.diff`.
