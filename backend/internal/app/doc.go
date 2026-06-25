// Package app 负责组装 HTTP 路由树、健康检查和模块 handler。
//
// 这个包只做应用层组合：把 store、cache、配置、中间件和各业务模块接到 Gin
// engine 上；具体鉴权、缓存策略、请求协商、响应 envelope 和业务规则分别放在
// middleware、httputil 与 modules 中，避免路由注册层承载过多行为。
package app
