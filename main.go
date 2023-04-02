package main

import (
	"fmt"
	"net/http"

	"github.com/k0kubun/pp"
	"github.com/mdsohelmia/gdl/pkg/downloader"
	"github.com/schollz/progressbar/v3"
)

func main() {
	downloader, err := downloader.NewDownloader(&downloader.Config{
		Url:            "https://storage.googleapis.com/muxdemofiles/mux-video-intro.mp4",
		ShowProgress:   true,
		RootPath:       "downloads",
		Debug:          true,
		CopyBufferSize: 1024,
	})
	if err != nil {
		return
	}
	if downloader.AllReadyExist() {
		fmt.Println("Downloaded URL:", downloader.GetOriginUrl())
		fmt.Println("Downloaded Size:", downloader.GetFileSize())
		fmt.Println("Downloaded filename:", downloader.GetFilename())
		fmt.Println("Downloaded Path:", downloader.GetPath())
		fmt.Println("Downloaded URL:", downloader.GetOriginUrl())
		return
	}

	downloader.Hook = func(resp *http.Response, progressbar *progressbar.ProgressBar, err error) error {
		if err != nil {
			pp.Println(err)
		}
		return nil
	}
	downloader.SetBaseFolder("1680426182")
	fmt.Println("Downloaded URL:", downloader.GetOriginUrl())
	fmt.Println("Downloaded Size:", downloader.GetFileSize())
	fmt.Println("Downloaded filename:", downloader.GetFilename())
	fmt.Println("Downloaded Path:", downloader.GetPath())
	fmt.Println("Downloaded URL:", downloader.GetOriginUrl())

	err = downloader.Download()

	pp.Println(err)
}
