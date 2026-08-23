# Bug reproduction

基线分支：`green_base_bug_008`

归档服务使用固定短超时，且数据库 rows 与事务释放范围不合理，慢归档会被错误中止并留下异常状态。

复现：使用较大的活动记录集触发归档，观察约两秒后是否出现 deadline exceeded 或资源未及时释放。

验证：`go test ./internal/service -run '^TestBug008ArchiveHonorsCallerCancellation$' -count=20`
