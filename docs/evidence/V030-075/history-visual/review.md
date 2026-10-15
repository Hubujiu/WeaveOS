# P6 actual browser visual review — 2026-10-11 19:03 UTC

Root personally inspected all three PNGs. Artifact11712810417, run38165815487/job114548377011; ZIP SHA256 59b6f8976e4ad627e403e2789e7e2db34b551df3697be07fe90c35e8dbd0fe8f verified. Report source1d6f7d3bcdc2c83b15350002ec083e2ba3685f93 fetched: tree00be3d0dfd2aaa6c5f92d67e6fcf034f3e7f74be matches localc9485553/remote7cfe4385 exactly.

- Desktop right history is340px, gap20; original record editable field and separate Save/Agree, card hierarchy visible. This one-field fixture cannot certify every field/layout.
- 390px mobile cards stack with internal vertical scrolling; stable IDs wrap; no body horizontal overflow. Existing narrow Shell horizontal navigation remains outside this slice.
- Exact historical decimal is preserved next to persisted current200.00; unsaved300.00 is excluded as required.
- **Found a real visual defect:** after opening comparison from a lower history detail, the content scroller retains its old offset. The comparison heading/top is clipped. Added an independent browser viewport/focus-return regression before fixing; current11 behavior cases passed but this new case is NOT RUN. No claim of full visual acceptance.
- PNG hashes: desktop38fbc644df465f4ae41bb08367d35e7f2a1a3deeaeb54ab9a9e001526215b4a1, comparison080c725c612c48721d9911579ac5d962169937392a1d2661ce0171bad5dd216d, mobile22bee7852052495d54c6f423d2006043464e369954936acf1bd4327684ee6314.
