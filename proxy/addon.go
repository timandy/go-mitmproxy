package proxy

import (
	"net/http"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/log"
)

type Addon interface {
	// A client has connected to mitmproxy. Note that a connection can correspond to multiple HTTP requests.
	ClientConnected(*ClientConn)

	// A client connection has been closed (either by us or the client).
	ClientDisconnected(*ClientConn)

	// Mitmproxy has connected to a server.
	ServerConnected(*ConnContext)

	// A server connection has been closed (either by us or the server).
	ServerDisconnected(*ConnContext)

	// The TLS handshake with the server has been completed successfully.
	TlsEstablishedServer(*ConnContext)

	// The request flow begin
	BeginFlow(*Flow)

	// The request flow is completed
	EndFlow(*Flow)

	// The full HTTP request has been read.
	Request(*Flow)

	// The full HTTP response has been read.
	Response(*Flow)

	// onAccessProxyServer
	AccessProxyServer(req *http.Request, res http.ResponseWriter)
}

// BaseAddon do nothing
type BaseAddon struct{}

func (addon *BaseAddon) ClientConnected(*ClientConn)                          {}
func (addon *BaseAddon) ClientDisconnected(*ClientConn)                       {}
func (addon *BaseAddon) ServerConnected(*ConnContext)                         {}
func (addon *BaseAddon) ServerDisconnected(*ConnContext)                      {}
func (addon *BaseAddon) TlsEstablishedServer(*ConnContext)                    {}
func (addon *BaseAddon) BeginFlow(*Flow)                                      {}
func (addon *BaseAddon) EndFlow(*Flow)                                        {}
func (addon *BaseAddon) Request(*Flow)                                        {}
func (addon *BaseAddon) Response(*Flow)                                       {}
func (addon *BaseAddon) AccessProxyServer(*http.Request, http.ResponseWriter) {}

// LogAddon log connection and flow
type LogAddon struct {
	BaseAddon
}

func (addon *LogAddon) ClientConnected(client *ClientConn) {
	log.Infof("%v client connect", client.Conn.RemoteAddr())
}

func (addon *LogAddon) ClientDisconnected(client *ClientConn) {
	log.Infof("%v client disconnect", client.Conn.RemoteAddr())
}

func (addon *LogAddon) ServerConnected(connCtx *ConnContext) {
	log.Infof("%v server connect %v (%v->%v)", connCtx.ClientConn.Conn.RemoteAddr(), connCtx.ServerConn.Address, connCtx.ServerConn.Conn.LocalAddr(), connCtx.ServerConn.Conn.RemoteAddr())
}

func (addon *LogAddon) ServerDisconnected(connCtx *ConnContext) {
	log.Infof("%v server disconnect %v (%v->%v) - %v", connCtx.ClientConn.Conn.RemoteAddr(), connCtx.ServerConn.Address, connCtx.ServerConn.Conn.LocalAddr(), connCtx.ServerConn.Conn.RemoteAddr(), connCtx.FlowCount.Load())
}

func (addon *LogAddon) Request(f *Flow) {
	log.Debugf("%v Request %v %v", f.ConnContext.ClientConn.Conn.RemoteAddr(), f.Request.Method, f.Request.URL.String())
	start := time.Now()
	go func() {
		<-f.Done()
		var StatusCode int
		if f.Response != nil {
			StatusCode = f.Response.StatusCode
		}
		var contentLen int
		if f.Response != nil && f.Response.Body != nil {
			contentLen = len(f.Response.Body)
		}
		log.Infof("%v %v %v %v %v - %v ms", f.ConnContext.ClientConn.Conn.RemoteAddr(), f.Request.Method, f.Request.URL.String(), StatusCode, contentLen, time.Since(start).Milliseconds())
	}()
}

type UpstreamCertAddon struct {
	BaseAddon
	UpstreamCert bool // Connect to upstream server to look up certificate details.
}

func NewUpstreamCertAddon(upstreamCert bool) *UpstreamCertAddon {
	return &UpstreamCertAddon{UpstreamCert: upstreamCert}
}

func (addon *UpstreamCertAddon) ClientConnected(conn *ClientConn) {
	conn.UpstreamCert = addon.UpstreamCert
}
