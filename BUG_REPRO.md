# Bug reproduction

基线分支：`green_base_bug_004`

归档完成阶段向尚未初始化的状态 Metadata map 写入完成时间，首次归档会触发 nil map panic。

复现：准备满足阈值的审计记录并执行归档，观察完成状态更新处的运行时崩溃。

验证：`go test ./internal/service -run '^TestBug004ArchiveStatusMetadataIsWritable$' -count=20`
