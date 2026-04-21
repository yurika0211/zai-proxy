/**
 * 首页处理
 */
package handler

import "github.com/valyala/fasthttp"

/**
 * handleIndex 返回静态首页
 */
func (h *ProxyHandler) handleIndex(ctx *fasthttp.RequestCtx) {
	if len(h.indexHTML) == 0 {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		return
	}
	ctx.SetContentType("text/html; charset=utf-8")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBody(h.indexHTML)
}
