# V030-049 原始执行证据

所有路径均属于独立V049工作树，基线67eb444d3e698dac53ede3cb344909384c91cc13。Root亲写测试/debug/验收；执行者仅按合同实现配置与runner。没有旧测试修改、推送或CI执行。

- initial-test-run：原传输HTML转义导致SyntaxError，非有效RED，原始字节保留。
- workflow-red：Root授权html.unescape并核对原件SHA后，工作流4项测试实际11目标断言失败，图反例PASS；完整基线源码与YAML保留。
- action-cache：官方actions/cache v6.1.0/tag SHA与PyPI PyYAML6.0.3实查记录；本机未安装依赖。
- cache-edit-attempt：机械编辑器KeyError发生在写配置前，保留原错误与仍为基线RED的复测；只修编辑器，不计新行为RED。
- cache-stage：缓存两项PASS，gate缺失仍为2 failures、exit1，阶段真实结果。
- preflight-red：唯一无行为占位与Root13项测试原件；13FAIL、exit1，为有效目标RED。
- first-green：首次工作流4PASS/preflight13PASS，后续追加Python独立venv gate前的源码与配置。
- final-green：最终Python4PASS/Node13PASS，0skip、两个exit0；运行前后源码哈希、最终全部实现与Root测试原件；结构比较证明原CI/acceptance完整定义剥离授权新增后与基线一致。

Root原件SHA256：Python bfa864b21ef70fff8830436fe909c6f3b15a5441b550b9bb309fbd960968fcde；Node b8216c279038acaf487340d51de953bd5b31c16c6b92e36cae49a04b09c734f0。

这些早期GREEN仅证明Root配置/注入执行行为合同，不代表真实spawn子命令、候选Gitleaks扫描、完整共享preflight或CI执行。

组件阶段新增：components-red保存Root10项组件模块与8项文件加载无行为占位的全部有效FAIL、18项原件校验PASS以及Python新增第5项调度有效RED；component-discovery保存固定Node24.14.0/pnpm10.28.2实际locked安装与未分片--list JSON（502身份/36文件/errors空），只发现、不执行组件；components-green保存Root13+18+10+8共49项Node及5项Python全部PASS、0skip的原始日志、14路径运行前后SHA与源码。Root五份新/更新原件SHA见任务文档，均匹配。

完整四片、真实候选CLI产物、product全栈及适用CI未执行，V048最终通过候选尚未整合；以上定向GREEN不能代替这些检查。实现与所有证据均未提交、未推送；后续由Root亲审全文diff并冻结Actions候选。

容器依赖阶段：dependency-red保存Root公共依赖helper11项占位有效FAIL及初版Python第6项product缓存声明缺失的有效RED；dependency-helper-green保存精确Root实现/测试与11PASS；dependency-node-green保存累计60项Node定向PASS、0skip及源路径哈希，不重复复制整套workflows。修正版Python仅追加Root明确要求的go.mod/go.sum输入，等待Root最终SHA核验；product三路径声明尚未实施，不能把部分接线说成完成。未运行真实冷/热/失效或CI，旧security扫描文件不动。

Root最终确认Python修正版SHA后，dependency-product-red记录该版本6项实际1目标FAIL；随后product三公共路径/key声明实施，dependency-final-green记录Python6/Node60全部PASS。regression记录固定Node24.14.0的既有治理/底座/契约379PASS、0skip与结构/任务/diff全部exit0。完整组件发现baseline.json以baseline.json.gz无损归档，baseline-archive.json记录原始/压缩SHA，原件留工作树。上述无真实冷/热/失效或CI结果。

本地授权检查点71bba1128d77afa2068714fd6424cdb105ca5206包含显式选入的批准源码和已审最小证据；checkpoint-evidence-selection.json保留该提交前逐文件SHA。secret-only记录原固定Gitleaks在该确切HEAD的1PASS/0skip/exit0。此结果和更新的任务摘要未再次提交，保持已扫描HEAD不变；无推送/PR/CI。附加cached whitespace检查原RED两日志16处空白为exit2，源码/任务0，原日志字节保留。

python-runtime-cache记录Root第7项原件SHA匹配、实际RED（6PASS/1目标FAIL）及指定两处YAML补正后的Python7/Node60/旧快检379PASS与结构检查exit0；仅最小日志与原件，不复制整套workflow。此前71b确切检查点已按Root单次授权推送；当前新增cache补正尚未推送，新检查点不等于已secret-scan或完整CI通过。
