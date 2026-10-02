# Arca Data Table source consumption

Source: [Hubujiu/arca-ui](https://github.com/Hubujiu/arca-ui/tree/c0319d887e10775c8968a7d6a451f248858726ec/src/components/motion/table), fixed commit `c0319d887e10775c8968a7d6a451f248858726ec`. User Q35 explicitly requested direct source, original styles/motion, and all example column and paging interactions. The 22 TS/TSX files are the official table's source closure, including Checkbox, DropdownPanel, Select, Button, and ease/touch/utils. The upstream snapshot did not contain a LICENSE file; no license designation is inferred here.

Permanent exact upstream bytes, hashes, necessary project diff and dependency integrities are in [Q35 evidence](../../../../../docs/evidence/V010-020/q35/source.md). The readable `vendor.patch` compares original source with these project copies. Upstream files and approval records remain unchanged.

Project adaptations preserve original component markup, class utilities and Motion timing:

- Relative import paths and table/portal `.arca-source` scope; Chinese accessible names and page-size keyboard/focus support.
- Controlled server page/index/count bridge, without slicing a server page twice or resetting server page for local sorting. Authoritative shrinking totals recover the actual valid page.
- ResizeObserver body fill, inert `aria-hidden` blank rows and full-height original scrollbar viewport. Blank rows never enter source data, selection, virtualizer, totals or API calls.
- Column menus ignore their own scrolling while retaining external-scroll/resize dismissal, fixing Firefox focus-induced premature close. Direction keys/Home/End, native Enter/Space opening, visible focus and Escape/selection focus return supplement original accessibility without changing panel motion.

`arca.css` uses the original inline theme, explicit-class dark variant, base foreground/background/border/outline/antialiasing, light variables and el-scrollbar rules, Tailwind 4.3.3 preflight and compiled utilities, plus original Geist font. Element preflight, utilities, variables and font usage are scoped to table/portal wrappers; global font-face registration and Tailwind theme/property definitions are declarations. Existing Noto Sans SC remains the Chinese fallback. Source radius aliases are local so Button and pager keep their original dimensions. Unrelated workspace element rules exclude this scope, preventing legacy styles from overriding source controls.

`PersonnelTable.tsx` is the business bridge; `personnel-table.css` contains outer business-card geometry, typography for existing business content and toolbar layering. Identity uses the existing template card list. Default page size is 20 and the six allowed values are 5/10/20/25/50/100; sorting affects the currently fetched page only. These are the confirmed R3 §5.8/Q25 rules, rather than additional upstream capabilities.
