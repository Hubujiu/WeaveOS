# Root browser failure inspection at 95025dca

Job112362528080 failed two existing WebKit Q36 tests,157passed/9existing capability skips. Root downloaded artifact11426356230 and verified ZIP SHA256 a0324f6acf6b62dbe88750e30a1d2facc5771bc93479d4c1f43e626e17ab8cdd; personally viewed both screenshots and inspected nested traces.

The selection-mark press case spent10.014s in navigation; Google Fonts CSS took9.275s, leaving little of its15s whole-case budget. The member typed-filter case instead spent7.939s in newPage setup (fixture/hook8.897s),1.133s in navigation and3.255s clicking the filter trigger; its Google Fonts CSS was only108ms. Do not attribute both failures exclusively to font loading. Both hit the unchanged overall15s bound late in their interactions; screenshots are context, not proof of passed assertions. There is no product/UI/test/timeout change in V041 to these cases.

Root requested one same-head rerun of only the failed browser job at16:07UTC, preserving this first failure. No repeated green-chasing, removed assertion, skipped test or deadline increase. If it repeats, inspect actual evidence before further action. Full product job remains independent and running.

## Single same-head retry result
Job112370736921 failed only the same first WebKit typed-filter test (158PASS/9existing skips); prior selection-mark case passes. Root verified artifact11427127330 ZIP SHA256 ba11a9f15b4f56cf90ef3f3b07764acc4ec2e3155e0484865847a9b43a380813 and inspected screenshot/trace. newPage8.124s/setup8.721s/navigation1.234s/filter-trigger3.810s; Google Fonts CSS177.864ms, so fonts do not explain this repeated failure. No further blind rerun.

Proposed, NOT implemented: separate beforeAll browser preparation creates and closes only an isolated blank page/context. Every business test retains a fresh context/page, unchanged15s case and3s assertions, actual animations and zero automatic retries. No application state seeded/cached and no font blocking/UI change. User confirmation requested in Sentinel_eab6f5737adc819193dc0bf3b5f8552a at2026-10-06T16:24:07UTC after writing NotionADR. Pending decision does not authorize this change. Actual product run37490749896 is independently still running; first HTTPS and security stages passed.
