package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.con/ninedraft/acomics/file"
)

var (
	errImageNotFound = errors.New("main image is not found")
	ErrIssueNotFound = errors.New("issue is not found")
)

func (client *Client) TotalPages(ctx context.Context) (int, error) {
	var comicPage = schema + path.Join(site, client.comic)
	var resp, errResp = client.fetchPage(ctx, comicPage)
	if errResp != nil {
		return 0, fmt.Errorf("fetching main comic page: %w", errResp)
	}
	defer resp.Body.Close()

	var DOM, errParsing = goquery.NewDocumentFromReader(resp.Body)
	if errParsing != nil {
		return 0, fmt.Errorf("parsing page: %w", errParsing)
	}
	var raw = DOM.Find(".issueNumber").First().Text()
	var token = strings.SplitN(raw, "/", 2)[1]
	var n, errParseNumber = strconv.Atoi(token)
	if errParseNumber != nil {
		return 0, fmt.Errorf("parsing total page numbers: %w", errParseNumber)
	}
	return n, nil
}

func (client *Client) FetchIssue(ctx context.Context, id int) (*file.File, error) {
	var resp, errResp = client.fetchPage(ctx, client.comicPageURL(id))
	if errResp != nil {
		return nil, fmt.Errorf("fetching page %d: %w", id, errResp)
	}
	defer resp.Body.Close()

	var DOM, errParsing = goquery.NewDocumentFromReader(resp.Body)
	if errParsing != nil {
		return nil, fmt.Errorf("parsing page: %w", errParsing)
	}
	var actualIssueID = issueNumber(DOM)
	if actualIssueID != id {
		return nil, fmt.Errorf("%w: trying to fetch issue %d, got %d", ErrIssueNotFound, id, actualIssueID)
	}
	var imageURL, imageOK = DOM.Find("#mainImage").
		First().
		Attr("src")
	if !imageOK {
		return nil, errImageNotFound
	}

	var imageData, errImage = client.fetchPage(ctx, client.comicFile(imageURL))
	if errImage != nil {
		return nil, fmt.Errorf("loading image: %w", errImage)
	}
	defer imageData.Body.Close()

	return file.FromResponse(resp), nil
}

func issueNumber(DOM *goquery.Document) int {
	var raw = DOM.Find(".issueNumber").First().Text()
	var token = strings.SplitN(raw, "/", 2)[0]
	var n, _ = strconv.Atoi(token)
	return n
}

func (client *Client) comicPageURL(id int) string {
	return schema + path.Join(site, client.comic, strconv.Itoa(id))
}

func (client *Client) comicFile(link string) string {
	return schema + path.Join(site, link)
}

type ErrUnexpectedStatus int

func (status ErrUnexpectedStatus) Error() string {
	var text = http.StatusText(int(status))
	return fmt.Sprintf("unexpected status %d %s", status, text)
}

func (status ErrUnexpectedStatus) Code() int { return int(status) }

func (client *Client) fetchPage(ctx context.Context, pageURL string) (*http.Response, error) {
	var req, errReq = http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if errReq != nil {
		return nil, fmt.Errorf("preparing request: %w", errReq)
	}
	req.Header.Set("User-Agent", "acomics")
	var resp, errResp = client.c.Do(req)
	if errResp != nil {
		return nil, fmt.Errorf("executing request: %w", errResp)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, ErrUnexpectedStatus(resp.StatusCode)
	}
	return resp, nil
}
