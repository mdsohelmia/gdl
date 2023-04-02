package main

import (
	"fmt"
	"net/http"

	"github.com/k0kubun/pp"
	"github.com/mdsohelmia/gcdownloader/pkg/downloader"
	"github.com/schollz/progressbar/v3"
)

func main() {
	downloader := downloader.NewDownloader(&downloader.Config{
		Url:            "https://player.vimeo.com/progressive_redirect/download/806339269/container/752c299c-0e0d-4e45-8687-819aaf46180a/6b8d9238-7054c831/0007_7_social_networks_i--%5Btutflix.org%5D--%20%281080p%29.mp4?expires=1680508081&loc=external&signature=eba2f549a428c6c97779509c4626c4036fa46fd8f32287a36935a9a31579cc60",
		ShowProgress:   true,
		RootPath:       "downloads",
		Debug:          true,
		CopyBufferSize: 1024,
	})
	downloader.Hook = func(resp *http.Response, progressbar *progressbar.ProgressBar, err error) error {
		if err != nil {
			pp.Println(err)
		}
		return nil
	}
	downloader.SetBaseFolder("1680426182")
	err := downloader.Download()
	fmt.Println("Downloaded URL:", downloader.GetOriginUrl())
	fmt.Println("Downloaded Size:", downloader.GetFileSize())
	fmt.Println("Downloaded filename:", downloader.GetPath())
	fmt.Println("Downloaded Path:", downloader.GetPath())
	fmt.Println("Downloaded URL:", downloader.GetOriginUrl())
	pp.Println(err)
}
