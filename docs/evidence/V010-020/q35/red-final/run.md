# 本轮目标RED

产品基线 a2928991d005e694f69c75e31ba7b230bfc637b8，Windows / pnpm / Chromium，独立Vite127.0.0.1:4174 strictPort，保留用户4173审核窗口。命令：`pnpm exec playwright test --config .work/q35.config.ts personnel-arca.component.spec.ts`，第一次7项失败、第二次新增独立控制缺失断言9项全部失败；两次退出1。

实际原因：身份存在table(1≠0)；四种少/零数据无填充行/原自定义scroll；原checkbox要求Motion BUTTON实际INPUT；页脚48≠33、页大小选择和页码缺失；resize/reorder按钮0≠1；原排序菜单缺失。加载和fixture成功、无语法/依赖/网络失败。此时产品、依赖与配置尚未改动。

tested-source.zip保存当时精确测试、原产品、配置、锁文件；sha256.json可独立复验全部字节。先推送真实RED后实现，旧Q34期望按Q35正式来源替代并保留业务断言，不能把当前实现作为新预期。
