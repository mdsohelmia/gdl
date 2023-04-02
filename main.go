package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/k0kubun/pp"
	"github.com/mdsohelmia/gcdownloader/pkg/downloader"
	"github.com/schollz/progressbar/v3"
)

// Our pipeline uses the following components:
//1.Fetch Metadata from URL
//2.Worker
//3.combiner

type DownloadInfo struct {
	URL          string
	FilePath     string
	Size         int64
	Downloaded   int64
	Bandwidth    int64
	Remaining    time.Duration
	Resumable    bool
	Segments     int
	SegSize      int64
	SegDownloads []SegmentDownloadInfo
}

type SegmentDownloadInfo struct {
	ID         int
	Start      int64
	End        int64
	Downloaded int64
	Complete   bool
}

func main() {
	downloader := downloader.NewDownloader(&downloader.Config{
		Url:          "https://storage.googleapis.com/muxdemofiles/mux-video-intro.mp4",
		ShowProgress: false,
		RootPath:     "downloads",
		Debug:        false,
	})
	downloader.Hook = func(resp *http.Response, progressbar *progressbar.ProgressBar, err error) error {
		if err != nil {
			pp.Println(err)
		}
		return nil
	}
	downloader.SetBaseFolder(fmt.Sprintf("%d", time.Now().Unix()))
	err := downloader.Download()
	fmt.Println("Downloaded Size:", downloader.GetFileSize())
	fmt.Println("Downloaded filename:", downloader.GetPath())
	fmt.Println("Downloaded Path:", downloader.GetPath())
	pp.Println(err)
}
