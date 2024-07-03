package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/lqqyt2423/go-mitmproxy/internal/helper"
	"github.com/lqqyt2423/go-mitmproxy/log"
)

var normalErrMsgs = []string{
	"read: connection reset by peer",
	"write: broken pipe",
	"i/o timeout",
	"net/http: TLS handshake timeout",
	"io: read/write on closed pipe",
	"connect: connection refused",
	"connect: connection reset by peer",
	"use of closed network connection",
}

// 仅打印预料之外的错误信息
func logErr(err error) (loged bool) {
	msg := err.Error()

	for _, str := range normalErrMsgs {
		if strings.Contains(msg, str) {
			log.Debug(err)
			return
		}
	}

	log.Debug(err)
	loged = true
	return
}

// 转发流量
func transfer(server, client io.ReadWriteCloser) {
	//异步转发响应; 客户端<--代理(转发)<--服务端
	go func() {
		_, err := io.Copy(client, server)
		if err != nil {
			logErr(fmt.Errorf("copy client to server error. %v", err))
			return
		}
	}()
	//同步转发请求; 客户端-->代理(转发)-->服务端
	_, err := io.Copy(server, client)
	if err != nil {
		logErr(fmt.Errorf("copy client to server error. %v", err))
		return
	}
}

// 转发 http 流量
func transferHttp(w http.ResponseWriter, req *http.Request) {
	// 获取客户端 tcp 连接
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()
	// 连接服务器, 获取连接
	address, host, _ := getAddress(req.Host)
	serverConn, err := net.Dial("tcp", address)
	if err != nil {
		http.Error(w, "Error connecting to remote server", http.StatusInternalServerError)
		return
	}
	defer serverConn.Close()
	//重新构建在 hijack 之前已经读取的内容
	requestLine := getRequestLine(req)
	header := getRequestHeader(req, host)
	//在内存组合
	buff := bytes.NewBuffer([]byte(requestLine))
	_ = header.Write(buff)
	buff.Write([]byte("\r\n\r\n"))
	// 将 buff 内容写入服务端连接
	_, err = serverConn.Write(buff.Bytes())
	if err != nil {
		logErr(fmt.Errorf("copy client to server error. %v", err))
		return
	}
	//四层转发
	transfer(serverConn, clientConn)
}

// 分解地址和端口号
func getAddress(h string) (address, host, port string) {
	host, port = helper.SplitHostPort(h)
	if port == "" {
		port = "80"
	}
	address = helper.JoinHostPort(host, port)
	return
}

// 组合请求行
func getRequestLine(req *http.Request) string {
	url := req.URL
	path := url.Path
	if url.RawQuery != "" {
		path += "?" + url.RawQuery
	}
	return fmt.Sprintf("%v %v %v\r\n", req.Method, path, req.Proto)
}

// 处理请求头
func getRequestHeader(req *http.Request, host string) http.Header {
	header := req.Header
	header.Del("Proxy-Connection")
	header.Add("Host", host)
	return header
}
