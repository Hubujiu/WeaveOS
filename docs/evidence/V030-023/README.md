# V030-023 evidence

Root source/test commit: `75ff884aba7aea37cafd69d6855b77d76ea5984c`.
RED checkpoint: `6a2351705b293b162df20b67c861f58300b209ac`.
Implementation checkpoint: `2d3ec54f1ff848297541439077add7232b4157b9`;
final evidence commit also includes the scoped checkbox color correction.

## Test oracle and provenance

`red/root-tests.ts.txt` is Root's immutable four-case source; `red/run.txt` records
actual behavior RED (3 failed / 1 already passing), following successful typecheck.
Failures: both desktop palette widths 224 instead of196; preview focus not restored.
The narrow reduced-motion test already passed. No artificial RED was introduced.
`test-integrity.json` compares all448 baseline test/fixture blob hashes: zero changes.
No tests, expectations, fixtures, snapshots or test configs in the repository changed.

Final browser verification uses pinned Playwright v1.63.0 noble image digest
`sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27`.
`green/default-container.txt`: forms 234 passed and Root tests 4 passed using unchanged
original configs. `green/components-final.txt` records the final full component run
with original component config and four workers: 498 passed, 1 existing WebKit-only
skip under Chromium (499 total). No extra browser isolation or
host-validation override variables are supplied; see `default-browser-environment.txt`.
Built-in Playwright launch defaults are unchanged. Earlier host component logs and
scratch wrapper are diagnostic: one run lost its shared Vite server, so a fresh
independent container is used for the final complete regression.

`green/forms*.txt` except `default-container.txt` records earlier diagnostic attempts,
including an interrupted run, and must NOT be used for default-browser acceptance.
The initial Firefox EROFS is retained in `environment-firefox.txt`. Root rejected
local content-isolation overrides; the child process trees were explicitly terminated
and verified stopped. No such overrides are persisted in repository configurations.
`real-service.md` describes the actual isolated HTTP/PG18/Redis diagnostic bridge and
its fixture failure/recovery. Standard final-head product CI supplies default runtime
acceptance; the diagnostic bridge result does not replace it.

## Presentation screenshots

These screenshots use Root's existing HTTP component fixture with additional fields
created through the real UI, not hard-coded production data. The saved code remains
API-driven. Error images show denied configuration permission; confirmation images
show the existing unsaved-leave guard. Desktop filled views start at the content top;
mobile screenshots scroll to the active card. Existing outer Shell navigation and
application headings are retained as required, although the Figma sample differs.

| State | 1440 | 1280 | 390 |
| --- | --- | --- | --- |
| Designer with fields | [1440](screenshots/designer-filled-1440.png) | [1280](screenshots/designer-filled-1280.png) | [390](screenshots/designer-filled-390.png) |
| Preview | [1440](screenshots/preview-1440.png) | [1280](screenshots/preview-1280.png) | [390](screenshots/preview-390.png) |
| Field settings | [1440](screenshots/field-settings-1440.png) | [1280](screenshots/field-settings-1280.png) | [390](screenshots/field-settings-390.png) |
| Leave confirmation | [1440](screenshots/confirmation-1440.png) | [1280](screenshots/confirmation-1280.png) | [390](screenshots/confirmation-390.png) |
| Error | [1440](screenshots/error-1440.png) | [1280](screenshots/error-1280.png) | [390](screenshots/error-390.png) |

The two empty-designer geometry screenshots are also included. Figma screenshot is
only a visual target, never an app asset; existing Lucide components are reused.
`capture.cjs.txt` is a capture utility with no new or changed test assertions.
Library prepared-upload was attempted and failed authentication HTTP401; no Library
file was claimed saved. Raw short failure is in `library-attempt.txt`.

Root owns final visual acceptance. CI must be checked against the frozen PR head,
not either earlier checkpoint. No main merge or deployment is authorized/performed.

Q36 checkpoint failure diagnostic: the unchanged WebKit test passed once in the
default pinned container (5.8s test, original 15s timeout). This does not erase the
failed checkpoint CI or prove its timing cause; final-head CI remains authoritative.
See `green/q36-default-reproduction.txt` in the evidence directory.
