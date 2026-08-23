# Bug reproduction

基线分支：`green_base_bug_007`

verify 请求被取消后，验证链仍可能继续访问存储并写入验证结果，context 取消没有贯穿下游调用。

复现：在验证开始前取消请求 context，检查是否仍扫描记录或执行 SetVerify。

验证：`go test ./internal/service -run '^TestBug007VerifyPropagatesCancellation$' -count=20`
