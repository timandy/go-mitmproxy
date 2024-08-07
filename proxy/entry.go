package proxy

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/lqqyt2423/go-mitmproxy/internal/helper"
	"github.com/lqqyt2423/go-mitmproxy/log"
)

// wrap tcpListener for remote client
type wrapListener struct {
	net.Listener
	proxy *Proxy
}

func (l *wrapListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	proxy := l.proxy
	wc := newWrapClientConn(c, proxy)
	connCtx := newConnContext(wc, proxy)
	wc.connCtx = connCtx

	for _, addon := range proxy.Addons {
		addon.ClientConnected(connCtx.ClientConn)
	}

	return wc, nil
}

// wrap tcpConn for remote client
type wrapClientConn struct {
	net.Conn
	r       *bufio.Reader
	proxy   *Proxy
	connCtx *ConnContext

	closeMu   sync.Mutex
	closed    bool
	closeErr  error
	closeChan chan struct{}
}

func newWrapClientConn(c net.Conn, proxy *Proxy) *wrapClientConn {
	return &wrapClientConn{
		Conn:      c,
		r:         bufio.NewReader(c),
		proxy:     proxy,
		closeChan: make(chan struct{}),
	}
}

func (c *wrapClientConn) Peek(n int) ([]byte, error) {
	return c.r.Peek(n)
}

func (c *wrapClientConn) Read(data []byte) (int, error) {
	return c.r.Read(data)
}

func (c *wrapClientConn) Close() error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()

	//已关闭， 直接返回
	if c.closed {
		return c.closeErr
	}

	//记录日志
	log.Debug("in wrapClientConn close", c.connCtx.ClientConn.Conn.RemoteAddr())

	//关闭与客户端的底层连接
	c.closed = true
	c.closeErr = c.Conn.Close()
	close(c.closeChan)

	//触发事件
	for _, addon := range c.proxy.Addons {
		addon.ClientDisconnected(c.connCtx.ClientConn)
	}

	//调用 wrapServerConn.Close()
	if c.connCtx.ServerConn != nil && c.connCtx.ServerConn.Conn != nil {
		_ = c.connCtx.ServerConn.Conn.Close()
	}

	return c.closeErr
}

func (c *wrapClientConn) CloseRead() error {
	if tc, ok := c.Conn.(*net.TCPConn); ok && tc != nil {
		return tc.CloseRead()
	}
	return nil
}

// wrap tcpConn for remote server
type wrapServerConn struct {
	net.Conn
	proxy   *Proxy
	connCtx *ConnContext

	closeMu  sync.Mutex
	closed   bool
	closeErr error
}

func (c *wrapServerConn) Close() error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()

	//已关闭, 直接返回
	if c.closed {
		return c.closeErr
	}

	//记录日志
	cconn := c.connCtx.ClientConn
	if cconn != nil {
		log.Debug("in wrapServerConn close", cconn.Conn.RemoteAddr())
	}

	//关闭与远端服务器的底层连接
	c.closed = true
	c.closeErr = c.Conn.Close()

	//触发事件
	for _, addon := range c.proxy.Addons {
		addon.ServerDisconnected(c.connCtx)
	}

	//关闭客户端连接, 不能直接调用 wrapClientConn.Close(), 会造成死循环
	if cconn != nil {
		if !cconn.Tls {
			if wcc, ok := cconn.Conn.(*wrapClientConn); ok && wcc != nil {
				_ = wcc.CloseRead()
			}
		} else if !c.connCtx.closeAfterResponse {
			// if keep-alive connection close
			_ = cconn.Conn.Close()
		}
	}

	return c.closeErr
}

type entry struct {
	proxy  *Proxy
	server *http.Server
}

func newEntry(proxy *Proxy) *entry {
	e := &entry{proxy: proxy}
	e.server = &http.Server{
		Addr:    proxy.Opts.Addr,
		Handler: e,
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			return context.WithValue(ctx, connContextKey, c.(*wrapClientConn).connCtx)
		},
	}
	return e
}

func (e *entry) listen() (net.Listener, error) {
	addr := e.server.Addr
	if addr == "" {
		addr = ":http"
	}
	return net.Listen("tcp", addr)
}

func (e *entry) serve(ln net.Listener) error {
	pln := &wrapListener{
		Listener: ln,
		proxy:    e.proxy,
	}
	return e.server.Serve(pln)
}

func (e *entry) close() error {
	return e.server.Close()
}

func (e *entry) shutdown(ctx context.Context) error {
	return e.server.Shutdown(ctx)
}

func (e *entry) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	proxy := e.proxy

	// proxy via connect tunnel
	if req.Method == "CONNECT" {
		e.handleConnect(res, req)
		return
	}

	if !req.URL.IsAbs() || req.URL.Host == "" {
		res = helper.NewResponseCheck(res)
		for _, addon := range proxy.Addons {
			addon.AccessProxyServer(req, res)
		}
		if res, ok := res.(*helper.ResponseCheck); ok {
			if !res.Wrote {
				res.WriteHeader(400)
				io.WriteString(res, "此为代理服务器，不能直接发起请求")
			}
		}
		return
	}

	// direct transfer
	shouldIntercept := e.shouldHandle(req)
	if !shouldIntercept {
		transferHttp(res, req)
		return
	}
	// http proxy
	proxy.attacker.initHttpDialFn(req)
	proxy.attacker.attack(res, req)
}

func (e *entry) shouldHandle(req *http.Request) bool {
	proxy := e.proxy
	return proxy.shouldIntercept == nil || proxy.shouldIntercept(req)
}

func (e *entry) handleConnect(res http.ResponseWriter, req *http.Request) {
	shouldIntercept := e.shouldHandle(req)
	f := newFlow()
	f.Request = newRequest(req)
	f.ConnContext = req.Context().Value(connContextKey).(*ConnContext)
	f.ConnContext.Intercept = shouldIntercept
	defer f.finish()

	if !shouldIntercept {
		log.Debugf("begin transpond %v", req.Host)
		e.directTransfer(res, req, f)
		return
	}

	if f.ConnContext.ClientConn.UpstreamCert {
		e.httpsDialFirstAttack(res, req, f)
		return
	}

	log.Debugf("begin intercept %v", req.Host)
	e.httpsDialLazyAttack(res, req, f)
}

func (e *entry) establishConnection(res http.ResponseWriter, f *Flow) (net.Conn, error) {
	cconn, _, err := res.(http.Hijacker).Hijack()
	if err != nil {
		res.WriteHeader(502)
		return nil, err
	}
	_, err = io.WriteString(cconn, "HTTP/1.1 200 Connection Established\r\n\r\n")
	if err != nil {
		cconn.Close()
		return nil, err
	}

	f.Response = &Response{
		StatusCode: 200,
		Header:     make(http.Header),
	}

	return cconn, nil
}

func (e *entry) directTransfer(res http.ResponseWriter, req *http.Request, f *Flow) {
	proxy := e.proxy
	conn, err := proxy.getUpstreamConn(req.Context(), req)
	if err != nil {
		log.Debug(err)
		res.WriteHeader(502)
		return
	}
	defer conn.Close()

	cconn, err := e.establishConnection(res, f)
	if err != nil {
		log.Debug(err)
		return
	}
	defer cconn.Close()

	transfer(conn, cconn)
}

func (e *entry) httpsDialFirstAttack(res http.ResponseWriter, req *http.Request, f *Flow) {
	proxy := e.proxy
	conn, err := proxy.attacker.httpsDial(req.Context(), req)
	if err != nil {
		log.Error(err)
		res.WriteHeader(502)
		return
	}

	cconn, err := e.establishConnection(res, f)
	if err != nil {
		conn.Close()
		log.Error(err)
		return
	}

	peek, err := cconn.(*wrapClientConn).Peek(3)
	if err != nil {
		cconn.Close()
		conn.Close()
		log.Error(err)
		return
	}
	if !helper.IsTls(peek) {
		// todo: http, ws
		transfer(conn, cconn)
		cconn.Close()
		conn.Close()
		return
	}

	// is tls
	f.ConnContext.ClientConn.Tls = true
	proxy.attacker.httpsTlsDial(req.Context(), cconn, conn)
}

func (e *entry) httpsDialLazyAttack(res http.ResponseWriter, req *http.Request, f *Flow) {
	proxy := e.proxy
	cconn, err := e.establishConnection(res, f)
	if err != nil {
		log.Error(err)
		return
	}

	peek, err := cconn.(*wrapClientConn).Peek(3)
	if err != nil {
		cconn.Close()
		log.Error(err)
		return
	}

	if !helper.IsTls(peek) {
		// todo: http, ws
		conn, err := proxy.attacker.httpsDial(req.Context(), req)
		if err != nil {
			cconn.Close()
			log.Error(err)
			return
		}
		transfer(conn, cconn)
		conn.Close()
		cconn.Close()
		return
	}

	// is tls
	f.ConnContext.ClientConn.Tls = true
	proxy.attacker.httpsLazyAttack(req.Context(), cconn, req)
}
