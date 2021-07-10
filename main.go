package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.con/ninedraft/acomics/client"
	"github.con/ninedraft/acomics/file"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

func main() {
	var from = 1
	pflag.IntVar(&from, "from", from, `first page to download`)
	var to = -1
	pflag.IntVar(&to, "to", to, `last page to download, if -1, then download until not found`)
	var cbz string
	pflag.StringVar(&cbz, "cbz", cbz, `output cbz file, will not be generated if empty, will use comic name if @auto`)
	pflag.Parse()

	var comic = pflag.Arg(0)
	if cbz == "@auto" {
		cbz = comic + ".cbz"
	}

	if from < to && to > 0 {
		panic("--from must be less then --to")
	}
	var client, errClient = client.NewClient(client.Config{
		Comic:    comic,
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

	var imageDir = filepath.Join(os.TempDir(), comic)
	log.Printf("storing images in %s", imageDir)
	_ = os.MkdirAll(imageDir, 0775)
	var errDownload = (&downloader{
		client:  client,
		from:    from,
		to:      to,
		trottle: 500 * time.Millisecond,
		dir:     imageDir,
		cbz:     cbz,
		tickets: make(chan struct{}, 2),
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
	cbz      string
	tickets  chan struct{}
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
	var bar = progressbar.Default(int64(d.to - d.from))
	defer bar.Finish()

	var downloadIssue = func(id int) {
		wg.Go(func() error {
			var desc = fmt.Sprintf("page %d", id)
			bar.Describe(desc)
			var errDownload = d.downloadFile(ctx, id)
			if errDownload != nil {
				log.Printf("unable to download page %d: %v", id, errDownload)
				return errDownload
			}
			bar.Add(1)
			return nil
		})
	}

	for id := d.from; id <= d.to && ctx.Err() == nil; id++ {
		downloadIssue(id)
	}

	var errWait = wg.Wait()
	if errWait != nil {
		return fmt.Errorf("downloading images: %w", errWait)
	}
	if d.cbz == "" {
		return nil
	}
	var errCBZ = d.generateCBZ()
	if errCBZ != nil {
		return fmt.Errorf("generating cbz file: %w", errCBZ)
	}
	return nil
}

func (d *downloader) downloadFile(ctx context.Context, id int) error {
	var tmp = d.tmp(id)
	if fileExists(tmp) {
		_ = os.Remove(tmp)
	}

	var filename = d.filename(id)
	if fileExists(filename+".jpeg") ||
		fileExists(filename+".png") ||
		fileExists(filename+".gif") {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case d.tickets <- struct{}{}:
		defer func() { <-d.tickets }()
		defer time.Sleep(d.trottle)
	}

	var img, errIssue = d.client.FetchIssue(ctx, id)
	if errIssue != nil {
		return fmt.Errorf("fetchin image: %w", errIssue)
	}
	defer img.Data.Close()
	filename += img.Ext

	var file, errFile = os.Create(tmp)
	if errFile != nil {
		return fmt.Errorf("creating file: %w", errFile)
	}
	defer file.Close()

	var _, errCopy = io.Copy(file, img.Data)
	if errCopy != nil {
		return fmt.Errorf("downloading image: %w", errCopy)
	}

	return os.Rename(tmp, filename)
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

func (d *downloader) generateCBZ() error {
	var output, errOutput = os.Create(d.cbz)
	if errOutput != nil {
		return fmt.Errorf("creating cbz file: %w", errOutput)
	}
	defer output.Close()
	var files, errFiles = os.ReadDir(d.dir)
	if errFiles != nil {
		return fmt.Errorf("reading cache dir: %w", errFiles)
	}

	var archive = zip.NewWriter(output)
	for _, fileInfo := range files {
		var f, errFile = archive.Create(fileInfo.Name())
		if errFile != nil {
			return fmt.Errorf("storing file %s: %w", fileInfo.Name(), errFile)
		}
		var filename = path.Join(d.dir, fileInfo.Name())
		var errRead = readFileTo(f, filename)
		if errRead != nil {
			return fmt.Errorf("storing file %s: %w", fileInfo.Name(), errRead)
		}
	}
	return archive.Close()
}

func readFileTo(dst io.Writer, name string) error {
	var src, errSrc = os.Open(name)
	if errSrc != nil {
		return fmt.Errorf("opening source file %s: %w", name, errSrc)
	}
	defer src.Close()
	var _, errCopy = io.Copy(dst, src)
	if errCopy != nil {
		return fmt.Errorf("copying file %s: %w", name, errCopy)
	}
	return nil
}
