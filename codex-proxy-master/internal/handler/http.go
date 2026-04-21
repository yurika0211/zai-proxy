package handler

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/valyala/fasthttp"
)

func writeJSON(ctx *fasthttp.RequestCtx, status int, payload interface{}) {
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(status)
	b, err := json.Marshal(payload)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.SetBodyString(`{"error":{"message":"json编码失败","type":"server_error"}}`)
		return
	}
	ctx.SetBody(b)
}

func writeText(ctx *fasthttp.RequestCtx, status int, text string) {
	ctx.SetContentType("text/plain; charset=utf-8")
	ctx.SetStatusCode(status)
	ctx.SetBodyString(text)
}

func isWebSocketUpgradeRequest(ctx *fasthttp.RequestCtx) bool {
	if !bytesEqualFold(ctx.Request.Header.Peek("Upgrade"), []byte("websocket")) {
		return false
	}
	connection := string(ctx.Request.Header.Peek("Connection"))
	return strings.Contains(strings.ToLower(connection), "upgrade")
}

func bytesEqualFold(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if toLowerASCII(a[i]) != toLowerASCII(b[i]) {
			return false
		}
	}
	return true
}

func toLowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// fasthttp compatible http.ResponseWriter for executor stream APIs
type fastHTTPResponseWriter struct {
	ctx         *fasthttp.RequestCtx
	bufWriter   *bufio.Writer
	header      http.Header
	wroteHeader bool
}

func newFastHTTPResponseWriter(ctx *fasthttp.RequestCtx, w *bufio.Writer) *fastHTTPResponseWriter {
	return &fastHTTPResponseWriter{
		ctx:       ctx,
		bufWriter: w,
		header:    make(http.Header),
	}
}

func (w *fastHTTPResponseWriter) Header() http.Header {
	return w.header
}

func (w *fastHTTPResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	for k, vs := range w.header {
		for _, v := range vs {
			w.ctx.Response.Header.Add(k, v)
		}
	}
	w.ctx.SetStatusCode(statusCode)
	w.wroteHeader = true
}

func (w *fastHTTPResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	/* 不在每次 Write 时 Flush，降低 syscall 与首包后延迟；由调用方 Flusher.Flush 控制 SSE 块刷新 */
	return w.bufWriter.Write(p)
}

func (w *fastHTTPResponseWriter) Flush() {
	_ = w.bufWriter.Flush()
}

var _ http.Flusher = (*fastHTTPResponseWriter)(nil)
var _ http.ResponseWriter = (*fastHTTPResponseWriter)(nil)

/* streamBufWriter 仅写 bufio，不触碰 RequestCtx；用于 SetBodyStreamWriter 内写 SSE 体 */
type streamBufWriter struct {
	w *bufio.Writer
}

func newStreamBufWriter(w *bufio.Writer) *streamBufWriter {
	return &streamBufWriter{w: w}
}

func (s *streamBufWriter) Write(p []byte) (int, error) {
	return s.w.Write(p)
}

/*
 * writeOpenAIChatCompletionSSEError 在已发送 200 + text/event-stream 后写入一条 data 错误 JSON，可选再写 [DONE]。
 * 避免 Pump 失败时响应体完全为空，调试器与客户端能看到原因。
 */
func writeOpenAIChatCompletionSSEError(w *bufio.Writer, message, errType string, withDone bool) {
	sw := newStreamBufWriter(w)
	payload, mErr := json.Marshal(map[string]any{
		"error": map[string]any{"message": message, "type": errType},
	})
	if mErr != nil {
		payload = []byte(`{"error":{"message":"编码错误","type":"server_error"}}`)
	}
	_, _ = io.WriteString(sw, "data: ")
	_, _ = sw.Write(payload)
	_, _ = io.WriteString(sw, "\n\n")
	_ = w.Flush()
	if withDone {
		_, _ = io.WriteString(sw, "data: [DONE]\n\n")
		_ = w.Flush()
	}
}
