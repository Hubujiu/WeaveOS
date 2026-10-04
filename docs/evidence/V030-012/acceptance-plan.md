# V030-012 acceptance mapping

Independent sources: limited frozen V030-012.1–5, AP-FR-10 and B5a.1–7
from PRD/ADR009; original Figma Home358:18205/catalog327:2132/create332:3310;
versioned OpenAPI Application schemas. Status below describes this checkpoint.

| Requirement | Test/verification | Status |
| --- | --- | --- |
| Home opens real catalog | applications.component.spec.ts: Home opens catalogue | Observed missing Home action RED; result recorded separately |
| B5 metadata rather than me/access.applications | component catalogue fixture keeps old personnel applications empty | Observed missing route UI RED |
| No query parameters; authorized local filter only | component catalogue captures all requests and asserts no search string | Catalogue heading RED; later assertions not reached |
| Create hint from server capability | component create-only actor has applications.create and no Bootstrap/personnel role | Create action RED; dialog assertions not reached |
| Open reauthorizes; no fabricated forms | component workspace records access requests and expects real empty content | Observed missing route UI RED |
| Owner/Bootstrap policy configuration | real actor matrix: owner, Bootstrap, member, create-only, denied, cross-app | NOT RUN; implementation/source blocked |
| Same-group member plus enabled root-menu grant | real member catalog/access after membership only, grant, disabled group, revoke | NOT RUN |
| CAS and complete replacement | group updates, [] member/grant clears, stale revision refresh | Planned; no client behavior yet |
| Original operation recovery | lost response -> unconfirmed -> original operation query; absent result remains inconclusive; same-key retry | Planned; no client behavior yet |
| Server-confirmed success | hold write response, observe no success; then confirmed response and re-read | Planned |
| Unauthorized/error/loading/empty/retry | component controlled HTTP responses plus actual API 401/403 and revoke | Planned |
| Keyboard/focus/dirty-close | keyboard catalog/create/policy edits; Escape/cancel/focus return, history and tab-close guard | Planned, three browsers required |
| Rapid navigation/old async results | controlled deferred A read, switch B, deliver A last and assert B remains | Planned |
| Reduced motion | media emulation, repeat navigation/close, assert no stale overlay/interception | Planned |
| Original assets/continuous L chrome | actual screenshots and each local asset/callsite/rendered geometry checked against full Figma contexts | BLOCKED: required ZIP bytes unavailable |
| Retain auth/personnel/Table | source unchanged; relevant regression suite when routing implementation exists | Product sources unchanged; full regression NOT RUN |

Current B5 grants must be exactly:
resourceKind application, resourceId current app UUID, action menu.enter,
rowScope all, fields [].
Member replacement is memberIds []; current contract offers no member directory.
Do not treat permission groups as catalog grouping or infer data permissions.

The component fixtures mock only HTTP boundaries and cannot prove real B5
authorization, persistence or PostgreSQL/Redis semantics. The future real journey
must use isolated HTTPS/BFF/PostgreSQL/Redis, create and re-read an app, reopen
through live access, configure member/menu grants, and verify revocation across
actor Sessions. No production endpoint is involved.

