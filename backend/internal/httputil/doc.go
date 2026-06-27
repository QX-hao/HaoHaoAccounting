// Package httputil 统一封装 HTTP 响应格式、错误码和常用响应头。
//
// handler 应优先通过这里返回结构化 JSON 错误、分页 Link、Location 等跨模块契约，
// 避免各模块手写不同的错误 envelope 或把 request id、状态码语义写散。
package httputil
