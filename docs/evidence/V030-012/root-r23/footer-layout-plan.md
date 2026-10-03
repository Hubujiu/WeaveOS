# Root R23 footer and layout review

Root personally inspected the committed screenshots and source at7da95ebc. The11 cases pass, but source review found that binding RecordForm.onDiscard directly to closeEditor fixed recursive guard cleanup while making the visible footer discard button bypass confirmation. Root added a dedicated footer case and two layout assertions; the Shell suite is now14 cases. Root also migrated all four test fixture callers to the explicit callback contract.

## Frozen implementation
Only RecordForm.tsx, RecordRoute.tsx and record-scoped applications.css may change.
- RecordForm gets REQUIRED onRequestDiscard():void for a user's footer button intent. Keep onDiscard():void as the already-confirmed prepareLeave cleanup. The footer invokes onRequestDiscard; the leave controller invokes onDiscard only after a confirmed discard. Document the distinction at the props/type boundary. Do not change either callback's meaning implicitly or introduce an adapter.
- RecordRoute passes onRequestDiscard=guardedClose and onDiscard=closeEditor. This prevents both recursive confirmation and silent dirty input loss.
- Keep the original navbar/sidebar/background. Make .record-route a bounded flex-column child (flex:1,min-height:0,min-width:0), with its existing workspace filling the remaining area. Do not patch the vendor table, force a click, add sleep, or hard-code a viewport height.
- Visually hide the existing fieldset legend with a standard scoped screen-reader-only rule, retaining its accessible name. It currently leaks the unnecessary text '记录操作' into the visible layout.
- In record dialogs only, remove redundant inner surface decoration and use the existing form/button tokens with at least12px space between fields and footer and8px between buttons. Use scoped flex/gap rules, no global palette or navigation redesign. Preserve input containment and existing responsive sizing.

## Verification
Run the14 Root Shell cases before implementation and retain actual RED (new-prop type mismatch is not behavioral RED). Then implement only the above and run14 GREEN, the related92 selection and all record/editor fixtures, typecheck/build/task/repo checks.
Root owns every test and fixture; do not edit or weaken any test. Root syntax-checked the updated test file.
Capture the existing five states again plus the new unobscured list screenshot. Save actual raw logs/exit codes/outer commands; force-add only sanitized task logs and verify manifests.
Publish the bounded review checkpoint after GREEN by normal fast-forward; no main merge/deployment. Real-stack verification remains blocked on the user's pending CA permission, so do not attempt a trust change or real runner here.
