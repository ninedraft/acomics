package client

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/proxy"
)

func NewClientWithProxy(comic, proxyAddr string) (*Client, error) {
	var c, errDialer = newProxyClient(proxyAddr)
	if errDialer != nil {
		return nil, errDialer
	}
	return &Client{
		comic: "~" + comic,
		c:     c,
	}, nil
}

func newProxyClient(proxyAddr string) (*http.Client, error) {
	var dialer = &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	var proxyDialer, errDialer = proxy.SOCKS5("tcp", proxyAddr, nil, dialer)
	if errDialer != nil {
		return nil, fmt.Errorf("preparing dialer: %w", errDialer)
	}
	var transport = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		Dial:                  proxyDialer.Dial,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}, nil
}
