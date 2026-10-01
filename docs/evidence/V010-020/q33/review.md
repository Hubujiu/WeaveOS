# V010-020 · Q33 固定侧栏

用户2026-10-01明确要求移除侧栏收起，并删除Figma多余页面。继续PR21/现有独立工作树，主Agent唯一代码写者；一名实际子Agent只读审查代码，一名仅同步项目Figma并在独立目录归档。没有合并/部署。

Notion当前R3§5.3–5.5和登录功能已按确认范围同步并重读；Figma固定顶栏56px/侧栏176px，菜单144×48/x16/y116、人员图标20×20/x26/y130。移除箭头后Default/Hover视觉属性相同，不新增悬停增强；120ms仅保留原型进入/离开状态语义。正式来源以final-source-readback.json和Figma最终说明为准，source-and-red.json是实施前真实历史快照。审批/冻结/上线状态及外部组件源码保持。

Figma删除9个根：3张旧布局重复后台画板、Sidebar Toggle、2个Compact变体、独立Compact导航与2个箭头实例；共1380子树节点。主后台/身份/模板/记录四业务视图、登录/注册/Home、Q32规则和UI Kit保留。删除前完整限定属性、实例覆盖及原型已归档；最终读回9根均不存在、无Side/Compact、无指向删除根的原型连线。before JSON及manifest说明可恢复方式和新建ID限制；不是只引用临时工具返回。

产品变更仅PersonnelAdmin的side state/切换按钮/隐藏ARIA、侧栏CSS宽度/折叠动画/断点，以及已无引用的admin-chevron.svg。材质resize、后端请求/权限/编辑保护和Q32控件未改。既有窄屏顶部strong隐藏为预存差异，本轮不扩展到顶部响应式重设计；侧栏文字全视口常显。

真实TDD顺序：

1. 在未修改产品实现的31b1111上新增8项独立来源断言并实际8/8 RED：桌面按钮count1，900宽240、390/320宽72而需176，文字opacity0。源码/测试/配置/日志/LF哈希保存在red/，提交907864a先推送。
2. 根据已确认Q33来源，将9组旧折叠专用用例改为固定几何/resize/hover，保留全部业务、材质、滚动与输入断言；54组原用例数量不变。19项目标实际14失败5通过，red/adapted-*完整归档，61e4a9d先推送。
3. 才实施最小GREEN。目标19/19通过；完整组件与三引擎均通过。没有删除正确断言、skip/only或用实现计算expected。

|实际命令|退出/结果|证据|
|---|---|---|
|pnpm exec playwright test --config .work/q32-full.config.ts --grep Q33（初始）|1；8失败|red/run.txt|
|同上（旧用例按新来源适配）|1；14失败/5通过|red/adapted-run.txt|
|同上（最小实现后）|0；19/19，35.8s|target-green.txt|
|pnpm exec playwright test --config .work/q32-full.config.ts|0；114/114，3.9m|component-full.txt|
|pnpm exec playwright test --config .work/q32-cross-final.config.ts --grep 'Q33\|Q31\|Q32\|Figma responsive'|0；Chromium/Firefox/WebKit117/117，5.1m|cross-browser.txt|
|pnpm typecheck / pnpm build|分别0|typecheck.txt / build.txt|
|node --test tests/foundation/*.test.mjs tests/governance/*.test.mjs|0；132/132，无skip|governance.txt|
|node scripts/verify-repo.mjs / node scripts/check-tasks.mjs|分别0|verify-repo.txt / check-tasks.txt|
|git diff --check|0|主Agent实际执行|

4174完整/4175三引擎与4173用户预览隔离。9张三引擎1920/2504材质截图、当前2560×1351原生审核窗口和Figma截图人工检查固定外壳/原素材槽位/无箭头；7个可见素材已加载。当前预览和组件API均为合成fixture，没有冒充真实BFF/PG/Redis。真实产品/安全/同制品验收由最终head远端CI另验证。

只读代码Agent复审最小diff及54组测试：未发现回归/遗漏，未写文件或运行测试。仓库按.gitattributes统一LF，永久文件实际存储字节SHA256见manifest.json；red/sha256.json保持实施前快照，新增箭头归档另有supplemental hash。Figma Agent原始转存文件的逐字节版本保留figma-original-byte-archive.zip，其内部before与原manifest的4106d2a4…哈希一致；外部JSON规范LF仅改变换行，不改属性、连线或来源内容。Figma manifest内SHA描述ZIP中的原始字节，仓库LF文本以总manifest为准。日志末尾纯空白规范TDD:N/A，未改执行结果。

RED head61e4a9d沿用了历史ready且Q33清单未完成，远端check-tasks明确失败。当前清单真实完成后恢复ready并实跑结构检查通过；没有弱化门禁或改写历史。

当前交付ready仅表示可审阅。下一条命令：提交最小GREEN/证据，对实际HEAD git archive执行固定Gitleaks扫描；推送后核对PR21最终head五项CI。开放工作树、在用4173和必要恢复现场保留，尚未合并/验收/部署。
