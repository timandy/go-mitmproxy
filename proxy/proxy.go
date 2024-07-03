package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/cert"
	"github.com/lqqyt2423/go-mitmproxy/internal/helper"
	"github.com/lqqyt2423/go-mitmproxy/log"
)

type Options struct {
	Debug             int
	Addr              string
	StreamLargeBodies int64 // 当请求或响应体大于此字节时，转为 stream 模式
	SslInsecure       bool
	CaRootPath        string
	NewCaFunc         func() (cert.CA, error) //创建 Ca 的函数
	Upstream          string
	ShutdownTimeout   time.Duration // 服务关闭超时时间
}

type Proxy struct {
	Opts      *Options
	Version   string
	Addons    []Addon
	errorChan chan error
	quitChan  chan os.Signal

	entry           *entry
	attacker        *attacker
	shouldIntercept func(req *http.Request) bool              // req is received by proxy.server
	upstreamProxy   func(req *http.Request) (*url.URL, error) // req is received by proxy.server, not client request
}

// proxy.server req context key
var proxyReqCtxKey = new(struct{})

func NewProxy(opts *Options) (*Proxy, error) {
	if opts.StreamLargeBodies <= 0 {
		opts.StreamLargeBodies = 1024 * 1024 * 5 // default: 5mb
	}

	proxy := &Proxy{
		Opts:      opts,
		Version:   "1.8.5",
		Addons:    make([]Addon, 0),
		errorChan: make(chan error, 1),
		quitChan:  make(chan os.Signal, 1),
	}

	proxy.entry = newEntry(proxy)

	attacker, err := newAttacker(proxy)
	if err != nil {
		return nil, err
	}
	proxy.attacker = attacker

	return proxy, nil
}

func (proxy *Proxy) AddAddon(addon Addon) {
	proxy.Addons = append(proxy.Addons, addon)
}

func (proxy *Proxy) Start() {
	// release resources
	defer func() {
		close(proxy.errorChan)
		close(proxy.quitChan)
	}()

	// start attack
	go func() {
		if err := proxy.StartAttack(); err != nil {
			log.Errorf("Attack failed to start, %v", err)
		}
	}()

	// start proxy
	go func() {
		log.Info("Proxy is starting...")
		ln, err := proxy.Listen()
		if err != nil {
			proxy.errorChan <- err
			return
		}
		log.Infof("Proxy already listen at %v", ln.Addr().(*net.TCPAddr).Port)
		proxy.errorChan <- proxy.Serve(ln)
	}()

	// wait for quit signal
	signal.Notify(proxy.quitChan, syscall.SIGINT, syscall.SIGTERM)
	select {
	case startErr := <-proxy.errorChan:
		log.Errorf("Proxy failed to start, %v", startErr)
	case <-proxy.quitChan:
		log.Info("Proxy is shutting down...")
		var shutdownCtx context.Context
		if proxy.Opts.ShutdownTimeout > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), proxy.Opts.ShutdownTimeout)
			defer cancel()
			shutdownCtx = ctx
		} else {
			shutdownCtx = context.Background()
		}
		shutdownErr := proxy.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			_ = proxy.Close()
			log.Errorf("Proxy already forced shutdown, %v", shutdownErr)
			return
		}
		log.Info("Proxy already shutdown")
	}
}

func (proxy *Proxy) Stop() {
	proxy.quitChan <- syscall.SIGTERM
}

func (proxy *Proxy) StartAttack() error {
	return proxy.attacker.start()
}

func (proxy *Proxy) Listen() (net.Listener, error) {
	return proxy.entry.listen()
}

func (proxy *Proxy) Serve(ln net.Listener) error {
	return proxy.entry.serve(ln)
}

func (proxy *Proxy) Close() error {
	return proxy.entry.close()
}

func (proxy *Proxy) Shutdown(ctx context.Context) error {
	return proxy.entry.shutdown(ctx)
}

func (proxy *Proxy) GetCertificate() x509.Certificate {
	return *proxy.attacker.ca.GetRootCA()
}

func (proxy *Proxy) GetCertificateByCN(commonName string) (*tls.Certificate, error) {
	return proxy.attacker.ca.GetCert(commonName)
}

func (proxy *Proxy) SetShouldInterceptRule(rule func(req *http.Request) bool) {
	proxy.shouldIntercept = rule
}

func (proxy *Proxy) SetUpstreamProxy(fn func(req *http.Request) (*url.URL, error)) {
	proxy.upstreamProxy = fn
}

func (proxy *Proxy) realUpstreamProxy() func(*http.Request) (*url.URL, error) {
	return func(cReq *http.Request) (*url.URL, error) {
		req := cReq.Context().Value(proxyReqCtxKey).(*http.Request)
		return proxy.getUpstreamProxyUrl(req)
	}
}

func (proxy *Proxy) getUpstreamProxyUrl(req *http.Request) (*url.URL, error) {
	if proxy.upstreamProxy != nil {
		return proxy.upstreamProxy(req)
	}
	if len(proxy.Opts.Upstream) > 0 {
		return url.Parse(proxy.Opts.Upstream)
	}
	cReq := &http.Request{URL: &url.URL{Scheme: "https", Host: req.Host}}
	return http.ProxyFromEnvironment(cReq)
}

func (proxy *Proxy) getUpstreamConn(ctx context.Context, req *http.Request) (net.Conn, error) {
	proxyUrl, err := proxy.getUpstreamProxyUrl(req)
	if err != nil {
		return nil, err
	}
	var conn net.Conn
	address := helper.CanonicalAddr(req.URL)
	if proxyUrl != nil {
		conn, err = helper.GetProxyConn(ctx, proxyUrl, address, proxy.Opts.SslInsecure)
	} else {
		conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", address)
	}
	return conn, err
}
