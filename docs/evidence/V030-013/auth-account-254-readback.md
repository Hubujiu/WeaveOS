# Actual source readback for the source-hook regression

+Here is the result of "fetch" for the Page with URL https://app.notion.com/p/3e52f5a9e64880cd951fea95bdde9cdd as of 2026-09-30T01:30:29.407Z:
<page url="https://app.notion.com/p/3e52f5a9e64880cd951fea95bdde9cdd">
<ancestor-path>
<parent-data-source url="collection://73e57a4a-3b61-4a72-88a0-a1f5b3887865" name="迭代与 PRD"/>
<ancestor-2-database url="https://app.notion.com/p/a6a6c92db4cb41fc98c917bd465cc0e2" title="迭代与 PRD"/>
<ancestor-3-page url="https://app.notion.com/p/3e52f5a9e6488037975bc303bee464b5" title="PRD"/>
<ancestor-4-page url="https://app.notion.com/p/3e42f5a9e64880ae9cf5ebbd2088d773" title="项目 - 企业管理系统"/>
</ancestor-path>
<properties>
{"date:排期:is_datetime":0,"url":"https://app.notion.com/p/3e52f5a9e64880cd951fea95bdde9cdd","优先级":"P0","关联功能":["https://app.notion.com/p/3e52f5a9e64880a3aaa1caa116d73996","https://app.notion.com/p/3e52f5a9e64881428779fb73a2c8fc51"],"已冻结":"__NO__","最后更新时间":"2026-09-30T01:30:29.407Z","版本号":"v0.1.0","版本类型":"MVP","迭代/需求名称":"完成用户登录态","迭代状态":"已上线归档"}
</properties>
<iconMetadata>null</iconMetadata>
<content>
<callout icon="🧩" color="blue_bg">
	**v0.1.0 完成 Web 端最小认证闭环：邀请码注册 + 账号密码登录 + 登录态。**
	本版本只解决“用户如何获得账号、如何证明身份、当前请求是否已登录”。角色、权限、部门权限、文档 ACL 等授权能力全部后置。
</callout>
## 1. 背景与目标
### 本期目标
- 用户通过 **账号 + 密码 + 有效邀请码** 完成注册。
- 注册页支持从 URL 携带邀请码并自动填入邀请码字段。
- **没有邀请码或邀请码校验不通过时不允许注册。**
- **每个邀请码只能成功使用一次；注册成功后邀请码立即失效，不得再次用于注册。**
- 已有用户通过 **账号 + 密码** 登录。
- 登录成功后建立可恢复的 Web 登录态，页面刷新后仍可识别当前用户。
- 未登录用户无法访问需要登录的页面与接口。
- 用户可主动退出，使当前登录态失效。
- 普通用户不提供“忘记密码 / 自助找回密码”；密码只能由管理员重置，重置后的密码固定为 `Abc@123456`。
- V0.1.0 初始化一个 **Bootstrap Admin** 超级管理员用于测试与系统初始化；其权限视为 `ALL`，可生成邀请码和执行管理员密码重置。通用权限模型仍不属于本版本。
- Session 空闲超时时间为 **1 小时**，采用滑动续期：每次成功的已认证活动将有效期重新延长至当前时间后 1 小时。
- 记录最小认证审计日志，包括登录 IP、设备 / User-Agent、时间、登录结果等信息。
### 非目标（Non-goals）
本期明确不完成：
- 通用角色、身份模板、权限点、RBAC / ABAC / ACL；V0.1.0 仅保留 Bootstrap Admin 的 `ALL` 特例，不建设完整权限系统。
- 手机端、桌面客户端或其它 Native Client；本期只有 Web。
- 多端设备管理、登录设备管理中心、踢设备下线。
- 用户自助忘记密码、邮件找回密码、短信找回密码。
- 登录失败次数限制、连续失败锁定、验证码。
- 登录接口限流。
- MFA、OTP、Passkey。
- OIDC / SSO / SAML / LDAP / AD。
- Google / GitHub / Microsoft / 企业微信 / 飞书等第三方登录。
- 完整安全审计中心与审计查询后台。
- AI 参与密码校验、登录态校验或认证决策。
### 成功指标 / 上线门槛
<table fit-page-width="true" header-row="true">
<tr>
<td>指标</td>
<td>目标</td>
<td>验证方式</td>
</tr>
<tr>
<td>邀请码注册</td>
<td>无有效邀请码无法创建账号；URL 携带邀请码可自动填入</td>
<td>注册流程自动化测试 + 人工验收</td>
</tr>
<tr>
<td>核心登录流程</td>
<td>账号密码登录验收用例 100% 通过</td>
<td>自动化测试 + 人工验收</td>
</tr>
<tr>
<td>登录态恢复</td>
<td>有效期内刷新页面后仍能识别当前用户</td>
<td>刷新页面测试</td>
</tr>
<tr>
<td>未登录访问保护</td>
<td>受保护页面 / API 不可匿名访问</td>
<td>接口测试 + 路由测试</td>
</tr>
<tr>
<td>密码安全</td>
<td>用户密码与管理员重置后的密码均不以明文形式落库、记录或返回</td>
<td>数据库 / 日志 / 接口检查</td>
</tr>
<tr>
<td>最小审计</td>
<td>登录事件可记录 IP、设备 / User-Agent、时间与结果</td>
<td>日志检查</td>
</tr>
</table>
## 2. 需求范围（Scope）
<columns>
	<column ratio="50">
		<callout icon="✅" color="green_bg">
			**In-Scope｜本期必须交付**
			- Web 登录页与注册页。
			- 邀请码注册。
			- URL 邀请码自动填充。
			- 账号 + 密码登录。
			- 登录成功 / 失败反馈。
			- 登录态建立、恢复、校验、过期与退出。
			- 获取当前登录用户基础信息。
			- 受保护前端路由与后端接口。
			- 未登录请求统一按未登录处理。
			- Bootstrap Admin 初始化账号，权限视为 `ALL`。
			- Bootstrap Admin 生成一次性邀请码。
			- 管理员密码重置：重置为 `Abc@123456`。
			- 1 小时空闲超时 + 滑动续期 Session。
			- 登录 IP、设备 / User-Agent、时间、结果的认证日志。
		</callout>
	</column>
	<column ratio="50">
		<callout icon="⛔" color="red_bg">
			**Out-of-Scope｜明确不在本期**
			- 角色 / 权限 / ACL。
			- 用户自助忘记密码。
			- 登录限流。
			- 失败次数限制 / 账号自动锁定。
			- MFA / OTP / Passkey。
			- OIDC / SSO / SAML / LDAP。
			- 第三方登录。
			- Mobile / Desktop Native Client。
			- 多端设备管理。
			- 完整审计中心。
		</callout>
	</column>
</columns>
### 依赖与前置条件
- **用户基础数据**：至少包含稳定用户 ID、唯一账号、密码凭据和账号状态。
- **邀请码数据**：邀请码由具备生成能力的员工产生；V0.1.0 暂不建设通用的邀请码生成权限模型，由 Bootstrap Admin 直接生成。每个邀请码初始可用、无时间过期限制，直到首次成功注册后立即失效。
- **Bootstrap Admin**：系统初始化时创建一个超级管理员，权限视为 `ALL`，用于测试、生成邀请码与管理员密码重置；这不是正式 RBAC 模型。
- **认证边界**：本期只判断“是否已经登录”；除 Bootstrap Admin 的系统初始化特例外，不判断登录后的业务权限。
- **技术实现边界**：Session、Redis、Cookie、密码凭据存储等由 <mention-page url="https://app.notion.com/p/3e52f5a9e64881c2b972d71ab1fc2e93"/> 决定。
- **客户端范围**：V1 仅支持 Web。
## 3. 用户流程与原型
### 场景 A：邀请码注册
1. 用户进入注册页。
2. 如果入口 URL 携带邀请码，页面自动将邀请码填入邀请码字段。
3. 用户填写账号和密码；邀请码也必须存在。
4. 前端完成基础输入校验后提交。
5. 服务端校验账号唯一性、密码规则，以及邀请码是否有效且未使用。
6. 邀请码缺失、无效或已使用 → 拒绝注册。
7. 注册成功 → 创建账号，并将该邀请码原子地标记为已使用，使其立即失效。
8. 注册失败不得消耗邀请码；并发使用同一邀请码时只能有一个注册成功。
9. **注册成功后不自动建立 Session，返回登录界面，用户必须重新输入账号和密码完成登录。**
### 场景 B：账号密码登录
1. 用户进入登录页。
2. 输入账号和密码。
3. 服务端校验账号状态与密码凭据。
4. 成功后建立登录态。
5. 客户端获取当前用户基础信息并进入目标页面。
6. 后续受保护请求持续校验登录态。
### 场景 C：未登录 / 登录态过期
- 匿名用户进入受保护页面 → 进入登录流程。
- 匿名请求受保护 API → 不返回受保护数据。
- 登录态过期 / 无效 → 当前请求按未登录处理并重新进入登录流程。
### 场景 D：忘记密码
- 普通用户没有自助找回 / 重置流程。
- 用户需联系管理员。
- 管理员执行重置后，新密码固定为 `Abc@123456`。
- 数据库仍只保存密码安全哈希，不保存明文 `Abc@123456`。
### 场景 E：Bootstrap Admin
- 系统初始化时准备一个 Bootstrap Admin 测试账号。
- Bootstrap Admin 权限视为 `ALL`，无需依赖本期尚未建设的权限系统。
- Bootstrap Admin 可生成一次性邀请码。
- Bootstrap Admin 可执行管理员密码重置。
- 后续正式权限系统上线后，再迁移或替换该 Bootstrap 特例。
### 登录 / 注册页原型
<callout icon="🎨" color="green_bg">
	**原型已确认**
	- 注册页：[Figma · WaveOS Register](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=40-2)
	- 登录页：[Figma · WaveOS Login](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=13-2)
	- 原型文件更换：2026-09-26 用户指定 WaveOS 文件；Q12 已确认登录页“记住账号”“忘记密码？”及 Google/Microsoft/GitHub 控件保持可见、置灰、不可点击，本期不实现其功能。对应 Figma Login `13:2` 已同步禁用状态。
</callout>
本轮实现次序（2026-09-25 用户确认，2026-09-26 更换原型并答复 Q12）：登录与注册页面先按上述 WaveOS Figma 原型保持视觉一致，动画与可复用组件的进一步打磨后续处理；不更改已批准组件库的源码或审批范围。登录页的忘记密码、第三方登录与记住账号控件保留外观、置灰且不可点击；不提供自助找回、第三方认证或记住账号的数据保存行为，后续版本再调整。
### 最低交互元素
**登录：**
- 账号输入框。
- 密码输入框。
- 显示 / 隐藏密码。
- 登录按钮。
- 错误反馈与 Loading。
**注册：**
- 账号输入框；前端不允许账号包含普通空格。
- 密码输入框。
- 密码强度反馈；不显示密码长度下限提示。
- 邀请码输入框。
- URL 携带邀请码时自动填入。
- 注册按钮。
- 邀请码缺失 / 无效、账号重复等错误反馈。
## 4. 需求明细
<table fit-page-width="true" header-row="true">
<tr>
<td>ID</td>
<td>需求 / 规则</td>
<td>验收结果</td>
</tr>
<tr>
<td>FR-001</td>
<td>使用账号 + 密码登录</td>
<td>正确账号密码可建立登录态；错误凭据不能建立登录态</td>
</tr>
<tr>
<td>FR-002</td>
<td>账号必须唯一</td>
<td>重复账号不能完成注册</td>
</tr>
<tr>
<td>FR-003</td>
<td>注册必须填写账号 + 密码 + 有效且未使用的邀请码；每个邀请码只能成功使用一次</td>
<td>没有邀请码、邀请码无效或已使用时不能创建账号；注册成功后该邀请码立即失效</td>
</tr>
<tr>
<td>FR-004</td>
<td>URL 可携带邀请码并自动填充注册表单</td>
<td>通过邀请链接进入时无需用户手动复制邀请码；服务端仍重新校验邀请码</td>
</tr>
<tr>
<td>FR-005</td>
<td>成功登录后建立登录态</td>
<td>后续请求可识别当前用户</td>
</tr>
<tr>
<td>FR-006</td>
<td>Session 采用 1 小时空闲超时并滑动续期</td>
<td>每次成功的已认证活动将 Session 有效期刷新为当前时间后 1 小时；连续 1 小时无有效认证活动则过期</td>
</tr>
<tr>
<td>FR-007</td>
<td>未登录访问受保护资源</td>
<td>页面进入登录流程；API 不返回受保护数据</td>
</tr>
<tr>
<td>FR-008</td>
<td>登录态过期 / 无效</td>
<td>不能继续访问受保护资源，并重新进入登录流程</td>
</tr>
<tr>
<td>FR-009</td>
<td>主动退出</td>
<td>当前登录态立即失效</td>
</tr>
<tr>
<td>FR-010</td>
<td>禁用账号不可完成新登录</td>
<td>账号状态为 Disabled 时拒绝认证</td>
</tr>
<tr>
<td>FR-011</td>
<td>不提供用户自助忘记密码</td>
<td>登录 / 注册界面不提供自助找回密码闭环</td>
</tr>
<tr>
<td>FR-012</td>
<td>管理员可重置密码</td>
<td>重置后用户密码变为 `Abc@123456`，存储层只保存安全哈希</td>
</tr>
<tr>
<td>FR-013</td>
<td>不做登录限流和失败次数限制</td>
<td>V1 不因连续失败自动锁定账号，也不实现认证接口限流</td>
</tr>
<tr>
<td>FR-014</td>
<td>记录最小登录审计信息</td>
<td>登录事件至少可记录账号 / 用户标识、IP、设备或 User-Agent、时间、结果；不得记录密码</td>
</tr>
<tr>
<td>FR-015</td>
<td>V1 仅支持 Web</td>
<td>不要求 Native App、多端登录策略或设备管理能力</td>
</tr>
<tr>
<td>FR-016</td>
<td>初始化 Bootstrap Admin</td>
<td>Bootstrap Admin 权限视为 `ALL`，可生成邀请码并执行管理员密码重置；无需本期实现通用权限模型</td>
</tr>
<tr>
<td>FR-017</td>
<td>注册成功后返回登录页</td>
<td>注册不会自动建立 Session；用户必须重新输入账号和密码登录</td>
</tr>
<tr>
<td>FR-018</td>
<td>密码仅允许标准 ASCII 可打印字符，且同时包含大写英文字母、小写英文字母、数字和可见 ASCII 标点</td>
<td>缺少任一字符类型时不能完成注册或设置密码；固定重置密码 `Abc@123456` 满足该规则</td>
</tr>
</table>
### 状态与规则
#### 用户状态
- **Active**：允许登录。
- **Disabled**：拒绝新登录。
#### 注册规则
- 账号、密码、邀请码均为必填。
- 邀请码自动填入只是交互便利，**不能替代服务端校验**。
- 无邀请码时不允许注册。
- 每个邀请码只能成功使用一次；注册成功后立即失效。
- 注册失败不得消耗邀请码；同一邀请码发生并发注册时最多允许一个请求成功。
- 账号去除首尾普通空格后长度为 1–254 个字符，账号中不允许普通空格；保留大小写，`Alice` 与 `alice` 是两个不同账号。登录按相同规则精确匹配。
- 账号必须唯一。
- 密码仅允许标准 ASCII 可打印字符 U+0020–U+007E；至少包含 A–Z、a–z、0–9、可见 ASCII 标点各 1 个。特殊符号范围为 U+0021–U+002F、U+003A–U+0040、U+005B–U+0060、U+007B–U+007E；普通空格可保留但不计作特殊符号，非 ASCII 与控制字符拒绝。密码不去首尾空格、不做 Unicode 规范化，不额外设置长度下限。依据：2026-09-26 用户 Q13 答复“都只允许标准 ASCII 可打印符号”。
- 本版本不额外规定字符长度下限；如后续增加长度要求，通过密码策略规范更新。
- 管理员固定重置密码 `Abc@123456` 满足上述字符类型规则。
#### 登录规则
- 账号、密码不能为空。
- 账号不存在和密码错误使用统一凭据失败提示，不要求记录失败次数。
- 不做认证接口限流。
- 不做连续失败自动锁定。
- 网络错误与凭据错误必须区分。
- 密码不得出现在客户端日志、服务端日志或错误响应中。
#### 密码重置
- 普通用户不能自助重置密码。
- 管理员重置后的固定密码为 `Abc@123456`。
- 重置动作不得把明文密码写入数据库或日志。
- 密码重置使该用户全部旧 Session 在后续认证校验中失效。
#### Session 生命周期
- Session 空闲 TTL：**1 小时**。
- 采用**滑动续期**：每次成功的已认证活动将服务端 Session 的过期时间重新设置为当前时间后 1 小时。
- 连续 1 小时无成功的已认证活动则 Session 过期。
- V0.1.0 不额外设置绝对最长 Session 生命周期；主动退出、账号禁用或服务端失效仍可提前终止 Session。
## 5. 数据、安全与 AI 边界
### 最小逻辑实体
- User。
- Credential。
- Invitation Code。
- Session。
- Authentication Log。
### 最小审计日志
登录相关日志至少保留：
- 用户 / 账号标识。
- 登录结果。
- 客户端 IP。
- 设备信息或 User-Agent。
- 事件时间。
仅当前有效 Bootstrap Admin 可读取认证日志；本期不建设通用权限模型或审计管理后台。日志保留 1 年，按月转入单独的冷归档库，日常认证日志查询不索引该归档库；满 1 年自动删除到期数据。当前由用户承担视觉、风险和运行恢复的最终人工验收；本地自动归档与删除仍须以实际测试和演练证明，未执行时不得声称完成。生产运行责任与发布条件另行确定。
不得记录：
- 明文密码。
- 完整 Session ID / Token。
- 可直接复用的认证凭据。
### AI 边界
- AI 不参与注册邀请码校验、密码校验、Session 校验或管理员重置决策。
- 密码、Session、密钥不得发送给 AI 服务。
## 6. 非功能需求
- **安全**：密码必须使用适合密码存储的不可逆哈希；浏览器登录态的最终真实性由服务端判断。
- **性能**：登录态验证为高频路径，具体 Session 存储和缓存策略由 ADR 决定。
- **客户端**：本版本仅要求 Web 浏览器。
- **开发环境**：本轮仅在本地开发，并用 WSL 中的 Docker 模拟 Linux 运行环境；不等同于公网或真实环境发布批准。
- **可观测性**：可区分注册成功 / 失败、登录成功 / 失败、退出、Session 失效与服务异常。
- **安全取舍**：V1 明确不实现登录限流、失败次数限制与自动锁定；这些不是遗漏，而是本期范围决定。
- **UX**：登录 / 注册状态明确；输入框有 label；支持键盘操作；提交期间防止同一按钮重复提交。
## 7. 验收标准
### Definition of Ready
- [x] 注册、登录、管理员重置范围已明确
- [x] 邀请码是注册强制条件
- [x] URL 邀请码自动填充行为已明确
- [x] V1 只有 Web
- [x] 无限流、无失败次数限制
- [x] 最小认证日志字段已明确
- [x] 新 WaveOS 登录原型的非目标控件处理已确认（Q12：可见、置灰、不可点击，Figma 已同步）；注册原型及核心表单已确认
- [x] <mention-page url="https://app.notion.com/p/3e52f5a9e64881c2b972d71ab1fc2e93"/> 已接受并同步当前方案
### Definition of Done
- [x] 有效邀请码 + 合法账号密码可以完成注册
- [x] 没有邀请码不能注册
- [x] 无效邀请码不能注册
- [x] 已使用的邀请码不能再次注册
- [x] 注册失败不会提前消耗邀请码
- [x] 并发使用同一邀请码时最多一个注册成功
- [x] 邀请链接中的邀请码可自动填入
- [x] 重复账号不能注册
- [x] 正确账号密码可登录
- [x] 错误凭据不能建立登录态
- [x] Disabled 用户不能完成新登录
- [x] 刷新后有效登录态可恢复
- [x] 未登录用户不能访问受保护资源
- [x] 登录态过期后不能继续访问
- [x] 退出后当前登录态失效
- [x] 普通用户无自助找回密码流程
- [x] 管理员重置后密码为 `Abc@123456`
- [x] 注册成功后返回登录页且不会自动建立 Session
- [x] Session 连续 1 小时无活动后过期，正常认证活动可滑动续期
- [x] Bootstrap Admin 可生成一次性邀请码并执行管理员密码重置
- [x] 密码数据库中不存在明文
- [x] 登录日志包含 IP、设备 / User-Agent、时间与结果
- [x] 日志不包含密码或完整敏感凭据
- [x] 核心认证路径具备自动化测试
### 最小验收用例矩阵
<table fit-page-width="true" header-row="true">
<tr>
<td>编号</td>
<td>场景</td>
<td>预期</td>
</tr>
<tr>
<td>AC-01</td>
<td>有效邀请码 + 新账号 + 合法密码</td>
<td>注册成功</td>
</tr>
<tr>
<td>AC-02</td>
<td>没有邀请码</td>
<td>拒绝注册</td>
</tr>
<tr>
<td>AC-03</td>
<td>无效邀请码</td>
<td>拒绝注册</td>
</tr>
<tr>
<td>AC-04</td>
<td>URL 带邀请码进入注册页</td>
<td>邀请码自动填充，并在提交时由服务端重新校验</td>
</tr>
<tr>
<td>AC-04A</td>
<td>使用邀请码成功注册后再次使用同一邀请码</td>
<td>第二次注册被拒绝</td>
</tr>
<tr>
<td>AC-04B</td>
<td>两个请求并发使用同一邀请码</td>
<td>最多一个注册成功，邀请码只消费一次</td>
</tr>
<tr>
<td>AC-05</td>
<td>正确账号 + 正确密码</td>
<td>登录成功并建立登录态</td>
</tr>
<tr>
<td>AC-06</td>
<td>错误账号或密码</td>
<td>登录失败，不建立登录态</td>
</tr>
<tr>
<td>AC-07</td>
<td>管理员执行密码重置</td>
<td>新密码为 `Abc@123456`，数据库只保存哈希</td>
</tr>
<tr>
<td>AC-08</td>
<td>已登录用户持续产生有效认证活动</td>
<td>Session 过期时间持续滑动为最后一次有效活动后 1 小时</td>
</tr>
<tr>
<td>AC-08A</td>
<td>连续 1 小时没有有效认证活动</td>
<td>Session 过期，后续请求按未登录处理</td>
</tr>
<tr>
<td>AC-09</td>
<td>匿名访问受保护 API</td>
<td>不返回受保护数据</td>
</tr>
<tr>
<td>AC-10</td>
<td>登录事件产生</td>
<td>日志可看到 IP、设备 / User-Agent、时间和结果，且无密码</td>
</tr>
</table>
## 8. 风险与明确取舍
<table fit-page-width="true" header-row="true">
<tr>
<td>取舍 / 风险</td>
<td>当前决定</td>
</tr>
<tr>
<td>固定重置密码 `Abc@123456` 仍是公开固定凭据</td>
<td>虽然满足当前字符类型要求，但固定默认密码仍属于已知安全债务；后续可替换为一次性随机密码 / 强制改密流程</td>
</tr>
<tr>
<td>无登录限流、无失败次数限制</td>
<td>V1 明确不实现；后续安全版本可补充</td>
</tr>
<tr>
<td>邀请码生命周期规则</td>
<td>Bootstrap Admin 可生成；V0.1.0 不做通用生成权限模型；邀请码无时间过期限制，首次成功注册后立即失效</td>
</tr>
<tr>
<td>第三方登录 / 企业 SSO</td>
<td>V1 不引入；未来通过可替换认证边界扩展</td>
</tr>
</table>
## 9. 发布与回滚
- 发布前执行注册、登录、Session 恢复、未登录保护、退出、管理员重置、认证日志 Smoke Test。
- 新项目无历史账号迁移。
- 密码字段只保存安全哈希。
- 登录态无法确认是否合法时采用 **Fail Closed（视为未登录）**。
- 回滚不得使已经失效的 Session 重新生效。
### 本地实现与验收记录（2026-09-27）
已合入的002–007提供注册/登录/会话/管理员/页面及真实产品测试；008在[PR11](https://github.com/Hubujiu/WeaveOS/pull/11)补充受限日志读取、冷归档/删除、同制品运行与恢复。85c2ee1本机Linux镜像完整验收通过，包含25项真实API、30项三浏览器及运行/恢复/故障/TLS测试；[具体材料](https://github.com/Hubujiu/WeaveOS/blob/6eaf36335c048620a8e348b6fb577eb613d8a04f/docs/evidence/V010-008/review.md)保留截图、镜像摘要、扫描、恢复约9.5秒和快照后数据丢失边界。审阅资料head c3dbfa5的[完整产品/运行CI](https://github.com/Hubujiu/WeaveOS/actions/runs/36258050989)与Go/浏览器/治理检查已全部通过；第二轮本机源6eaf363的74项运行检查通过、恢复8203ms，材料见[最终审阅包](https://github.com/Hubujiu/WeaveOS/blob/c3dbfa5/docs/evidence/V010-008/review.md)。用户于2026-09-27在Q10对MAN-UI、MAN-OPERATIONS、MAN-RISK分别实际填写“确认”；Q14原文为“接受基础镜像和工具的漏洞，不用帮他们修复”。确认范围为上述具体材料与Q5/Q6的本地合成数据开发：接受当前WaveOS视觉/交互及已说明的恢复、同机备份、命令采样监控、自签名TLS局限；接受清单内基础镜像/上游工具残余漏洞，不要求代上游修复。固定重置密码、无限流/锁定/绝对会话期限仍按当前产品规则保留。不会自动豁免本项目应用依赖、未来新增风险或生产环境；完整扫描继续留档。最终head 8fb82b2全部CI通过后，PR11已于2026-09-27 00:58:15Z squash到远程main 6e94610；同ID全勾任务与merged PR已核验accepted。本期本地范围Definition of Done按实际产品测试与用户签署更新为完成，冻结/上线属性未改。[完整本地版本包交付](https://github.com/Hubujiu/WeaveOS/actions/runs/36284110511)第2次尝试全部成功，完整人工门禁通过，[本地版本包](https://github.com/Hubujiu/WeaveOS/actions/runs/36284110511/artifacts/10920517529)已实际下载并导入校验。v0.1.0本地版已完成验收与交付；首轮回滚源码引用缺失已恢复并保留原失败记录。没有生产部署。风险答复集中在<mention-page url="https://app.notion.com/p/3e62f5a9e648814497c8df6bf27c8724"/> Q14。
## 10. 变更记录
<table fit-page-width="true" header-row="true">
<tr>
<td>日期</td>
<td>变更内容</td>
<td>原因</td>
<td>状态</td>
</tr>
<tr>
<td>2026-09-23</td>
<td>创建首版认证 PRD</td>
<td>形成认证基础版本</td>
<td>已被后续范围调整覆盖</td>
</tr>
<tr>
<td>2026-09-23</td>
<td>收缩为用户登录态，不包含授权</td>
<td>权限能力后置</td>
<td>已被下一条认证范围更新覆盖</td>
</tr>
<tr>
<td>2026-09-24</td>
<td>完成认证需求基线：Bootstrap Admin、一次性邀请码、1 小时滑动 Session、注册后重新登录、固定重置密码 `Abc@123456`、最终 Figma 原型</td>
<td>补齐 v0.1.0 开发前剩余产品与会话决策</td>
<td>当前基线</td>
</tr>
</table>
## 数据库设计与数据字典（v0.1.0）
本版本的数据库设计已建立为独立文档，状态为**设计待评审**；此处只关联设计产物，不改变本 PRD 的范围、验收勾选或冻结状态。
**版本设计**：<mention-page url="https://app.notion.com/p/3e52f5a9e64881e2b759d67ef982dd01"/>。
**ER 图**：<mention-page url="https://app.notion.com/p/3e52f5a9e648815ebf88ccc3aa9a5bac"/>。
**DDL / 事务**：<mention-page url="https://app.notion.com/p/3e52f5a9e6488155bfcef8a7cfde4550"/>；**Redis 会话规范**：<mention-page url="https://app.notion.com/p/3e52f5a9e6488184b345d41309068308"/>。
**长期字典入口**：<mention-page url="https://app.notion.com/p/3e52f5a9e64881cb8c39c16e2f3c4c53"/>，含对象级和字段级两个关联数据库。
本轮热库物理模型为 4 张 PostgreSQL 表 + 1 类 Redis Session；Q9另明确独立冷归档库。用户于2026-09-25/26已批准本期数据、DDL与Redis实施范围，并确认账号保留大小写且大小写敏感唯一、去首尾普通空格、重置失效全部旧会话，以及日志保留1年/月度冷归档/年度自动删除。页面原有审批和冻结属性保持原值；这些明确答复不代表批准其他候选技术或生产发布。
## 已授权服务器成品运行（2026-09-27，Q15）
用户明确授权将当前项目部署运行到43.133.34.48，并确认访问方式为仅SSH隧道；业务入口仅绑定服务器127.0.0.1，保留私有HTTPS与Secure Cookie，不开放公网业务端口。服务器仅运行main 6e94610已验收成品和必要迁移/初始化，不承担构建、测试或漏洞扫描。现有产品认证规则、合成数据开发范围及Q14残余风险记录保持，不据此接受公网生产风险或更改审批/冻结/上线状态。初始化仅准备Bootstrap Admin，不导入验收fixture及真实用户数据；账号凭据、服务器密钥与备份受限保存。运行健康检查与配置核验属于部署确认，不等同产品测试。部署前检查现有服务/目录/磁盘，保留已有数据。来源：用户本轮部署指令及Q15选项1答复；实际部署和恢复保障结果另行记录。
### Q16 服务器HTTPS证书方向（2026-09-27）
用户确认Let’s Encrypt + ACME自动续期，[域名weave.hubujiu.site](http://域名weave.hubujiu.site)，并允许使用DNS API。沿用Q15仅SSH隧道访问，采用DNS-01，不开放公网业务；保持Secure/HttpOnly/SameSite Cookie与同源校验，PUBLIC_ORIGIN在证书可用后同步为该域名的HTTPS入口。用户后续明确允许主账号、提供CSV并授权长期保存复用；服务器root:0600私有文件已配置。2026-09-27实际完成Let’s Encrypt DNSPod DNS-01签发、可信链校验与Nginx安装，域名HTTPS Origin及双日自动续期检查已启用，首次检查成功。默认信任下页面/健康200与认证201/200/204通过；独立Chrome域名窗口经SSH回环正常打开，不忽略TLS校验。旧[localhost](http://localhost)地址与新域名证书不匹配，使用域名入口；本机域名解析限定该窗口，未改系统hosts/根信任或公开A记录。运行细节及期限见ADR-004与Q16；最终PR验收另核对。没有产品功能、数据或Figma交互变更，不改变审批/冻结/上线状态。
## 公网运行范围更新（2026-09-28）
依据已接受的 <mention-page url="https://app.notion.com/p/3e52f5a9e64881acafe1fe79376a9c91"/> 第15节及用户本轮部署授权，当前单用户、单机阶段允许公网业务入口。用户已配置域名解析及云防火墙；本次实施 [https://weave.hubujiu.site](https://weave.hubujiu.site) 标准443入口，HTTP80仅跳转HTTPS，保持同源API、Secure/HttpOnly/SameSite Cookie与CSRF校验。数据库、Redis、BFF及管理服务不开放公网。
本节覆盖此前本地/仅SSH的运行范围限制；认证产品规则不变。已记录安全债务及同机备份局限按ADR-004当前范围接受，不形成多用户、容量或恢复SLA。原服务器成品、数据、凭据与ACME续期保留；部署和公网验证尚在执行，未宣称完成。不改变PRD冻结/上线属性。
### 公网运行验证完成（2026-09-28）
[https://weave.hubujiu.site/login](https://weave.hubujiu.site/login) 与 /register 已可普通浏览器访问，无需SSH；HTTP自动跳转HTTPS。默认信任证书、页面/资源、登录/身份恢复/退出、旧会话失效和CSRF跨源拒绝实际验证通过，现有认证/UI产品行为未改。具体运行、回滚和证据见 <mention-page url="https://app.notion.com/p/3e52f5a9e64881acafe1fe79376a9c91"/> 最新实施记录及 [PR16](https://github.com/Hubujiu/WeaveOS/pull/16)。配置PR最终CI/合并仍另核对，不把公网可访问等同版本冻结/上线属性变更。
### main 自动交付目标（2026-09-28）
用户要求main更新后自动同步既有服务器，并明确包含已验收的兼容迁移和配置更新。执行采用 <mention-page url="https://app.notion.com/p/3e52f5a9e64881acafe1fe79376a9c91"/> 最新自动交付范围：先完整检查/构建/验收，再发布同一份制品；不即时同步未验证源码。迁移前备份，失败保留或恢复旧应用/配置，破坏性和未声明兼容变化停止发布。当前单用户产品与安全范围不变；自动链路尚待实现验证。
### main自动交付已合入并触发（2026-09-28）
[PR17](https://github.com/Hubujiu/WeaveOS/pull/17) 最终head的5项检查全部成功后，已于2026-09-28T04:25:50Z按匹配head条件squash到main `88ce2da3091ad8e29c9ce65fa7d54863c281dd2f`。远程main同ID任务与merged PR核对accepted=true；远程/本地任务分支、工作树及可丢弃测试构建产物已清理，永久证据与受限密钥/服务器回滚备份资料保留，详见[清理记录](https://github.com/Hubujiu/WeaveOS/pull/17#issuecomment-5863386428)。
[首次自动发布](https://github.com/Hubujiu/WeaveOS/actions/runs/36377672016) 已实际由main push触发，新main的基础检查和核心产品验收已通过，正在验证实际镜像；尚未提前宣称服务器已切换成功。最终结果须同时核对工作流和服务器current.json。此次仅合并016，不改其他开放任务及审批/冻结/上线属性。
### Q18 已完成：main自动部署首次成功（2026-09-28）
[首次自动发布流程](https://github.com/Hubujiu/WeaveOS/actions/runs/36377672016) 由main push触发，CI、完整产品/运行/恢复/回滚/安全验收、兼容迁移测试、同制品包装和部署全部SUCCESS。服务器于2026-09-28T04:51:26.727Z返回deployed，运行提交为main `88ce2da3091ad8e29c9ce65fa7d54863c281dd2f`，发布序号 `36377672016001`。
随后独立读取服务器current.json及BFF、Nginx、审计维护三个容器，实际镜像ID与发布记录一致，三个源提交均等于远程main。公网默认TLS健康/页面200、HTTP跳转308、真实登录201、身份恢复200、退出204及旧Session401均通过；Cookie保持Secure/HttpOnly/SameSite=Lax/host-only，API no-store，monitor无当前告警。凭据与Cookie未进入仓库、日志或本页。服务器仅执行部署/运行确认，没有构建、产品测试运行器或扫描。
以后推送/合入main，检查与验收通过后自动更新同一站点，并执行显式声明兼容且已验收的迁移和应用配置；先备份，失败保留/恢复旧应用配置，不自动Down或恢复旧Session。秘密、存储引擎/拓扑、接收器本身仍需单独实施。完整边界与恢复操作见[运行手册](https://github.com/Hubujiu/WeaveOS/blob/main/infra/server/deploy/README.md)。
[PR17](https://github.com/Hubujiu/WeaveOS/pull/17) 已合并验收；远程/本地任务分支、工作树及本任务临时产物已清理，永久TDD证据、受限部署密钥和服务器回滚/备份保留。已确认范围、实施和首次自动发布均完成，无待答复问题；不更改PRD冻结/上线属性或其他开放PR。
## 登录注册视觉更新（2026-09-30，V010-019）
用户本轮要求“补齐，同时登录页其实也做了调整。全部补齐”，并在Q24确认“先按照figma原版复刻，组件之后再优化”。本轮登录/注册对齐当前WaveOS Figma节点13:2、40:2；授权项目内页面控件实现原版，组件优化后置。登录成功进入Home，登录态导航与设置人员管理按已批准的 <mention-page url="https://app.notion.com/p/3e92f5a9e64881f4be86c10bc38fff61"/> 接入。Q12禁用非目标控件、注册后返回登录、账号密码与Session规则继续适用；本节不修改审批/冻结/上线属性。集中答复：<mention-page url="https://app.notion.com/p/3ea2f5a9e64881f3a36be62911683316"/> Q24。
</content>
</page>

Here is the result of "fetch" for the Page with URL https://app.notion.com/p/3e52f5a9e64881c2b972d71ab1fc2e93 as of 2026-09-26T03:46:14.357Z:
<page url="https://app.notion.com/p/3e52f5a9e64881c2b972d71ab1fc2e93" icon="🔐">
<ancestor-path>
<parent-data-source url="collection://235f72c0-b039-40d8-9dcc-7abff28b9406" name="架构决策 ADR"/>
<ancestor-2-database url="https://app.notion.com/p/f4b3520a9eb84c8280f33f35142ca388" title="架构决策 ADR"/>
<ancestor-3-page url="https://app.notion.com/p/3e52f5a9e64881b9b005dc78a252c08f" title="Architecture & ADR"/>
<ancestor-4-page url="https://app.notion.com/p/3e42f5a9e64880ae9cf5ebbd2088d773" title="项目 - 企业管理系统"/>
</ancestor-path>
<properties>
{"ADR 标题":"ADR-001 登录态方案","ADR 编号":"ADR-001","date:决策日期:is_datetime":0,"date:决策日期:start":"2026-09-23","url":"https://app.notion.com/p/3e52f5a9e64881c2b972d71ab1fc2e93","关联功能":["https://app.notion.com/p/3e52f5a9e64880a3aaa1caa116d73996","https://app.notion.com/p/3e52f5a9e64881428779fb73a2c8fc51"],"关联迭代":["https://app.notion.com/p/3e52f5a9e64880cd951fea95bdde9cdd"],"决策领域":"认证与会话","影响范围":"系统级","最后更新时间":"2026-09-26T03:46:14.357Z","状态":"已接受"}
</properties>
<iconMetadata>{"type":"emoji","emoji":"🔐"}</iconMetadata>
<content>
<callout icon="🔐" color="green_bg">
	**已接受：V1 采用 Go BFF + 本地账号密码认证 + Server-side Session。**
	V1 不引入 ZITADEL / Keycloak 等外部 IdP，也不引入 OIDC Client SDK。OIDC 仅作为未来 SSO、第三方登录等能力的演进边界。
</callout>
## 1. 决策摘要
- **决策问题**：V1 如何实现 Web 注册、登录与登录态，同时避免认证能力与 Spring / Java 绑定。
- **最终决策**：采用 **Go BFF + 本地认证 + Redis Session + HttpOnly Secure Cookie + 统一 IdentityContext**。
- **状态**：已接受。
- **关联版本**：v0.1.0 完成用户登录态。
- **范围边界**：本 ADR 只决定认证与会话技术方案，不定义 API 路径、DTO、错误码，也不讨论权限系统。
## 2. 产品约束
来自当前 PRD 的认证范围：
- 注册必须提供 **账号 + 密码 + 有效邀请码**。
- 注册 URL 可以携带邀请码并由前端自动填入；最终有效性必须由服务端校验。
- 没有邀请码或邀请码无效时不得注册。
- 已有用户使用 **账号 + 密码**登录。
- V1 仅支持 Web。
- 普通用户没有自助忘记密码；只能由管理员重置密码。
- 管理员重置后的密码固定为 `Abc@123456`。
- V0.1.0 初始化一个 Bootstrap Admin，权限视为 `ALL`，用于测试、生成邀请码与执行管理员密码重置；不因此建设通用权限系统。
- Session 采用 1 小时空闲 TTL，并实现滑动续期。
- V1 不做登录限流、失败次数限制、自动锁定。
- V1 不做多端设备管理。
- 安全审计只保留最小登录日志：IP、设备 / User-Agent、时间、结果等。
- V1 不包含角色、权限、RBAC / ABAC / ACL。
## 3. 决策
### 3.1 Web 认证边界
Web 认证统一由独立的 **Go BFF** 承担。
前端和业务服务不得自行实现密码校验。业务服务只消费统一身份上下文，不依赖 Spring Security、Go 特定认证对象或未来某个 IdP 的私有模型。
### 3.2 V1 认证方式
V1 使用 **Local Authenticator**：
- 负责账号注册。
- 负责邀请码校验。
- 负责密码校验。
- 负责账号状态检查。
- 负责管理员密码重置。
密码只在认证边界内处理，业务模块不得读取或验证密码。
邀请码生成由 BFF 内部独立 Invitation Service / Domain 负责；V0.1.0 仅允许 Bootstrap Admin 调用该能力，暂不抽象完整员工权限判断。
### 3.3 密码存储
- 数据库只保存适合密码存储的不可逆哈希，不保存明文密码。
- 密码哈希算法与参数属于实现规范，可以独立升级，不作为业务数据模型的一部分。
- 密码仅允许标准 ASCII 可打印字符 U+0020–U+007E；至少包含 A–Z、a–z、0–9、可见 ASCII 标点各 1 个。特殊符号范围为 U+0021–U+002F、U+003A–U+0040、U+005B–U+0060、U+007B–U+007E；普通空格可保留但不计作特殊符号，非 ASCII 与控制字符拒绝。密码不去首尾空格、不做 Unicode 规范化，不额外设置长度下限。依据：2026-09-26 用户 Q13 答复“都只允许标准 ASCII 可打印符号”。
- 管理员将密码重置为 `Abc@123456` 时，数据库仍只保存其哈希。
- 明文密码不得进入普通日志、Session 或 IdentityContext。
### 3.4 Web Session
- 浏览器登录态采用 **Server-side Session**。
- Session 状态存储在 Redis。
- 浏览器仅持有不可读业务信息的随机 Session ID。
- Session ID 通过 `HttpOnly + Secure + SameSite` Cookie 传递。
- 不在 localStorage / sessionStorage 保存认证 Token 或 Session ID。
- Session 空闲 TTL 固定为 **1 小时**。
- 采用**滑动续期**：每次成功的已认证请求 / 活动将 Redis Session 过期时间重新设置为当前时间后 1 小时，并同步浏览器 Cookie 的有效期。
- 连续 1 小时没有成功的已认证活动，Session 自动过期。
- V0.1.0 不设置额外的绝对最长 Session 生命周期。
- 主动退出、Session 过期、账号禁用时，服务端必须能够使已有 Session 失效。
- 无法确认 Session 有效性时按未登录处理。
### 3.5 内部身份边界
认证成功后统一形成内部 **IdentityContext**。
V1 最小语义：
- `subjectId`
- `sessionId`
业务服务只依赖 IdentityContext，不依赖：
- 密码凭据；
- Redis Session 结构；
- Spring Security `UserDetails`；
- JWT Claim；
- ZITADEL / Keycloak / authentik 等产品模型。
### 3.6 未来 OIDC 演进
V1 **不引入 OIDC Client SDK，也不部署外部 IdP**。
代码结构必须预留独立认证提供方边界，例如：
```plain text
Authenticator
├── LocalAuthenticator        # V1
└── OIDCAuthenticator         # Future
```
未来出现以下明确需求时，再引入 OIDC / IdP：
- 企业 SSO；
- Google / GitHub / Microsoft 等第三方登录；
- MFA / Passkey；
- SAML / LDAP / AD 等身份集成。
引入 OIDC 后，不应改变：
- Browser ↔ BFF 的 Session 契约；
- 业务服务的 IdentityContext；
- 业务模块认证代码。
### 3.7 Bootstrap Admin
- 系统初始化时通过 Seed / 初始化配置创建 Bootstrap Admin。
- Bootstrap Admin 是 V0.1.0 的系统初始化特例，权限语义直接视为 `ALL`。
- Bootstrap Admin 可生成一次性邀请码，并执行管理员密码重置。
- 本特例不得演化为散落在业务代码中的硬编码权限判断；正式权限系统上线后应由后续 ADR 替换该机制。
### 3.8 注册完成后的会话行为
- 注册成功仅创建用户并消费邀请码。
- **注册成功后不自动建立 Session。**
- 浏览器返回登录界面，用户重新输入账号与密码后才建立 Session。
## 4. 明确取舍
<table fit-page-width="true" header-row="true">
<tr>
<td>方案 / 能力</td>
<td>当前决定</td>
<td>原因</td>
</tr>
<tr>
<td>Go BFF</td>
<td>采用</td>
<td>形成独立 Web 认证边界，避免认证体系绑定 Spring / Java</td>
</tr>
<tr>
<td>本地账号密码认证</td>
<td>V1 采用</td>
<td>当前需求只有邀请码注册、账号密码登录和管理员重置</td>
</tr>
<tr>
<td>Server-side Session + Redis</td>
<td>采用</td>
<td>登录态撤销明确，并为后续 BFF 多实例保留能力</td>
</tr>
<tr>
<td>HttpOnly Cookie</td>
<td>采用</td>
<td>避免认证凭据直接暴露给前端 JavaScript</td>
</tr>
<tr>
<td>ZITADEL / Keycloak / authentik</td>
<td>V1 不引入</td>
<td>当前能力需求不足以抵消完整 IAM 的部署与认知成本</td>
</tr>
<tr>
<td>OIDC Client SDK</td>
<td>V1 不引入</td>
<td>当前没有外部 OIDC Provider，无实际运行职责</td>
</tr>
<tr>
<td>纯 JWT Web 登录态</td>
<td>不采用</td>
<td>退出、禁用和即时撤销需要额外状态机制</td>
</tr>
<tr>
<td>登录限流 / 失败锁定</td>
<td>V1 不实现</td>
<td>产品范围明确排除</td>
</tr>
<tr>
<td>Bootstrap Admin</td>
<td>V0.1.0 采用</td>
<td>在尚未建设权限系统时提供测试、邀请码生成和管理员重置入口；权限直接视为 `ALL`</td>
</tr>
<tr>
<td>Session 生命周期</td>
<td>1 小时空闲 TTL + 滑动续期</td>
<td>满足 Web 会话恢复，同时让长期无活动的 Session 自动失效</td>
</tr>
</table>
## 5. 已知安全债务
- 管理员重置密码固定为 `Abc@123456`。虽然满足当前大小写字母、数字、特殊符号的字符类型规则，但固定且已知的默认密码仍弱于一次性随机密码或强制改密流程；V1 按产品要求实现，作为明确安全债务保留。
- V1 不做登录限流、失败次数限制或自动锁定，暴力尝试防护能力有限；后续安全版本可单独补充。
- 当前审计仅记录最小登录日志，不建设完整安全审计中心。
## 6. 验证原则
- [ ] 有效邀请码才能完成注册。
- [ ] URL 自动填入邀请码不能绕过服务端校验。
- [ ] 正确账号密码可以建立 Session。
- [ ] 错误账号密码不能建立 Session。
- [ ] 密码只以安全哈希形式落库。
- [ ] 管理员重置后可使用 `Abc@123456` 登录，但数据库和日志中不存在该明文密码。
- [ ] 密码策略拒绝缺少大写、小写、数字或特殊符号任一类型的密码。
- [ ] Bootstrap Admin 权限语义为 `ALL`，可生成邀请码与执行管理员重置。
- [ ] 注册成功后不会自动创建 Session，必须回到登录页重新认证。
- [ ] Session 空闲 1 小时后过期；成功认证活动会把有效期滑动至当前时间后 1 小时。
- [ ] 页面刷新后有效 Session 可恢复当前用户。
- [ ] 退出后原 Session 不可继续使用。
- [ ] HttpOnly Cookie 无法被前端 JavaScript 读取。
- [ ] 日志至少记录登录 IP、设备 / User-Agent、时间与结果，且不包含密码或完整 Session ID。
- [ ] 业务服务只依赖 IdentityContext，不直接处理密码或认证实现。
- [ ] 未来新增 OIDCAuthenticator 时无需修改业务模块身份模型。
## 7. 关联文档
- **PRD**：<mention-page url="https://app.notion.com/p/3e52f5a9e64880cd951fea95bdde9cdd"/>
- **API 契约 / OpenAPI**：系统级规范见 <mention-page url="https://app.notion.com/p/3e52f5a9e648813da842e2ef15a09b3a"/>。具体 endpoint、DTO 与错误码在仓库 contracts 中单独维护；本 ADR 仍只决定认证与会话方案。
- **未来 ADR**：出现企业 SSO / 第三方登录需求后，再创建 OIDC / Identity Provider 选型 ADR。
## 8. 变更记录
<table fit-page-width="true" header-row="true">
<tr>
<td>日期</td>
<td>变更</td>
<td>原因</td>
</tr>
<tr>
<td>2026-09-23</td>
<td>创建 ADR-001</td>
<td>确定 Web 登录态架构</td>
</tr>
<tr>
<td>2026-09-23</td>
<td>曾拟采用 Go BFF + OIDC Provider</td>
<td>为未来跨技术栈与外部身份认证预留标准边界</td>
</tr>
<tr>
<td>2026-09-24</td>
<td>收敛为 Go BFF 本地认证 + Session；OIDC 后置，并补齐 Bootstrap Admin、1 小时滑动续期与固定密码规则</td>
<td>完成 v0.1.0 认证与会话技术基线</td>
</tr>
</table>
</content>
</page>

Here is the result of "fetch" for the Page with URL https://app.notion.com/p/3e52f5a9e6488155bfcef8a7cfde4550 as of 2026-09-29T11:58:52.447Z:
<page url="https://app.notion.com/p/3e52f5a9e6488155bfcef8a7cfde4550" icon="🛠️">
<ancestor-path>
<parent-page url="https://app.notion.com/p/3e52f5a9e64881cb8c39c16e2f3c4c53" title="Database & Data Dictionary"/>
<ancestor-2-page url="https://app.notion.com/p/3e42f5a9e64880ae9cf5ebbd2088d773" title="项目 - 企业管理系统"/>
</ancestor-path>
<properties>
{"title":"v0.1.0 PostgreSQL DDL 与事务规范"}
</properties>
<iconMetadata>{"type":"emoji","emoji":"🛠️"}</iconMetadata>
<content>
<callout icon="🛠️" color="yellow_bg">
	**PostgreSQL 18 目标设计稿，不是已执行迁移。**
	4 张表、31 列；与 v0.1.0 ER 和字段字典配套。以下建表语句无业务秘密；事务片段使用参数绑定。DDL、并发与权限仍需在项目锁定的数据库版本上集成验证。
</callout>
<table_of_contents/>
## 1. 使用约定
基线：<mention-page url="https://app.notion.com/p/3e52f5a9e64881e2b759d67ef982dd01"/>。图：<mention-page url="https://app.notion.com/p/3e52f5a9e648815ebf88ccc3aa9a5bac"/>。
建议将 SQL 作为首个 Goose migration 的 Up 内容；迁移序号应按仓库现状分配，不假定仓库当前为空。本段保留初始设计含义；用户已批准本期DDL实施，003迁移已合入，008运行权限/冷库/恢复见[具体证据](https://github.com/Hubujiu/WeaveOS/blob/6eaf36335c048620a8e348b6fb577eb613d8a04f/docs/evidence/V010-008/review.md)。未连接或部署生产数据库；不得重复执行已发布建表迁移或修改其内容。建表由独立迁移身份执行，运行时身份不拥有 schema/table。
连接时区设 UTC；账号比较必须大小写敏感；数据库 locale / collation 在环境初始化时固定并验证唯一约束与登录检索行为。schema 名称为 `auth`，应用 SQL 使用全限定表名。
## 2. 建表 DDL
```sql
CREATE SCHEMA auth;

CREATE TABLE auth.users (
    id uuid CONSTRAINT pk_users PRIMARY KEY
        DEFAULT gen_random_uuid(),
    account varchar(254) NOT NULL,
    account_key text GENERATED ALWAYS AS (account) STORED NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'active',
    is_bootstrap_admin boolean NOT NULL DEFAULT false,
    auth_version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_users_account_key UNIQUE (account_key),
    CONSTRAINT ck_users_account CHECK (
        char_length(account) BETWEEN 1 AND 254
        AND account = btrim(account)
        AND position(' ' in account) = 0
        AND account !~ '[[:cntrl:]]'
    ),
    CONSTRAINT ck_users_status CHECK (status IN ('active', 'disabled')),
    CONSTRAINT ck_users_auth_version CHECK (auth_version > 0)
);

CREATE UNIQUE INDEX uq_users_single_bootstrap
    ON auth.users (is_bootstrap_admin)
    WHERE is_bootstrap_admin;

CREATE TABLE auth.password_credentials (
    user_id uuid CONSTRAINT pk_password_credentials PRIMARY KEY,
    password_hash text NOT NULL,
    password_changed_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_password_credentials_user FOREIGN KEY (user_id)
        REFERENCES auth.users (id) ON DELETE RESTRICT,
    CONSTRAINT ck_password_credentials_hash CHECK (
        char_length(password_hash) BETWEEN 1 AND 1024
    )
);

CREATE TABLE auth.invitations (
    id uuid CONSTRAINT pk_invitations PRIMARY KEY
        DEFAULT gen_random_uuid(),
    code_hash bytea NOT NULL,
    created_by uuid NOT NULL,
    used_by uuid,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_invitations_code_hash UNIQUE (code_hash),
    CONSTRAINT uq_invitations_used_by UNIQUE (used_by),
    CONSTRAINT fk_invitations_creator FOREIGN KEY (created_by)
        REFERENCES auth.users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_invitations_consumer FOREIGN KEY (used_by)
        REFERENCES auth.users (id) ON DELETE RESTRICT,
    CONSTRAINT ck_invitations_hash CHECK (octet_length(code_hash) = 32),
    CONSTRAINT ck_invitations_usage_pair CHECK (
        (used_by IS NULL AND used_at IS NULL)
        OR (used_by IS NOT NULL AND used_at IS NOT NULL)
    ),
    CONSTRAINT ck_invitations_usage_time CHECK (
        used_at IS NULL OR used_at >= created_at
    )
);

CREATE INDEX ix_invitations_created_by
    ON auth.invitations (created_by);

CREATE TABLE auth.authentication_events (
    id uuid CONSTRAINT pk_authentication_events PRIMARY KEY
        DEFAULT gen_random_uuid(),
    event_type varchar(32) NOT NULL,
    outcome varchar(16) NOT NULL,
    actor_user_id uuid,
    subject_user_id uuid,
    account_fingerprint varchar(81),
    client_ip inet,
    user_agent varchar(2048),
    session_ref uuid,
    reason_code varchar(63),
    request_id varchar(128) NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT fk_authentication_events_actor FOREIGN KEY (actor_user_id)
        REFERENCES auth.users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_authentication_events_subject FOREIGN KEY (subject_user_id)
        REFERENCES auth.users (id) ON DELETE RESTRICT,
    CONSTRAINT ck_authentication_events_type CHECK (
        event_type IN (
            'register', 'login', 'logout', 'invitation_created',
            'password_reset', 'session_invalid',
            'account_status_changed', 'bootstrap_created'
        )
    ),
    CONSTRAINT ck_authentication_events_outcome CHECK (
        outcome IN ('success', 'failure', 'error')
    ),
    CONSTRAINT ck_authentication_events_fingerprint CHECK (
        account_fingerprint IS NULL
        OR account_fingerprint ~ '^[A-Za-z0-9_-]{1,16}:[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_authentication_events_request_id CHECK (
        char_length(request_id) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_authentication_events_reason CHECK (
        reason_code IS NULL
        OR reason_code ~ '^[A-Z][A-Z0-9_]{0,62}$'
    )
);

CREATE INDEX ix_authentication_events_time
    ON auth.authentication_events (occurred_at DESC, id DESC);
CREATE INDEX ix_authentication_events_subject_time
    ON auth.authentication_events (subject_user_id, occurred_at DESC)
    WHERE subject_user_id IS NOT NULL;
CREATE INDEX ix_authentication_events_actor_time
    ON auth.authentication_events (actor_user_id, occurred_at DESC)
    WHERE actor_user_id IS NOT NULL;
CREATE INDEX ix_authentication_events_request
    ON auth.authentication_events (request_id);

COMMENT ON TABLE auth.users IS 'v0.1.0 用户认证主档，不包含通用授权模型';
COMMENT ON COLUMN auth.users.account_key IS '大小写敏感的账号唯一登录检索键；Alice 与 alice 不同';
COMMENT ON COLUMN auth.users.auth_version IS '凭据重置或状态变化时递增；Session 保存快照';
COMMENT ON TABLE auth.password_credentials IS '本地密码自描述哈希；不得存明文或可逆密码';
COMMENT ON COLUMN auth.password_credentials.password_hash IS '应用密码库验证格式；长度约束不是原始密码长度策略';
COMMENT ON TABLE auth.invitations IS '无时间过期的一次性邀请码，仅存高熵原文的摘要';
COMMENT ON COLUMN auth.invitations.used_by IS 'NULL=未使用；与 used_at 同时赋值；正常应用不得清空复用';
COMMENT ON TABLE auth.authentication_events IS '只追加的最小认证审计，不是审计中心';
COMMENT ON COLUMN auth.authentication_events.session_ref IS '独立非认证 UUID，不是 Cookie SID，不是 Redis 外键';
```
`CREATE SCHEMA` 不使用 IF NOT EXISTS 掩盖未知旧结构；首次迁移前检查该命名空间是否已被使用。重复执行应由迁移工具阻止，不靠自动建表“修复”漂移。
哈希列的 CHECK 只限制存储形状，不证明内容是安全密码哈希；算法、参数、盐和格式验证由认证库负责。真实密码长度策略依 PRD，不从哈希字符串长度反推。
## 3. 约束与索引清单
<table fit-page-width="true" header-row="true">
<tr>
<td>约束 / 索引</td>
<td>支撑的查询或不变量</td>
<td>不保证的事项</td>
</tr>
<tr>
<td>users PK；account_key UNIQUE</td>
<td>按用户 ID 获取；去首尾普通空格后按大小写敏感账号登录；并发重复账号拒绝</td>
<td>不提供邮箱验证、两个登录别名或完整 Unicode case folding</td>
</tr>
<tr>
<td>uq_users_single_bootstrap</td>
<td>最多一条 is_bootstrap_admin=true</td>
<td>不会自动创建管理员，也不等于完整 RBAC</td>
</tr>
<tr>
<td>credentials.user_id PK + FK</td>
<td>每用户最多一份本地凭据；凭据必须有用户</td>
<td>不强制每个用户都存在凭据子行</td>
</tr>
<tr>
<td>invitation code_hash UNIQUE</td>
<td>摘要精确查找与生成碰撞保护</td>
<td>不能代替消费事务</td>
</tr>
<tr>
<td>used_by UNIQUE + usage_pair CHECK</td>
<td>每用户最多消费一个码；使用者和时间成对</td>
<td>不自动禁止有 SQL 写权限者清空使用标记；应用只允许单向消费</td>
</tr>
<tr>
<td>created_by 索引；审计用户索引</td>
<td>关联查询、外键相关检索、按用户定位事件</td>
<td>未对未知未来查询提前铺满索引</td>
</tr>
<tr>
<td>审计 time / request 索引</td>
<td>时间范围与 request_id 定位</td>
<td>request_id 非唯一，同一请求允许多条事件</td>
</tr>
</table>
PostgreSQL 会为主键和唯一约束建立支持索引，但不会自动为每个引用侧外键建索引；可空唯一字段默认允许多个 NULL。[约束依据](https://www.postgresql.org/docs/18/ddl-constraints.html)
## 4. 注册事务
先在事务外做输入校验、邀请码格式解析和密码哈希计算，避免持有邀请码锁时执行昂贵哈希。邀请码原文建议为 32 随机字节的无填充 Base64URL；服务端严格解码并验证规范编码，存储 / 查询摘要统一为 SHA-256(原始 32 字节)，不是 SHA-256(任意输入字符串)。
以下是**按顺序执行的 pgx 事务片段**；每条 SQL 的 `$1` 从该语句重新计数，并非可直接粘贴 psql 的独立脚本。每步返回值和受影响行数必须由应用检查。
```sql
BEGIN;

-- 参数：code_hash。查不到 / used_by 非 NULL 均拒绝并 ROLLBACK。
SELECT id, used_by
FROM auth.invitations
WHERE code_hash = $1
FOR UPDATE;

-- 参数：已校验且无内部普通空格的 account。使用返回 id；禁止接受客户端管理员标记。
INSERT INTO auth.users (account)
VALUES ($1)
RETURNING id;

-- 参数：新用户 id、安全哈希。
INSERT INTO auth.password_credentials (user_id, password_hash)
VALUES ($1, $2);

-- 参数：邀请码 id、新用户 id。必须返回恰好一行。
UPDATE auth.invitations
SET used_by = $2, used_at = clock_timestamp()
WHERE id = $1 AND used_by IS NULL
RETURNING id;

-- 参数：新用户 id、客户端 IP、截断后的 UA、request_id。
INSERT INTO auth.authentication_events (
    event_type, outcome, subject_user_id, client_ip, user_agent, request_id
) VALUES ('register', 'success', $1, $2, $3, $4);

COMMIT;
```
默认 READ COMMITTED 配合 `FOR UPDATE` 即可串行化同一码的消费；不使用 SKIP LOCKED 把正在使用的码误判为不存在。并发等待者在获得锁后重新检查已使用状态。任何失败，包括账号唯一冲突、凭据插入失败、审计失败和未消费到一行，都必须整体回滚。[行锁依据](https://www.postgresql.org/docs/18/explicit-locking.html)
失败事件在事务回滚后另行记录，不能把它和成功事务一起回滚掉。成功仅返回注册结果，不访问 Redis、不设置登录 Cookie。
若 COMMIT 响应丢失，结果可能已经提交。不得自动释放邀请码、删除用户或盲目再执行创建；通过账号 / 邀请使用结果核对，并引导用户尝试登录。客户端 Idempotency-Key 不参与这个最小原子性保证。
## 5. 登录与重置竞态
登录以一次 JOIN 读取 `users.id/status/auth_version` 和凭据哈希，记录版本 N；完成密码验证后复查状态与版本仍为 N，再把 **N** 写入 Session。禁止验证旧密码后读取 N+1，并用 N+1 签发会话。
重置与登录并发时，重置在一个 PG 事务内改密码并递增版本。若重置提交后迟到的登录写出了版本 N 的 Session，下一次受保护请求会因版本不一致拒绝它；不能将其升级为当前版本。
Redis Session 创建、PG 登录事件、HTTP Cookie 不存在天然原子提交。建议顺序为 Redis 创建成功 → 持久化成功事件 → 下发 Cookie；审计失败则尽力删除刚创建的 Session、拒绝登录并告警。进程崩溃留下的未交付随机 Session 由 TTL 清理，不向外暴露 SID。
## 6. 管理员密码重置事务
首先校验当前 Session；事务内将操作者和目标用户按 UUID 排序加锁，统一顺序避免相互重置时锁顺序倒置。再次确认操作者 active、Bootstrap=true 且认证版本与 Session 一致；客户端不能指定操作身份。
```sql
-- 参数：操作者与目标用户 UUID 数组。
SELECT id, status, is_bootstrap_admin, auth_version
FROM auth.users
WHERE id = ANY($1::uuid[])
ORDER BY id
FOR UPDATE;

-- 参数：目标用户 id、新盐生成的安全哈希。
UPDATE auth.password_credentials
SET password_hash = $2,
    password_changed_at = clock_timestamp(),
    updated_at = clock_timestamp()
WHERE user_id = $1;

-- 参数：目标用户 id；必须和上一步、审计写入在同一事务内。
UPDATE auth.users
SET auth_version = auth_version + 1,
    updated_at = clock_timestamp()
WHERE id = $1;
```
两个 UPDATE 都必须影响一行；追加 `password_reset/success` 事件记录 actor 与 subject 后才能 COMMIT。固定重置密码来自 PRD，数据库只接收其新盐哈希，SQL 和日志不得出现明文。auth_version 不得归零或覆盖旧值。
禁用 / 重新启用采用相同状态与版本的事务更新原则。单纯升级密码哈希参数不改变 password_changed_at / auth_version；使用旧哈希条件的比较更新，避免覆盖并发重置后的密码。
## 7. Bootstrap Seed 与数据库访问身份
Seed 使用一次性受控初始化身份；通过串行初始化或事务锁避免重复创建，条件唯一索引兜底。已存在 Bootstrap 时验证完整性并退出；若只有同名普通用户则报冲突，禁止自动提升权限。Seed 重跑不覆盖密码，不重置 auth_version，不重新启用被禁用管理员。
运行时身份必须与 schema/table owner 分开。以下为已由运维创建 `auth_app` 角色后的授权示例，不包含角色密码或连接串；不应以超级用户运行应用。
```sql
REVOKE ALL ON SCHEMA auth FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA auth FROM PUBLIC;
GRANT USAGE ON SCHEMA auth TO auth_app;
GRANT SELECT ON auth.users, auth.password_credentials, auth.invitations TO auth_app;
GRANT INSERT (id, account, status, auth_version, created_at, updated_at)
    ON auth.users TO auth_app;
GRANT UPDATE (status, auth_version, updated_at)
    ON auth.users TO auth_app;
GRANT INSERT ON auth.password_credentials TO auth_app;
GRANT UPDATE (password_hash, password_changed_at, updated_at)
    ON auth.password_credentials TO auth_app;
GRANT INSERT (id, code_hash, created_by, created_at)
    ON auth.invitations TO auth_app;
GRANT UPDATE (used_by, used_at) ON auth.invitations TO auth_app;
GRANT INSERT ON auth.authentication_events TO auth_app;
GRANT SELECT (id) ON auth.authentication_events TO auth_app;
```
普通运行身份不获写入 is_bootstrap_admin、表结构变更、用户删除或审计 UPDATE / DELETE 权限。审计读取仅限经应用确认的当前有效 Bootstrap Admin；独立受限数据库身份仅供该读取路径使用。应用仍须校验用例授权和不可逆的邀请消费；列级授权不是完整业务规则引擎。已有额外角色继承 / grant 应在部署时一并审计。
## 8. 迁移、回滚与验收
新环境按 schema → users → credentials → invitations → events → 索引 / 授权 → Seed 的顺序建立。创建后检查 pg_catalog / information_schema，确认生成列、约束、索引和应用权限与字典一致。
生产回滚优先回滚应用到兼容版本，不使用 DROP TABLE 删除用户与审计数据。首个建表迁移的破坏性 Down 仅可在明确可销毁的测试库执行；生产迁移应使用向前修复。恢复备份必须同步切换共享 Session generation，防止认证版本或已删除会话回退。
以下完整性 SQL 预期无异常行，但**本轮尚未执行**：
```sql
-- 已创建用户缺失凭据。
SELECT u.id FROM auth.users u
LEFT JOIN auth.password_credentials c ON c.user_id = u.id
WHERE c.user_id IS NULL;

-- 非 Bootstrap 用户缺失注册邀请码。
SELECT u.id FROM auth.users u
LEFT JOIN auth.invitations i ON i.used_by = u.id
WHERE NOT u.is_bootstrap_admin AND i.id IS NULL;

-- 初始化后应为 1；唯一索引本身只能保证不超过 1。
SELECT count(*) FROM auth.users WHERE is_bootstrap_admin;
```
还需测试：同邀请码 20 个并发注册至多 1 成功；两个邀请码注册同账号只创建 1 用户且失败请求不耗码；多次 Seed 不改凭据；非法外键 / 状态 / 半使用邀请码被拒；匿名日志无需伪用户；应用不能写管理员标记或篡改历史事件。
对象登记入口：<mention-page url="https://app.notion.com/p/554868452adb45b589d5f585216672c9"/>；逐列定义：<mention-page url="https://app.notion.com/p/ca3aa22691ad4ee0a7fddbc21d9b3fcb"/>。
## 2026-09-29 已确认变更：管理员已提交结果与会话续期
来源：用户明确采用 <mention-page url="https://app.notion.com/p/3e92f5a9e64881e4bda8cc98c085c889"/> 剩余全部方案，见 <mention-page url="https://app.notion.com/p/3e92f5a9e6488141b26cc627572f407e"/> D8及 <mention-page url="https://app.notion.com/p/3ea2f5a9e64881f3a36be62911683316"/> Q19。
已经通过身份认证、事务内管理员复核且PostgreSQL明确提交成功的生成邀请码或密码重置操作，随后Redis Renew失败时仍交付已提交业务结果：邀请码201及生成结果，重置200。清除浏览器两种Cookie并记录脱敏诊断，后续请求重新认证；清Cookie不等于Redis已撤销，不重建旧Session。提交前身份/权限/审计失败不提交；Commit结果不确定仍报错，不自动重试。自我重置仍递增auth_version并清Cookie，其他既有认证规则不变。
此为已确认待实现规则，尚未宣称上线；不变更表/字段/索引、Redis key/结构/TTL、历史迁移、认证审计冷热保留或Figma界面。
</content>
</page>
