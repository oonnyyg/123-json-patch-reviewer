# json-patch-reviewer

纯 Go 标准库实现的 RFC 6902 JSON Patch 审查工具：先预览逐步变化与逆操作，
确认后再落盘；任一步失败整份补丁不生效，已有输出文件不会被覆盖。不接入
任何远程服务。

## 用法

```
go build ./cmd/jsonpatch
jsonpatch -doc doc.json -patch patch.json                 # 预览：报告打印到 stdout
jsonpatch -doc doc.json -patch patch.json -out out.json   # 全部成功才写入 out.json
jsonpatch ... -report report.json                         # 报告同时写入文件
```

失败时以退出码 1 结束，标准错误给出失败操作序号与原因，输出文件保持原样。

## 审查报告

报告为 JSON，包含：

- `ok` / `error` / `failedOpIndex`：整体结果与失败操作序号。
- `steps`：每步的操作、路径、该步之后的完整文档快照及该步逆操作。
- `final`：全部成功后的最终文档。
- `inverseOps`：逐步逆操作；反序回放可精确恢复原文档（含数组顺序），
  库内 `ApplyInverses` 提供该能力，测试中有往返验证。

预览与落盘走同一条 `Apply` 路径，两种模式结果必然一致。

## 支持范围

- 操作：`add` / `remove` / `replace` / `move` / `copy` / `test`，按数组顺序
  依次作用于前一步结果；未知操作或缺少 `path`/`from`/`value` 时报错并给出
  操作序号。
- 路径：RFC 6901 JSON Pointer，支持空路径（整篇文档）与 `~0`/`~1` 转义；
  对象键按原文匹配，数组索引不允许越界、前导零或非数字；`-` 仅作为 add 的
  追加位置；缺失的父节点不会自动创建。
- 语义：数组 add 为插入并后移元素，remove 后索引立即变化；move 先移除来源
  再定位目标，禁止移入自身后代；copy 深拷贝子树，不与来源共享可变状态；
  test 按结构比较（对象键序无关、数组顺序相关、数值按数学值比较、布尔不
  等同于数值）；允许替换根值，拒绝删除根值。
- 原子性：补丁在内存中逐步应用，任何一步失败即整体拒绝，不写任何文件。

## 测试

```
go test ./...
```

覆盖转义键、数组插入/移动/重索引、中途 test 失败的原子性、根值替换与
删除限制、copy 独立性、test 语义及逆操作往返恢复。
