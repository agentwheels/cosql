# 无事务执行 proposal

## 目标

让不支持事务的 MySQL 协议数据库也能通过 cosql 提交 SQL，仍由用户审阅 proposal 后执行。使用可选参数，不改变现有 proposal 的默认行为。

## 验收条件

- 使用 `propose --no-transaction` 创建的 proposal 保存无事务模式，不执行 SQL 或 EXPLAIN。
- 人工执行 `sudo cosql apply` 时显示该模式，直接执行 proposal 中的 SQL，不开启、提交或回滚事务。
- 未指定参数及历史 proposal 继续使用原来的事务路径。
- 无事务执行失败时说明可能已有部分变更，不能宣称已回滚；执行结果仍记录到 proposal 和审计日志。
- 非 root 用户不能执行 `apply`；`query` 和 `explain` 的只读事务保护不变。
- 自动测试使用拒绝事务的数据库替身验证执行结果，并验证默认提交、失败回滚、失败提示和 proposal 持久化。

## 范围

本次只支持无事务写入审批流程，不给 `query` 或 `explain` 增加跳过只读保护的参数。每条 DDL 单独提交，避免多语句部分成功后难以核对。真实 Lindorm 的权限、连接和 SQL 语法由目标环境验证，不以数据库替身的测试结果代替。
