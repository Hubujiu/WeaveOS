# Root HTTP contract RED

2026-10-07 UTC. Source baseline develop8ecd39a2; behavioral Save implementation remains a declaration-only error.
Root authored root-node-save-http-contract.test.mjs from Notion V043 HTTP/body/preview requirements, then ran it against the unchanged OpenAPI. All three tests loaded and failed for the missing PATCH, Save body schema and editableFieldIds. This is registration-contract RED, not a real HTTP or storage result.
previous-preview.json preserves the exact old four-field schema. Node test raw output is in tests.log. Root then added only the new path/input and additive editableFieldIds; 3/3 registration tests pass, raw output ../http-contract-green/tests.log. One intermediate candidate failed the explicit object-type check and was corrected before GREEN. No assertion changed.
Original HTTP exact-preview assertion is deliberately updated to five fields AND an explicitly empty editable array for the original read-only node, per the new response contract. It is not loosened into a partial assertion.
Real storage and HTTPS RED remain separate evidence; no implementation released on the basis of these registration tests alone.
