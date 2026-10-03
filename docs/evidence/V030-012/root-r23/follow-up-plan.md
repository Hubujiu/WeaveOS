# Root R23 coordinator follow-up

Root read the published source and exact final-shell log at9508b75. The old immediate second-GET count assertion observed one request before the asynchronous reread completed. Root corrected the oracle by requiring a distinct normalized PATCH display (input20.105 -> authoritative20.11) and polling the same >=2 request condition. No success or reread requirement is weakened.

Root personally expanded the suite from9 to11 cases and added captures of create/detail/edit/read-error states. Run this frozen suite RED before implementation. Only RecordRoute.tsx is allowed to change unless Root identifies a further blocker.

## Fixes
1. Create-conflict refresh must work even without editor.record. Route it through requestSectionLeave. Continuing editing preserves current input. After explicit discard, close the unsaved editor, clear any saved notice, and remount/refresh RecordWorkspace at page1 with no old queryVersion. Never submit another write as part of refresh.
2. A read attempt must retain whether an actual write was previously confirmed. Add a boolean fact to PendingRead (or an equally explicit read-attempt field): ordinary detail or explicit refresh=false; post-write reread=true. Retry must preserve this fact, not default refreshRead to showSaved=true. A rejected write followed by a successful GET is never a saved write.
3. A failed read must stop the loading status. Keep the safe error and retry button, but do not simultaneously announce that it is still reading. The existing pendingRead plus readError state is enough; no second mutation state machine.
4. Entering edit mode should clear the previous saved notice so a prior operation's success is not presented as success of the new unsent edit.

Run all11 Shell cases plus the related component selection previously92/92, typecheck and build, task and repository checks. Preserve actual raw logs, screenshots and full outer commands in the host mount. Force-add exact sanitized .log files under this task's evidence because repository ignore patterns otherwise omit them; verify every manifest entry is committed. Prior final-checks/build.log was missing; do not fabricate it. The new run supplies fresh build evidence.

Publish the bounded code/evidence checkpoint when green by normal fast-forward; Root will review it. No test edits, full-product claims, main merge or deployment.
