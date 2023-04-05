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
		Url:            "https://player.vimeo.com/progressive_redirect/playback/807882959/rendition/1080p/file.mp4?loc=external&oauth2_token_id=1713368872&signature=5bebcb3d9493ba314027adeaa8bb2f85f753c13d6a96a0a7ad2426910dd1f6f6",
		ShowProgress:   true,
		RootPath:       "downloads",
		Debug:          true,
		CopyBufferSize: 1024,
	})
	if err != nil {
		return
	}
	downloader.SetBaseFolder("1680426182")
	downloader.SetFilename("test gotipath.mp4")

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

	fmt.Println("Downloaded URL:", downloader.GetOriginUrl())
	fmt.Println("Downloaded Size:", downloader.GetFileSize())
	fmt.Println("Downloaded filename:", downloader.GetFilename())
	fmt.Println("Downloaded Path:", downloader.GetPath())
	fmt.Println("Downloaded URL:", downloader.GetOriginUrl())

	err = downloader.Download()

	pp.Println(err)
}
