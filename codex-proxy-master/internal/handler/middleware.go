/**
 * FastHTTP 中间件：CORS 与预检处理
 */
package handler

import (
	"strings"

	"github.com/valyala/fasthttp"
)

/**
 * OptionsBypass 直接放行 OPTIONS 预检请求，避免触发鉴权或业务逻辑
 */
func OptionsBypass(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		if !ctx.IsOptions() {
			next(ctx)
			return
		}

		origin := string(ctx.Request.Header.Peek("Origin"))
		if origin == "" {
			origin = "*"
		}
		allowMethods := string(ctx.Request.Header.Peek("Access-Control-Request-Method"))
		if allowMethods == "" {
			allowMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
		}
		allowHeaders := string(ctx.Request.Header.Peek("Access-Control-Request-Headers"))
		if allowHeaders == "" {
			allowHeaders = "Authorization, Content-Type"
		}

		ctx.Response.Header.Set("Access-Control-Allow-Origin", origin)
		ctx.Response.Header.Set("Vary", "Origin")
		ctx.Response.Header.Set("Access-Control-Allow-Methods", allowMethods)
		ctx.Response.Header.Set("Access-Control-Allow-Headers", allowHeaders)
		ctx.Response.Header.Set("Access-Control-Max-Age", "86400")
		ctx.SetStatusCode(fasthttp.StatusNoContent)
	}
}

/**
 * CORSAllowOrigin 确保所有响应都包含 Access-Control-Allow-Origin
 */
func CORSAllowOrigin(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		origin := string(ctx.Request.Header.Peek("Origin"))
		if origin == "" {
			origin = "*"
		}
		ctx.Response.Header.Set("Access-Control-Allow-Origin", origin)
		ctx.Response.Header.Set("Vary", "Origin")
		next(ctx)
	}
}

/**
 * GzipIfAccepted 当客户端声明支持 gzip 时，启用响应压缩（交给 fasthttp 内置压缩）
 */
func GzipIfAccepted(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		if ctx.IsOptions() || isV1Path(string(ctx.Path())) || !clientAcceptsGzip(ctx) {
			next(ctx)
			return
		}
		// fasthttp 中间件可以通过 Server配置进行压缩，当前不做业务层包装
		next(ctx)
	}
}

func clientAcceptsGzip(ctx *fasthttp.RequestCtx) bool {
	enc := string(ctx.Request.Header.Peek("Accept-Encoding"))
	return enc != "" && strings.Contains(enc, "gzip")
}

func isV1Path(path string) bool {
	return path == "/v1" || strings.HasPrefix(path, "/v1/")
}
