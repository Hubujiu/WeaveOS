# Seed 邀请码字节语义

来源：Notion auth.invitations / code_hash 字段明确 SHA-256(解码后的32字节)，Base64URL仅展示编码。5b212ca测试提交在真实PostgreSQL18运行 go test ./cmd/acceptance-seed -run TestAcceptanceSeedProduces -count=1，退出1，四类邀请码的独立解码哈希匹配行数均0、预期1，有效RED。永久测试seed-red-test.go/旧实现seed-before.go。

最小修复两处先解码再哈希，完整seed测试退出0。中间一次返回值数量编译错误不计业务RED。无真实凭据/邀请码写入证据。
