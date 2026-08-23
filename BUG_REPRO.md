# Bug reproduction

基线分支：`green_base_bug_002`

并发追加审计记录时，服务层缓存的序号和前置哈希缺少同步保护，可能生成重复序号或断裂链。

复现：并发提交多组相同格式的追加请求，检查返回记录的 `seq` 和 `prev_hash` 是否保持唯一、连续。

验证：`go test -race ./internal/service -run '^TestBug002ConcurrentAppendKeepsUniqueSequences$' -count=20`
