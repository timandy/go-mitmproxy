package main

import (
	"github.com/lqqyt2423/go-mitmproxy/log"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

type RewriteHost struct {
	proxy.BaseAddon
}

func (a *RewriteHost) ClientConnected(client *proxy.ClientConn) {
	// necessary
	client.UpstreamCert = false
}

func (a *RewriteHost) Request(f *proxy.Flow) {
	log.Infof("Host: %v, Method: %v, Scheme: %v", f.Request.URL.Host, f.Request.Method, f.Request.URL.Scheme)
	f.Request.URL.Host = "www.baidu.com"
	f.Request.URL.Scheme = "http"
	log.Infof("After: %v", f.Request.URL)
}

func main() {
	opts := &proxy.Options{
		Addr:              ":9080",
		StreamLargeBodies: 1024 * 1024 * 5,
	}

	p, err := proxy.NewProxy(opts)
	if err != nil {
		log.Fatal(err)
	}

	p.AddAddon(&RewriteHost{})
	p.AddAddon(&proxy.LogAddon{})

	p.Start()
}
