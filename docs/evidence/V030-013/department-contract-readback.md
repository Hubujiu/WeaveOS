# Accepted task ADR appendix A.5

Actual authorized connector readback: https://app.notion.com/p/3ee2f5a9e64881489017e2f38a4bb29f
Revision: 2026-10-03T10:50:22.720Z. New section explicitly marked 主负责人冻结.
The existing B5 inactive assignment ambiguity remains; no source text resolves it.

GET /api/v1/applications/{appId}/department-candidates is reserved to actual owner/Bootstrap,
with live Session/active/auth_version, same expected-actor guard, and no global personnel grant.
q is optional, TrimSpace and name prefix, max100 characters; pageSize1..50/default20;
opaque cursor binds actor/app/normalizedq, same scope/expiry checks as member candidates,
order name/id. data.items is exactly {id,label,parentId,status:"active"}, actual existing
department rows/hierarchy; standard meta.pagination. No full organization or permission DTO.
Definition reference defaults reuse these management candidates; Save must recheck/lock sources.
Ordinary record candidate permissions remain V015-owned; no widening this endpoint.
Tests cover authorization, cross-app, pagination/query/cursor, real deletion exclusion/minimal
response and source removed between candidate read and Save. No production change authorized.
