package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.con/ninedraft/acomics/client"
	"github.con/ninedraft/acomics/file"

	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

func main() {
	var from = 1
	pflag.IntVar(&from, "from", from, `first page to download`)
	var to = -1
	pflag.IntVar(&to, "to", to, `last page to download, if -1, then download until not found`)
	pflag.Parse()

	if from < to && to > 0 {
		panic("--from must be less then --to")
	}
	var client, errClient = client.NewClient(client.Config{
		Comic:    pflag.Arg(0),
		ProxyURL: client.SOCKS5("localhost:9050", nil),
	})
	if errClient != nil {
		panic(errClient)
	}
	var ctx, cancel = context.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	if to < 0 {
		var totalPages, errTotalPages = client.TotalPages(ctx)
		if errTotalPages != nil {
			panic(errTotalPages)
		}
		to = totalPages
	}

	_ = os.MkdirAll("pages", 0775)
	var errDownload = (&downloader{
		client:  client,
		from:    from,
		to:      to,
		trottle: 500 * time.Millisecond,
		dir:     "pages",
	}).Run(ctx)
	if errDownload != nil {
		panic(errDownload)
	}
}

type downloader struct {
	client   Client
	from, to int
	trottle  time.Duration
	dir      string
}

type Client interface {
	TotalPages(ctx context.Context) (int, error)
	FetchIssue(ctx context.Context, id int) (*file.File, error)
}

func (d *downloader) Run(ctx context.Context) error {
	var to = d.to
	if to < 0 {
		to = 0
	}
	wg, ctx := errgroup.WithContext(ctx)
	var tickets = make(chan struct{}, 2)
	var downloadIssue = func(id int) {
		wg.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case tickets <- struct{}{}:
				// pass
			}
			defer func() { <-tickets }()
			var tick = time.Now()
			var errDownload = d.downloadFile(ctx, id)
			if errDownload != nil {
				log.Printf("unable to download page %d: %v", id, errDownload)
				return errDownload
			}
			log.Printf("downloaded issue %d in %s", id, time.Since(tick))
			return nil
		})
	}

	var ticker = time.NewTicker(d.trottle)
	defer ticker.Stop()

pagesDownloading:
	for id := d.from; (id <= d.to || d.to < 0) && ctx.Err() == nil; id++ {
		downloadIssue(id)
		select {
		case <-ctx.Done():
			break pagesDownloading
		case <-ticker.C:
		}
	}

	return wg.Wait()
}

func (d *downloader) downloadFile(ctx context.Context, id int) error {
	var tmp = d.tmp(id)
	if fileExists(tmp) {
		_ = os.Remove(tmp)
	}
	var filename = d.filename(id)
	if fileExists(filename) {
		return nil
	}

	if err := d.saveFile(ctx, id, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, filename)
}

func (d *downloader) saveFile(ctx context.Context, id int, dst string) error {
	var img, errIssue = d.client.FetchIssue(ctx, id)
	if errIssue != nil {
		return fmt.Errorf("fetchin image: %w", errIssue)
	}

	var file, errFile = os.Create(dst + img.Ext)
	if errFile != nil {
		return fmt.Errorf("creating file: %w", errFile)
	}
	defer file.Close()

	var _, errCopy = io.Copy(file, img.Data)
	if errCopy != nil {
		return fmt.Errorf("downloading image: %w", errCopy)
	}
	return nil
}

func (d *downloader) filename(id int) string {
	return filepath.Join(d.dir, fmt.Sprintf(d.nameFormat(), id))
}

func (d *downloader) tmp(id int) string {
	return filepath.Join(d.dir, fmt.Sprintf(d.nameFormat()+"tmp", id))
}

func (d *downloader) nameFormat() string {
	var width = 0
	var max = d.to
	if max < 0 {
		return "%05d"
	}
	for max > 0 {
		max /= 10
		width++
	}
	return "%0" + strconv.Itoa(width) + "d"
}

func fileExists(name string) bool {
	var _, err = os.Stat(name)
	return err == nil
}
