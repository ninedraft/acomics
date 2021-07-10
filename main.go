package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.con/ninedraft/acomics/client"

	"github.com/cheggaaa/pb/v3"
	"golang.org/x/sync/errgroup"
)

func main() {
	var client = client.NewClient("gunnerkrigg")
	var ctx, cancel = context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	_ = os.MkdirAll("pages", 0775)

	const firstPage = 1547
	const lastPage = 2490
	var bar = pb.StartNew(lastPage)
	bar.Add(firstPage)
	defer bar.Finish()

	wg, ctx := errgroup.WithContext(ctx)
	var downloadIssue = func(id int) {
		wg.Go(func() error {
			var errDownload = downloadFile(ctx, client, id)
			if errDownload != nil {
				log.Printf("unable to download page %d: %v", id, errDownload)
				return errDownload
			}
			bar.Increment()
			return nil
		})
	}
	for id := firstPage; id <= lastPage; id++ {
		downloadIssue(id)
		time.Sleep(500 * time.Millisecond)
	}
	var errWait = wg.Wait()
	if errWait != nil {
		panic(errWait)
	}
}

func downloadFile(ctx context.Context, client *client.Client, id int) error {
	var filename = fmt.Sprintf("pages/%04d.jpeg", id)
	var file, errFile = os.Create(filename)
	if errFile != nil {
		return fmt.Errorf("creating file: %w", errFile)
	}
	defer file.Close()
	return client.FetchIssue(ctx, id, file)
}
