# Root 原样测试首次运行

命令：`python3 tests/governance/root_ci_throughput_test.py`

工作目录：`/workspace/WeaveOS-worktrees/V030-049`

基线：`67eb444d3e698dac53ede3cb344909384c91cc13`

Python3.12.14、PyYAML6.0.3，未安装依赖。源文件原样保存，包括传入第76行的`&lt;=`。运行exit1，stdout为空，stderr为SyntaxError；未执行任何断言，不是有效行为RED。测试/debug归Root；未自主改测试或实施配置。

同目录root_ci_throughput_test.py为运行时原始源码，SHA256 df01c2bc7854c198834bd1f532c2119534823e89a8b29539b62f556a2f3b9c40。command.json保存命令与环境元数据；stdout.txt/stderr.txt保持原始字节；exit-code.txt保存实际退出码。
