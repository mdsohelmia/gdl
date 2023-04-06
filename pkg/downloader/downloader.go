package downloader

import (
	"fmt"
	"time"

	"github.com/cavaliergopher/grab/v3"
	"github.com/stoewer/go-strcase"
)

// Downloader is the main struct
type Downloader struct {
	OriginUrl string
	Filename  string
	RootPath  string
	Header    map[string]string
	client    *grab.Client
	Url       string
	Hook      Hook
	Size      int64
}

// NewDownloader creates a new Downloader
// with the given URL and file path.
// The concurrency parameter specifies the number of threads
func NewDownloader(config *Config) *Downloader {
	if config.RootPath == "" {
		config.RootPath = "downloads"
	}
	d := &Downloader{
		client:   grab.NewClient(),
		Url:      config.Url,
		RootPath: config.RootPath,
	}

	return d
}

// SetFilename sets the filename
func (d *Downloader) SetFilename(filename string) {
	d.Filename = strcase.SnakeCase(filename)
}

// Download downloads the file partial
func (d *Downloader) Download(outputPath string) (string, error) {
	req, err := grab.NewRequest(fmt.Sprintf("%s/%s", d.RootPath, outputPath), d.Url)
	if err != nil {
		return "", err
	}
	resp := d.client.Do(req)
	d.OriginUrl = resp.Request.URL().String()
	d.Size = resp.Size()
	// start Hook loop
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
Loop:
	for {
		select {
		case <-t.C:
			if d.Hook != nil {
				d.Hook(resp)
			}
		case <-resp.Done:
			break Loop
		}
	}
	// check for errors
	if err := resp.Err(); err != nil {
		return "", err
	}
	return resp.Filename, nil
}

func (d *Downloader) GetFilename() string {
	return d.Filename
}

func (d *Downloader) GetFullPath() string {
	return fmt.Sprintf("%s/%s", d.RootPath, d.Filename)
}
func (d *Downloader) GetOriginUrl() string {
	return d.OriginUrl
}
