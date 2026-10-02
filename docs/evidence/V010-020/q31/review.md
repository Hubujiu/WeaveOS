# Q31 用户审核修改的独立来源与 RED

用户2026-09-30明确移除顶栏收缩、截图中页签多余滚动条并缩短侧栏标题/菜单间距；要求同步Figma作为视觉与交互唯一事实源。继续任务020开放PR21和唯一工作树，主Agent唯一写者，实际子Agent只读成因定位，无并发写入。remote main6ffba4d、旧head5541466五项CI实际SUCCESS；新阶段不能引用旧绿灯。

先读取项目/PRD/v010历史/Architecture/ADR003、人员R3相关正文和登录功能，以及组件库consumer/contract/批准登记；Q24项目原版页面控件授权继续适用。Figma先同步：Navigation / Web只剩Mode=Auth/App，固定56px；Admin Shell只剩Side176/72和Interaction普通/悬停，旧Top属性、40px状态和收缩原型控件移除。108:247/279实例改为固定顶栏，113:425链接保留为Side=Collapsed/Sidebar Hover。人员导航槽位移除原64px留白，实际四变体菜单y116、展开x16/144×48、收起x12/48×48；人员图标x26/y130。页签总高48含边框，按钮47，横滚可达，无纵向溢出。R3§5.3–5.5与登录功能同步并重读，审批/冻结/上线属性不改，旧截图保留为明确历史。R3更新时间2026-09-30T15:41:57.634Z，登录15:42:44.826Z；Figma工具未提供更新时间。高保真75:2/108:151/113:425及截图实际重读，几何通过Plugin API核对。

Windows/Node22.23.1/pnpm10.28.2/Playwright1.63 Chromium，测试独立4174，4173用户预览保留。实现前运行`pnpm exec playwright test --config .work/q31.config.ts --grep 'Q31 Home|Q31 admin uses|Q31 tabs|Q31 real member'`：退出1、3失败1通过、23.9s。目标断言实际观察顶栏收缩控件仍1个，菜单旧y180≠116；页签overflowY=auto≠hidden、scrollHeight48≠clientHeight47、按钮48≠47、scrollTop=1≠0。真实成员纵/横滚动保护用例通过；不是语法/依赖故障。red.txt为实际日志；red/归档新测试和全部旧实现/配置，规范化UTF8/LF SHA256不可变。

旧四态测试按正式Q31改为固定56×两侧态；保留编辑、材质覆盖/resize、20px原图标/48热区/36中轴、逐帧稳定y、快速反向/减少动效、业务权限/错误/保存与Session断言。新测试独立验证无控件、y116、页签真实溢出和末页签键盘可达；20条合成成员验证必要滚动未被隐藏。先提交推送本RED，再做最小实现。

最小实现后目标4/4 GREEN（11.6s）、完整95/95 Chromium（3.5m）、类型/构建、132治理/基础和结构门禁均通过；此前真实RED已实际推送，时序可恢复。侧栏两态padding88→24、顶栏state/控件/40px移除、页签适配47内高；所有原素材文件未变。当前4173浏览器实测几何及静态SVG加载成功见local-preview-geometry.json，图标20px与箭头/保存/退出16px槽位对照源节点；1920及2504截图保留。只读子Agent审查无问题，不将审查称执行。三引擎和最终head CI待独立核对。

最终三引擎63/63通过（3.7m），cross-browser.txt为完整输出；95/95组件、类型/构建、132治理、结构检查均实际通过，Windows日志中的pnpm个人配置/颜色警告未影响退出码。8805fbe HEAD archive同固定Gitleaks8.30.1镜像退出0。1920/2504三引擎两侧态截图已保存；人工对照原材质/边缘/图标与源几何，scoped控件/菜单/页签目标一致。当前4173审核窗口三标签保留；synthetic fixture不冒充真实后端。最终head CI实时核对，未合并/验收/部署。
