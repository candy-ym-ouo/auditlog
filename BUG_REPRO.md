# Bug reproduction

基线分支：`green_base_bug_006`

不存在的审计记录经过 TraceService 的错误包装后丢失可识别的 ErrNotFound，API 层无法稳定返回 404。

复现：请求不存在的 trace entry id，观察接口是否错误返回 500 或无法通过 `errors.Is` 分类。

验证：`go test ./internal/service -run '^TestBug006MissingEntryPreservesNotFound$' -count=20`
