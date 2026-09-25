# V010-006 来源映射与组件缺口

2026-09-25 12:01 Asia/Shanghai 读取。本文只记来源观察与阻塞，TDD:N/A；尚无页面实现或产品 GREEN。

| 来源 | 实际观察 | 实施影响 |
| --- | --- | --- |
| [v0.1.0 PRD](https://app.notion.com/p/3e52f5a9e64880cd951fea95bdde9cdd) | 注册必须有邀请码、URL 自动填入；密码须含大小写字母、数字、特殊符号，但无长度下限；登录与注册要显示密码、反馈错误/Loading；键盘可用、提交防重复；注册成功返回登录页且不建 Session。最后编辑 2026-09-24T12:15:05.462Z，状态需求编写中/未冻结。 | 前端用例独立来自 PRD，后端仍需重新校验。 |
| [登录原型 26:423](https://www.figma.com/design/9UE263yT1anH3KG9qBjXiT?node-id=26-423) | 已读取截图和设计上下文：页顶 DocWeave 导航；中心卡片，账号/密码、主按钮、注册链接。画布未展示密码显示切换、错误和 Loading 态。 | 批准原型覆盖基础布局；交互仍按 PRD 验证。 |
| [注册原型 26:439](https://www.figma.com/design/9UE263yT1anH3KG9qBjXiT?node-id=26-439) | 已读取截图和设计上下文：账号、密码、确认密码、四段强度条、主按钮、返回登录；**没有邀请码输入框**，帮助文字写“至少 10 位，组合字母、数字与符号”。 | 用户本轮已确认以 PRD 为准：加入邀请码字段，去掉长度提示。 |
| [组件库消费规则](https://github.com/Hubujiu/React-/blob/HEAD/docs/consumer.md) | 42 个基础组件的 2026-09-17 批准版本固定于 `d139c95e679742bfff3dd46cb2918bf5d9ac3fc6`；收藏原件不等于批准；不匹配则报告 COMPONENT_GAP。 | 只对照已批准的具体版本，不能自行改来源组件。 |
| `watermelon-floating-input` | registry 状态 approved；实际导出 `FloatingInput`，采用浮动标签、较大圆角与内部间距。Figma `Input` 为独立静态标签、32px 高、6px 圆角。 | COMPONENT_GAP：输入控件外观和标签交互不匹配；批准记录许可字段仍为 unknown。 |
| `beui-button-base` | registry 状态 approved；实际 `Button` 的 sm 规格为 32px 高、胶囊圆角；Figma `Button` 32px 高、6px 圆角。 | COMPONENT_GAP：按钮几何不匹配；仅调整业务外层宽度无法修正内部圆角。 |

账号规则补充来自本轮用户确认：去首尾空格、禁止账号中间空格、不转小写，大小写敏感唯一。此处不把 Notion 草案的旧 `lower(account)` 当作现行要求。

代码/测试状态：NOT RUN。只读来源核对及文档排版不是行为 TDD 阶段。取得方案后先为账号空格、URL 邀请码、密码规则、成功/失败/Loading、恢复/退出写独立期望和测试，实际观察目标 RED，再实施对应页面。
