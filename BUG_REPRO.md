# Bug reproduction

基线分支：`green_base_bug_001`

归档请求返回成功后，后台任务继承已经结束的 HTTP request context，归档不会执行。

复现：启动服务后执行 `curl -X POST http://127.0.0.1:8080/api/v1/archive`，随后查询 `/api/v1/archive/status`，观察任务未完成且没有新增归档批次。

验证：`go test ./internal/api -run '^TestBug001ArchiveRequestContextDoesNotCancelWork$' -count=20`
