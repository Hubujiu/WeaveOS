# 账号字符计数与规范化 RED → GREEN

2026-09-26 重新实际读取Notion auth.users.account/ account_key字段（编辑2026-09-25T08:31:05.040Z/08:31:04.255Z）；批准语义为1–254字符、仅去首尾普通空格、大小写敏感。PostgreSQL varchar/char_length按Unicode字符计数，不按UTF-16单元；非普通空白并未获授权折叠。

bab0b3f先保存 account-character-red.spec.ts/ login-before-account-character.tsx，运行 `pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep 'approved account dictionary'` 退出1。真实键盘输入254个非BMP字符仅保留127个（HTML maxlength按UTF-16）；NBSP包围的账号请求被.trim()改为Alice，预期原值。两个均为有效目标RED。

移除按UTF-16截断的maxlength，提交前用Array.from计Unicode字符；前端仍禁止普通空格，因此无需再用.trim()改变合法账号。登录/注册保持原大小写及合法字符。完整20项组件测试41.1s退出0，typecheck/build退出0。Q13密码特殊符号边界仍待答复，此修复不依赖它。真实后端三浏览器仍待007。
