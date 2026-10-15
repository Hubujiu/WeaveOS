# Actual CI visual recheck

Root personally viewed both native PNGs on 2026-10-11 17:27 UTC, not merely metadata.
Artifact 11709204962, run38159000869/job114528130194; ZIP SHA256 464c10851d6181666fec82a31d9741af3306dcb09823cc7f5d599115a8979ffe verified from downloaded bytes.
Report source bc512c2ed89e689f8d460ac6ab49a6b757f14aad was fetched through Git. Its tree 00103cb9b8e0ec00888ee472193625ba6290e014 equals local7d115d9/remote ca5be0a5 exactly.

- Small-screen 390x844: first flow column and task text are no longer crushed. Table has internal horizontal overflow; pager fits the card; no outer overflow in the independent behavior assertion. Source Arca empty grid rows remain; no vendor change.
- Desktop readonly detail: field background is now approved #f5f5f5, exact decimal remains readable and unchanged. Actual computed font matches the workspace and includes Noto Sans SC (previous font assertions already passed). No unsupported claim that a font defect was fixed.
- This one-field readonly fixture does not demonstrate the full processing screen, editing layout, actions, timeline or every viewport. Those remain separate work and visual acceptance.
- Eight original node Save interaction cases passed in the same CI, but only readonly detail/inbox screenshots were attached. Do not claim these PNGs depict Save. One independently added browser-refresh warning case failed and was subsequently fixed; its new GREEN is still pending.
