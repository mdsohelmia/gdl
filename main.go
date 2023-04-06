package main

import (
	"fmt"
	"os"
	"time"

	"github.com/cavaliergopher/grab/v3"
	"github.com/inhies/go-bytesize"
	"github.com/k0kubun/pp"
	"github.com/mdsohelmia/gdl/pkg/downloader"
)

func main() {
	downloader := downloader.NewDownloader(&downloader.Config{
		Url: "https://player.vimeo.com/progressive_redirect/playback/807879296/rendition/1080p/file.mp4?loc=external&oauth2_token_id=1713368872&signature=8c17d337ee5ddee975c67bc74405206ea18045d30620d558f4caa3b2dfb3dded",
	})

	downloader.Hook = func(response *grab.Response) {
		fmt.Printf("transferred %v / %v bytes (%.2f%%)\n",
			response.BytesComplete(),
			response.Size,
			100*response.Progress())
	}

	outputPath, err := downloader.Download(fmt.Sprintf("videos/%d.mp4", time.Now().Unix()))

	if err != nil {
		panic(err)
	}
	pp.Println("Downloaded file size: ", bytesize.New(float64(downloader.Size)).String())
	pp.Println(outputPath)
	fileInfo, _ := os.Stat(outputPath)
	fmt.Println("Save file", bytesize.New(float64(fileInfo.Size())).String())
}
