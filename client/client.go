package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

const (
	schema = "https://"
	site   = "acomics.ru"
)

type Client struct {
	cfg   Config
	comic string
	c     *http.Client
}

type Config struct {
	Comic    string
	ProxyURL *url.URL
}

func NewClient(cfg Config) (*Client, error) {
	var transport, errTransport = cfg.transport(newDialer())
	if errTransport != nil {
		return nil, fmt.Errorf("creating HTTP transport: %w", errTransport)
	}
	return &Client{
		cfg:   cfg,
		comic: "~" + cfg.Comic,
		c: &http.Client{
			Transport: transport,
		},
	}, nil
}

func SOCKS5(host string, auth *proxy.Auth) *url.URL {
	var u = &url.URL{
		Scheme: "socks5",
		Host:   host,
	}
	if auth != nil {
		u.User = url.UserPassword(auth.User, auth.Password)
	}
	return u
}

func (config *Config) transport(dialer netDialer) (*http.Transport, error) {
	if config == nil || config.ProxyURL == nil {
		return newTransport(dialer), nil
	}
	var proxyDialer, errProxy = proxy.FromURL(config.ProxyURL, dialer)
	if errProxy != nil {
		return nil, fmt.Errorf("creating proxy: %w", errProxy)
	}
	return newTransport(proxyDialer), nil
}

func newTransport(dialer netDialer) *http.Transport {
	var transport = &http.Transport{
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          1,
		MaxConnsPerHost:       10,
		ReadBufferSize:        1 << 20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	switch dialer := dialer.(type) {
	case netContextDialer:
		transport.DialContext = dialer.DialContext
	default:
		transport.Dial = dialer.Dial
	}
	return transport
}

type netDialer interface {
	Dial(network, address string) (net.Conn, error)
}

type netContextDialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

func newDialer() *net.Dialer {
	return &net.Dialer{
		Timeout:   5 * time.Minute,
		KeepAlive: 30 * time.Second,
	}
}
