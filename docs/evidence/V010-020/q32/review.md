# Q32 控件交付核对

六处单选均使用PersonnelSelect，四页签使用PersonnelTabs。样式/动画来源为用户指定Arca固定版本和已同步R3§5.6/Figma规则；项目内原创React/CSS/WAAPI实现，无上游代码复制、UI依赖或后端改动。账号动作菜单、时间范围表单保留原用途。

| 已确认要求 | 独立实际用例 |
| --- | --- |
| 38px/r12触发器、32px选项、同宽面板与8px分离；身份ID/清空查询 | identity select |
| 分组操作/来源/目标三个选择框、Escape先关菜单、保留模态焦点 | grouped selectors |
| 父部门ID/创建参数、原生模态不关闭 | parent chooser |
| 操作类型最终项/实际action参数、有限视口长列表/外部点击 | long action menu |
| 40px轨道/32px黑色Pill/48px外位、连续位移、脏编辑确认前不移动 | pill indicator |
| 无位移/模糊/交错、末页签键盘可达、Tab退出 | reduced motion |
| 100项目录End立即可读；独立合成UUID目录 | large directories |
| 真实鼠标关闭/重开前确处于展开中间高度；下一帧差<16px | quick reversal strengthened |
| 窄屏搜索不改变正文scrollTop | searching below tabs |
| 向上开关全过程靠触发器，间距[-1,12]；有中间高度 | upward expansion |
| 向上面板下侧相邻角变化、上角保持12 | upward corners |

真实顺序：c7bcb7d六项RED → 95ef219首批GREEN/四边界RED → a3c52b3边界GREEN → 08762ef向上贴边RED → 96fd08e向上GREEN。目录red/、interaction-red/、anchor-red/保存完整可恢复源码、测试、配置、实际日志和LF哈希，不依赖分支保留。起初向上角只有一个选项未翻转的场景不算有效RED；其原日志保留。Chromium旧DOM.click长采样发生采样窗与动画启动错位，不据此宣称浏览器缺陷；可信点击与相同几何断言重放旧实现仍RED179px。快速反向强化后归档旧实现仍RED41.75px，当前三引擎3/3 GREEN。

最终产品源码的完整106/106 Chromium与96/96三引擎回归通过；之后仅快速反向测试强化，三引擎3/3再次通过，类型/构建/132治理与结构检查通过。测试API边界使用合成数据，只证明前端交互和请求参数；真实BFF/PG/Redis产品验收由最终head product CI验证，须实时核对，不冒充本机全栈。

保留12张三引擎两侧态截图，代表截图对照源几何；固定56顶栏、176/72侧栏、菜单y116/图标y130、原始素材与正文滚动保持。截图只改输出路径以隔离并行测试，不改断言，TDD:N/A。最终head CI待推送后核对，ready不等于accepted；PR21开放，用户4173合成前端审核窗口继续运行，未合并/部署。
