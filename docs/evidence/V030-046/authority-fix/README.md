# Root authority fix evidence

Preserves the original unrelated-edit-grant regression and Root-authored fix verification. Root test SHA256 remains bd53d7a402ef66ecbe34cc19463c01da620e916ad39d347943534696c4013154 before and after the production fix. The prior failure and new GREEN use the same frozen tests.

- `edit-grant-red`: original failing reproduction at ef1b8f4e6b9f85cd8858d795eb562ba213c37fc0.
- `root-fix-green`: service 8 PASS, HTTPS 4 PASS, complete service race 174 top-level/93 subcase PASS at 5ab529c619de10ee53f3bbb360d0b66e9b39aa98.

Raw files and source snapshots are byte-preserved; each directory has an SHA256 index. No executor implementation, test changes, debugging, migration, CI or deployment action. TDD:N/A for this evidence-only commit. Whole BFF/all-repository/CI acceptance was not run.
