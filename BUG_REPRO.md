# Bug reproduction

基线分支：`green_base_bug_005`

内存分页在极大页码下先计算乘积，整数溢出后形成非法切片下标并可能 panic。

复现：向分页函数传入 `page=math.MaxInt`、`page_size=200`，观察切片访问是否越界。

验证：`go test ./internal/store -run '^TestBug005MemoryPaginationDoesNotPanicOnOverflow$' -count=20`
