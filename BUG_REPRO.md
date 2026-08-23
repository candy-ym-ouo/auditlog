# Bug reproduction

基线分支：`green_base_bug_009`

TraceService 复用跨请求可变 map，连续或并发请求会互相污染 ChainPosition，并产生数据竞争。

复现：并发请求两个不同 trace entry，在响应 map 中分别写入请求标识，观察另一个响应是否出现该标识或触发 race detector。

验证：`go test -race ./internal/service -run '^TestBug009TraceContextOwnsChainPosition$' -count=20`
