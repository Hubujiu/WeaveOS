# edit-grant-red

Original reproduction at ef1b8f4e6b9f85cd8858d795eb562ba213c37fc0: one FAIL, exit 1. L155: unrelated edit capability changed read summary; complete query projection changed. Actual regression, not environment failure. Source, test, JSONL, stderr, exit and run.json preserved byte-for-byte.

TDD:N/A: evidence-only preservation. No executor production/test/debug changes.

| Original artifact | Bytes | SHA256 |
| --- | --- | --- |
| root_workflow_read_test.go | 5858 | bd53d7a402ef66ecbe34cc19463c01da620e916ad39d347943534696c4013154 |
| run.json | 2700 | 66170f6fbc4ad4b9de436643c472146c0c22f9d112ddece17994e9d24797a2e4 |
| test.exit | 2 | 4355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd865 |
| test.stderr | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| test.stdout.jsonl | 2123 | eb9cc939d188b6c58081b025219ae5fb28e38e4bbd67d1ed8422671de5b5a917 |
| workflow_read.go | 10764 | 19ded974602e1a8c47025df41b10c7ef4719238193a21bc367371671487db437 |
