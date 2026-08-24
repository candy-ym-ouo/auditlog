# Bug reproduction

基线分支：`green_base_bug_008`

归档服务把每次归档的内部 deadline 固定为两秒，无法支持耗时超过该预算的正常归档操作。

复现：让 Store 检查 ArchiveService 传入 context 的剩余 deadline；修复前剩余时间约为两秒，不足以完成较大的归档任务。

验证：`go test ./internal/service -count=20 -run '^TestBug008ArchiveAllowsLongRunningStoreOperation$'`
