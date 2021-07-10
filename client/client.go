package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	schema = "https://"
	site   = "acomics.ru"
)

type Client struct {
	comic string
	c     *http.Client
}

func NewClient(comic string) *Client {
	return &Client{
		comic: "~" + comic,
		c:     newDefaultHTTPClient(),
	}
}

var errImageNotFound = errors.New("main image is not found")

func (client *Client) FetchIssue(ctx context.Context, id int, dst io.Writer) error {
	var resp, errResp = client.fetchPage(ctx, client.comicPageURL(id))
	if errResp != nil {
		return fmt.Errorf("fetching page %d", id)
	}
	defer resp.Body.Close()

	var DOM, errParsing = goquery.NewDocumentFromReader(resp.Body)
	if errParsing != nil {
		return fmt.Errorf("parsing page: %w", errParsing)
	}
	var imageURL, imageOK = DOM.Find("#mainImage").
		First().
		Attr("src")
	if !imageOK {
		return errImageNotFound
	}
	var imageData, errImage = client.fetchPage(ctx, client.comicFile(imageURL))
	if errImage != nil {
		return fmt.Errorf("loading image: %w", errImage)
	}
	defer imageData.Body.Close()

	var _, errCopy = io.Copy(dst, imageData.Body)
	if errCopy != nil {
		return fmt.Errorf("streaming image: %w", errCopy)
	}
	return nil
}

func (client *Client) comicPageURL(id int) string {
	return schema + path.Join(site, client.comic, strconv.Itoa(id))
}

func (client *Client) comicFile(link string) string {
	return schema + path.Join(site, link)
}

var errUnexpectedStatus = errors.New("unexpected status")

func (client *Client) fetchPage(ctx context.Context, pageURL string) (*http.Response, error) {
	var req, errReq = http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if errReq != nil {
		return nil, fmt.Errorf("preparing request: %w", errReq)
	}
	var resp, errResp = client.c.Do(req)
	if errResp != nil {
		return nil, fmt.Errorf("executing request: %w", errResp)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: %s", errUnexpectedStatus, resp.Status)
	}
	return resp, nil
}

func newDefaultHTTPClient() *http.Client {
	var dialer = (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	var transport = &http.Transport{
		DialContext:           dialer,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          1,
		MaxConnsPerHost:       2,
		ReadBufferSize:        1 << 20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}
