# V010-007 完整真实栈验收

2026-09-26，本机WSL Docker Desktop Linux29.6.2，隔离项目weaveos-v010-007-1790398519196，`node infra/acceptance/run.mjs`实际exit0。公开结果local-result.json；随机fixture/HMAC/密码/TLS私钥/数据库内容保留在私有工作目录，未入仓库或artifact。前七次失败的原因和真实修复顺序详见red-green.md，不把其中部分绿灯充作整栈通过。

## 实际结果

| 验证 | 结果 |
| --- | --- |
| PostgreSQL18空库Goose3.28迁移、重复Up | 通过；初始迁移无破坏性Down，恢复路径归008 |
| Go race -p1 ./...、vet、静态BFF构建 | 全部exit0，含新增真实黑洞调用期限与所有PG/Redis生命周期/事务/并发测试 |
| Node契约/治理/基础/拓扑 | 76/76 exit0（14契约+57基础治理+5配置） |
| OpenAPI3.2.1 Redocly2.54.2 lint | exit0；一项原有license元数据警告，未自行选择许可证 |
| pnpm10.28.2冻结依赖、typecheck、Vite正式build | exit0 |
| Chromium独立组件 | 23/23，36.6s；mock仅用于组件，不能代替下两层 |
| Nginx同源HTTPS真实HTTP与响应Schema | 25/25，1.07s；代码/HTTP登记、violations、Cookie、401头、人类message、HEAD/204无正文、requestId/no-store |
| Chromium/Firefox/WebKit真实产品浏览器 | 30/30，1.9m，零自动retry、原30秒期限与严格Lax/Cookie断言；无API mock |
| 实际存储容器停止/重启 | 3/3；未知API JSON404/伪造身份拒绝，Redis/PG故障readiness与已有Session/新登录503、不发Cookie，重启后ready200 |

Redis故障用例总27.32s含Docker停止/启动和恢复等待，不是单次HTTP响应耗时；所有请求仍有10sAbortSignal并实际收到约定状态。PG故障与恢复4.72s。上次Redis忽略期限的真实缺陷已先RED后修复，未抬高客户端期限。

## 来源与映射

PRD全部FR-001..018映射见docs/acceptance/README.md。当前Notion PRD/ADR001/数据/凭据/身份功能Q13正文最后编辑03:46Z、ADR00201:55Z；需求编写/未冻结及ADR003/004拟议状态未更改。用户的Q5/Q6仅批准本地WSL Docker模拟，Q10由用户最终签署。Figma唯一当前视觉源WaveOS Login13:2/Register40:2；Q12禁用与Q13注释已同步并实际读回。

Go产品代码自4280845后未改；本地运行期间配置/测试文字更新至a1f71ce，不能将该本机工作树运行冒充单个不可变Git head的CI。POSIX fixture owner/chown分支只能在Linux宿主验证，Windows映射不充当该权限证明。最终PR提交必须另查对应GitHubCI/product的全部success，才可squash。

当前验证链接：[PR10](https://github.com/Hubujiu/WeaveOS/pull/10)、[产品run36219435809](https://github.com/Hubujiu/WeaveOS/actions/runs/36219435809)、[CI36219435796](https://github.com/Hubujiu/WeaveOS/actions/runs/36219435796)。这些链接在本文件形成时仍运行中，不预填success；文档/矩阵形成的最终head还须重新检查。

## 发布门禁与交接

七个自动项依据真实本机完整执行登记passed，并保留上述CI二次检查要求。MAN-UI、MAN-RISK、MAN-OPERATIONS均pending，用户Q10是负责人、不是已签署。`node scripts/check-release.mjs`仍应exit1。007交008的材料：006desktop/mobile截图、全部自动映射与永久RED源码/哈希；008须完成受限审计读取/年度按月冷归档和自动删除、权限、加密备份与隔离恢复、generation防旧会话、制品回滚/来源SHA，再在Notion问题页提交具体用户审核材料。未实施这些前不能宣布v0.1.0完成或公网可运行。
