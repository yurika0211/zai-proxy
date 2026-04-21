package netutil

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

/* DefaultHTTPReadWriteBuf 与 executor/handler 原 32KiB 对齐，利于吞吐与 syscall 次数平衡 */
const DefaultHTTPReadWriteBuf = 32 * 1024

/* MaxConnsPerHostHTTP2Cap HTTP/2 单主机连接过多易触发上游 GOAWAY ENHANCE_YOUR_CALM */
const MaxConnsPerHostHTTP2Cap = 30

// UpstreamTransportConfig 构建指向 Codex / 上游 API 的共享 *http.Transport。
// ResponseHeaderTimeout、TLSHandshakeTimeout、IdleConnTimeout、ExpectContinueTimeout 为 0 时 net/http 表示不限制。
type UpstreamTransportConfig struct {
	DialContext           func(context.Context, string, string) (net.Conn, error)
	ProxyURL              string
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	MaxConnsPerHost       int
	EnableHTTP2           bool
	ResponseHeaderTimeout time.Duration
	TLSHandshakeTimeout   time.Duration
	IdleConnTimeout       time.Duration
	ExpectContinueTimeout time.Duration
	/* WriteBufferSize / ReadBufferSize 为 0 时使用 net/http 默认值 */
	WriteBufferSize    int
	ReadBufferSize     int
	DisableCompression bool
}

// NewUpstreamTransport 创建配置一致的出站 Transport（连接池、HTTP/2、h2 连接数上限等）。
func NewUpstreamTransport(cfg UpstreamTransportConfig) *http.Transport {
	maxConns := cfg.MaxConnsPerHost
	if cfg.EnableHTTP2 && maxConns > MaxConnsPerHostHTTP2Cap {
		maxConns = MaxConnsPerHostHTTP2Cap
	}

	tlsConf := &tls.Config{InsecureSkipVerify: false}
	transport := &http.Transport{
		DialContext:           cfg.DialContext,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:       maxConns,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
		ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
		ExpectContinueTimeout: cfg.ExpectContinueTimeout,
		ForceAttemptHTTP2:     cfg.EnableHTTP2,
		DisableCompression:    cfg.DisableCompression,
		TLSClientConfig:       tlsConf,
	}
	if cfg.WriteBufferSize > 0 {
		transport.WriteBufferSize = cfg.WriteBufferSize
	}
	if cfg.ReadBufferSize > 0 {
		transport.ReadBufferSize = cfg.ReadBufferSize
	}
	if !cfg.EnableHTTP2 {
		transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
		transport.TLSClientConfig.NextProtos = []string{"http/1.1"}
	}

	if p := strings.TrimSpace(cfg.ProxyURL); p != "" {
		if u, err := url.Parse(p); err == nil {
			if sch := strings.ToLower(u.Scheme); sch == "http" || sch == "https" {
				transport.Proxy = http.ProxyURL(u)
			}
		}
	}

	return transport
}
