// Package middleware 提供 API 请求进入 handler 前后的横切能力。
//
// 这里集中处理 request id、超时、panic 恢复、安全响应头、CORS 前后的缓存策略、
// 请求体大小、Content-Type、Accept 协商、Prometheus 指标和 Bearer 鉴权。新增
// 中间件时应同时补充 README 与对应测试，确保全局行为不会只停留在隐式约定里。
package middleware
