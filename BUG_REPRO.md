# Bug reproduction

基线分支：`green_base_bug_003`

调度器停止函数重复关闭同一个 channel，正常信号退出时会在 shutdown 阶段 panic。

复现：启动服务并发送 SIGTERM，观察退出堆栈是否出现 `close of closed channel`。

验证：`go test ./internal/worker -run '^TestBug003StopIsIdempotent$' -count=20`
