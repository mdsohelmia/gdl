package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/schollz/progressbar/v3"
	"github.com/stoewer/go-strcase"
)

type Downloader struct {
	Filename       string
	FilePath       string
	RootPath       string
	Header         map[string]string
	CopyBufferSize int
	client         *http.Client
	URL            string
	//file size in bytes
	Size int64
	//downloaded size in bytes
	DownloadedSize int64
	//download speed in bytes per second
	Bandwidth int64
	//remaining time
	Remaining time.Duration
	//is resumable
	Resumable bool
	//is paused
	paused bool
	//is resumed
	resume bool
	//number of segments
	Segments int
	// concurrent downloads
	concurrency int
	// use to pause the download gracefully
	context      context.Context
	cancel       context.CancelFunc
	bar          *progressbar.ProgressBar
	Hook         Hook
	ShowProgress bool
}

type Result struct {
	Size     int64
	URL      string
	Path     string
	Filename string
}

type Config struct {
	Url            string
	ShowProgress   bool
	Concurrency    int
	RootPath       string
	CopyBufferSize int
}

// NewDownloader creates a new Downloader
// with the given URL and file path.
// The concurrency parameter specifies the number of threads
func NewDownloader(config *Config) *Downloader {
	if config.Concurrency == 0 {
		config.Concurrency = runtime.NumCPU()
	}
	if config.RootPath == "" {
		config.RootPath = "downloads"
	}

	if config.CopyBufferSize == 0 {
		config.CopyBufferSize = 1024
	}

	retryablehttpClient := retryablehttp.NewClient()
	retryablehttpClient.RetryMax = 10
	retryablehttpClient.RetryWaitMax = 10 * time.Second
	retryablehttpClient.RetryWaitMin = 1 * time.Second
	retryablehttpClient.Logger = nil

	d := &Downloader{
		client:         retryablehttpClient.StandardClient(),
		URL:            config.Url,
		concurrency:    config.Concurrency,
		RootPath:       config.RootPath,
		CopyBufferSize: config.CopyBufferSize, // 1kb
		Resumable:      false,
		ShowProgress:   config.ShowProgress,
	}

	return d
}

func (d *Downloader) SetBaseFolder(folderName string) {
	d.RootPath = fmt.Sprintf("%s/%s", d.RootPath, folderName)
}

func (d *Downloader) checkPartExist() bool {
	_, err := os.Stat(fmt.Sprintf("%s/%s.part0", d.RootPath, d.Filename))
	return err == nil
}
func (d *Downloader) checkFileExist() bool {
	_, err := os.Stat(fmt.Sprintf("%s/%s", d.RootPath, d.Filename))
	return err == nil
}

func (d *Downloader) ensureRootPath() {
	_, err := os.Stat(fmt.Sprintf("%s/%s", d.RootPath, d.Filename))
	if os.IsNotExist(err) {
		os.MkdirAll(d.RootPath, os.ModePerm)
	}
}

func (d *Downloader) fetchMetadata() error {
	request, err := d.makeRequest("HEAD")
	if err != nil {
		return err
	}
	// Make a Head request to the URL to get the file size
	resp, err := d.do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusPartialContent && resp.Header.Get("Accept-Ranges") == "bytes" {
		d.Resumable = true
	}
	d.Size = resp.ContentLength
	if err := d.detectFilename(resp); err != nil {
		return err
	}
	return nil
}
func (d *Downloader) Resume() {
	d.resume = true
}
func (d *Downloader) Pause() {
	d.paused = true
	d.cancel()
}

// DetectFilename detects the filename from the response
func (d *Downloader) detectFilename(response *http.Response) error {
	path := response.Request.URL.Path
	tokens := strings.Split(path, "/")
	if len(tokens) > 0 {
		d.Filename = strcase.SnakeCase(tokens[len(tokens)-1])
		d.FilePath = d.FilePath + "/" + d.Filename
		return nil
	}
	return nil
}

// Download downloads the file partial
func (d *Downloader) Download() error {
	ctx, cancel := context.WithCancel(context.Background())
	d.context = ctx
	d.cancel = cancel
	// ensure the root path exists or create it.
	d.ensureRootPath()
	// fetch the metadata
	if err := d.fetchMetadata(); err != nil {
		return err
	}
	// if the file already exists, we rename it
	log.Println("filename", d.Filename)

	if d.checkFileExist() {
		return fmt.Errorf("file already exists")
	}

	if d.Resumable {
		return d.multiDownload()
	}
	return d.simpleDownload()
}

func (d *Downloader) makeRequest(method string) (*http.Request, error) {

	if d.URL == "" {
		return nil, errors.New("url is empty")
	}
	req, err := http.NewRequest(method, d.URL, nil)

	if err != nil {
		return nil, err
	}

	for k, v := range d.Header {
		req.Header.Add(k, v)
	}
	return req, nil
}

func (d *Downloader) makeRequestWithRange(start, end int64) (*http.Request, error) {
	req, err := d.makeRequest("GET")
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	return req, nil
}

func (d *Downloader) do(req *http.Request) (*http.Response, error) {
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (d *Downloader) multiDownload() error {
	// Calculate the segment size
	d.resume = d.checkPartExist()
	partSize := d.Size / int64(d.concurrency)
	startRange := int64(0)
	wg := sync.WaitGroup{}

	if d.ShowProgress {
		d.bar = progressbar.DefaultBytes(d.Size, "Downloading...")
	} else {
		d.bar = progressbar.DefaultBytesSilent(d.Size, "Downloading...")
	}

	// Create a channel to receive errors from goroutines
	errChan := make(chan error, d.concurrency)
	wg.Add(d.concurrency)
	for i := 0; i < d.concurrency; i++ {
		download := int64(0)
		if d.resume {
			path := d.getPartFilename(i)
			file, err := os.Open(fmt.Sprintf("%s/%s", d.RootPath, path))
			if err != nil {
				return err
			}
			defer file.Close()

			stat, err := file.Stat()

			if err != nil {
				return err
			}
			download = stat.Size()
			d.bar.Add64(download)
		}

		if i == d.concurrency {
			go d.partialDownload(startRange+download, d.Size, i, &wg, errChan)
		} else {
			go d.partialDownload(startRange+download, startRange+partSize, i, &wg, errChan)
		}
		startRange += partSize + 1
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	if !d.paused {
		err := d.mergeParts()
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *Downloader) partialDownload(start, end int64, partNumber int, wg *sync.WaitGroup, errChan chan error) {
	defer wg.Done()

	if start >= end {
		return
	}

	log.Println("Downloading part", partNumber)
	request, err := d.makeRequestWithRange(start, end)

	if err != nil {
		errChan <- err
		return
	}

	resp, err := d.do(request)

	if err != nil {
		errChan <- err
		return
	}
	defer resp.Body.Close()

	outputPath := d.RootPath + "/" + d.getPartFilename(partNumber)

	flags := os.O_CREATE | os.O_WRONLY

	f, err := os.OpenFile(outputPath, flags, 0666)

	if err != nil {
		errChan <- err
		return
	}

	defer f.Close()

	// copy to output file
	for {
		select {
		case <-d.context.Done():
			return
		default:
			_, err = io.CopyN(io.MultiWriter(f, d.bar), resp.Body, int64(d.CopyBufferSize))
			if err != nil {
				if err == io.EOF {
					return
				}
				errChan <- err
				return
			}
			d.Hook(resp, d.bar, err)
		}
	}

}

func (d *Downloader) simpleDownload() error {
	return nil
}

func (d *Downloader) getPartFilename(partNum int) string {
	return d.Filename + ".part" + strconv.Itoa(partNum)
}

func (d *Downloader) mergeParts() error {
	// Create the output file
	outputPath := d.RootPath + "/" + d.Filename
	destination, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY, 0666)

	if err != nil {
		return err
	}
	defer destination.Close()

	// Open each part file and copy to the destination file
	for i := 0; i < d.concurrency; i++ {
		partPath := d.RootPath + "/" + d.getPartFilename(i)
		part, err := os.OpenFile(partPath, os.O_RDONLY, 0666)
		if err != nil {
			return err
		}
		defer part.Close()

		_, err = io.Copy(destination, part)
		if err != nil {
			return err
		}
		os.Remove(partPath)
	}

	return nil
}
