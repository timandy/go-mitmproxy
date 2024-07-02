package proxy

import (
	"crypto/tls"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/lqqyt2423/go-mitmproxy/log"
)

// 当前仅做了转发 websocket 流量

type webSocket struct{}

var defaultWebSocket webSocket

// func (s *webSocket) ws(conn net.Conn, host string) {
// 	log := log.WithField("in", "webSocket.ws").WithField("host", host)

// 	defer conn.Close()
// 	remoteConn, err := net.Dial("tcp", host)
// 	if err != nil {
// 		logErr(err)
// 		return
// 	}
// 	defer remoteConn.Close()
// 	transfer(log, conn, remoteConn)
// }

func (s *webSocket) wss(res http.ResponseWriter, req *http.Request) {
	upgradeBuf, err := httputil.DumpRequest(req, false)
	if err != nil {
		log.Errorf("DumpRequest: %v", err)
		res.WriteHeader(502)
		return
	}

	cconn, _, err := res.(http.Hijacker).Hijack()
	if err != nil {
		log.Errorf("Hijack: %v", err)
		res.WriteHeader(502)
		return
	}
	defer cconn.Close()

	host := req.Host
	if !strings.Contains(host, ":") {
		host = host + ":443"
	}
	conn, err := tls.Dial("tcp", host, nil)
	if err != nil {
		log.Errorf("tls.Dial: %v", err)
		return
	}
	defer conn.Close()

	_, err = conn.Write(upgradeBuf)
	if err != nil {
		log.Errorf("wss upgrade: %v", err)
		return
	}
	transfer(conn, cconn)
}
