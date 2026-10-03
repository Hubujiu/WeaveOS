# V030-014 mounting interface, read-only checkpoint review

Inspected remote `task/V030-014-form-designer` at
`cb08774ed00e715402e45c31228a508e86d93055` on 2026-10-03. This was
read-only; its branch and files were not merged or edited.

`apps/web/src/applications/forms/index.ts` exports `ApplicationStructurePanel`
with `{ appId, onOpenForm?, onDirtyChange? }` and `FormDesigner` with
`{ appId, viewId, onDirtyChange?, onBack? }`. The V030-012 `FormSlots` and
`ApplicationWorkspace` can mount them once the module is frozen.

Before mounting, the form module must share V030-012's actor protocol:

- `formRequest` currently receives no verified actor ID and sends no
  `X-Expected-Actor-Id` on structure, definition, write, or operation routes.
- `ApplicationStructurePanel` retains an uncertain operation only in local
  `pending` state; route unmount on 401 can discard its key and request packet.
- Form writes and operation queries need the same actor-keyed in-memory recovery,
  preflight identity check, 409 Shell masking, and 401 SPA reauthentication used
  by the catalog. A successful response must prove the expected status, envelope,
  and result before clearing an uncertain write.

The V030-013 candidate backend guard is present; mounting an unbound form client
would leave this frontend slice with a separate session-switch behavior. The
V030-014 owner retains its forms/API files, dependency and lock ownership;
V030-012 owns only the Shell/router adapter after root freezes a compatible SHA.
