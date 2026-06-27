# Backend Cache

这个包封装后端共享的 Redis 缓存适配器和缓存 key 生成规则。

- `RedisCache` 负责连接、健康检查、JSON 编解码、字符串写入和按前缀失效。
- 业务模块只应该依赖这里暴露的项目语义，不要在 handler 里直接拼 Redis 命令。
- 缓存未配置、启动失败或关闭后，读写方法会降级为 no-op，让本地开发和核心业务路径不强依赖 Redis。
- 前缀删除使用 `SCAN` 分批遍历，并拒绝空前缀，避免误删整个 Redis DB。

当前 key 归属：

- `report:summary:*` 用于报表聚合缓存，交易变更后按用户前缀失效。
- `ai:parse:*` 用于自然语言记账解析结果缓存，按用户和原始文本隔离。
- `auth:revoked:*` 用于 JWT 登出撤销列表，只保存 token 摘要，不保存明文 token。
