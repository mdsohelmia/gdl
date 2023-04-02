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

// Downloader is the main struct
type Downloader struct {
	originUrl      string
	filename       string
	filePath       string
	rootPath       string
	header         map[string]string
	copyBufferSize int
	client         *http.Client
	url            string
	//file size in bytes
	size int64
	//is resumable
	resumable bool
	//is paused
	paused bool
	//is resumed
	resume bool
	// concurrent downloads
	concurrency int
	// use to pause the download gracefully
	context      context.Context
	cancel       context.CancelFunc
	bar          *progressbar.ProgressBar
	Hook         Hook
	showProgress bool
	debug        bool
}

// NewDownloader creates a new Downloader
// with the given URL and file path.
// The concurrency parameter specifies the number of threads
func NewDownloader(config *Config) (*Downloader, error) {
	if config.Concurrency == 0 {
		config.Concurrency = runtime.NumCPU()
	}
	if config.RootPath == "" {
		config.RootPath = "downloads"
	}

	if config.CopyBufferSize == 0 {
		config.CopyBufferSize = 1024
	}
	if config.RetryMax == 0 {
		config.RetryMax = 10
	}

	if config.RetryWaitMax == 0 {
		config.RetryWaitMax = 10 * time.Second
	}

	if config.RetryWaitMin == 0 {
		config.RetryWaitMin = 1 * time.Second
	}

	retryablehttpClient := retryablehttp.NewClient()
	retryablehttpClient.RetryMax = config.RetryMax
	retryablehttpClient.RetryWaitMax = config.RetryWaitMax
	retryablehttpClient.RetryWaitMin = config.RetryWaitMin
	if config.Debug {
		retryablehttpClient.Logger = log.New(os.Stdout, "", log.LstdFlags)
	} else {
		retryablehttpClient.Logger = nil
	}

	d := &Downloader{
		client:         retryablehttpClient.StandardClient(),
		url:            config.Url,
		concurrency:    config.Concurrency,
		rootPath:       config.RootPath,
		copyBufferSize: config.CopyBufferSize, // 1kb
		resumable:      false,
		showProgress:   config.ShowProgress,
	}

	// fetch the metadata
	if err := d.fetchMetadata(); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *Downloader) SetBaseFolder(folderName string) {
	d.rootPath = fmt.Sprintf("%s/%s", d.rootPath, folderName)
}

func (d *Downloader) checkPartExist() bool {
	_, err := os.Stat(fmt.Sprintf("%s/%s.part0", d.rootPath, d.filename))
	return err == nil
}
func (d *Downloader) checkFileExist() bool {
	_, err := os.Stat(fmt.Sprintf("%s/%s", d.rootPath, d.filePath))
	return err == nil
}

func (d *Downloader) ensureRootPath() {
	_, err := os.Stat(fmt.Sprintf("%s/%s", d.rootPath, d.filename))
	if os.IsNotExist(err) {
		os.MkdirAll(d.rootPath, os.ModePerm)
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
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusPartialContent && resp.Header.Get("Accept-Ranges") == "bytes" {
		d.resumable = true
	}

	d.size = resp.ContentLength

	if err := d.detectFilename(resp); err != nil {
		return err
	}
	d.originUrl = resp.Request.URL.String()
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
		d.filename = strcase.SnakeCase(tokens[len(tokens)-1])
		d.filePath = d.filePath + "/" + d.filename
		return nil
	}
	return nil
}

// Download downloads the file partial
func (d *Downloader) Download() error {
	ctx, cancel := context.WithCancel(context.Background())
	d.context = ctx
	d.cancel = cancel

	if d.debug {
		log.Println("Number of concurrency:", d.concurrency)
	}
	// ensure the root path exists or create it.
	d.ensureRootPath()

	// if the file already exists, we rename it
	if d.debug {
		log.Println("filename", d.filename)
	}

	if d.checkFileExist() {
		return fmt.Errorf("file already exists")
	}

	if d.resumable {
		return d.multiDownload()
	}

	return d.simpleDownload()
}

func (d *Downloader) makeRequest(method string) (*http.Request, error) {

	if d.url == "" {
		return nil, errors.New("url is empty")
	}
	req, err := http.NewRequest(method, d.url, nil)

	if err != nil {
		return nil, err
	}

	for k, v := range d.header {
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
	partSize := d.size / int64(d.concurrency)
	startRange := int64(0)
	wg := sync.WaitGroup{}

	if d.showProgress {
		d.bar = progressbar.DefaultBytes(d.size, "Downloading...")
	} else {
		d.bar = progressbar.DefaultBytesSilent(d.size, "Downloading...")
	}

	// Create a channel to receive errors from goroutines
	errChan := make(chan error, d.concurrency)
	wg.Add(d.concurrency)
	for i := 0; i < d.concurrency; i++ {
		download := int64(0)
		if d.resume {
			path := d.getPartFilename(i)
			file, err := os.Open(fmt.Sprintf("%s/%s", d.rootPath, path))
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
			go d.partialDownload(startRange+download, d.size, i, &wg, errChan)
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
		err := d.merge()
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

	if d.debug {
		log.Println("Downloading part", partNumber)
	}

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

	outputPath := d.rootPath + "/" + d.getPartFilename(partNumber)

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
			_, err = io.CopyN(io.MultiWriter(f, d.bar), resp.Body, int64(d.copyBufferSize))
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
	if d.resume {
		return fmt.Errorf("resumable download is not supported for simple download")
	}

	request, err := d.makeRequest("GET")

	if err != nil {
		return err
	}

	resp, err := d.do(request)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	f, err := os.OpenFile(d.rootPath+"/"+d.filename, os.O_CREATE|os.O_WRONLY, 0666)

	if err != nil {
		return err
	}
	defer f.Close()

	d.bar = progressbar.DefaultBytes(d.size, "Downloading...")

	// copy to output file
	buffer := make([]byte, d.copyBufferSize)

	_, err = io.CopyBuffer(io.MultiWriter(f, d.bar), resp.Body, buffer)

	if err != nil {
		return err
	}

	return nil
}

func (d *Downloader) getPartFilename(partNum int) string {
	return d.filename + ".part" + strconv.Itoa(partNum)
}

func (d *Downloader) merge() error {
	// Create the output file
	outputPath := d.rootPath + "/" + d.filename
	destination, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY, 0666)

	if err != nil {
		return err
	}
	defer destination.Close()

	// Open each part file and copy to the destination file
	for i := 0; i < d.concurrency; i++ {
		partPath := d.rootPath + "/" + d.getPartFilename(i)
		part, err := os.OpenFile(partPath, os.O_RDONLY, 0666)
		if err != nil {
			return err
		}
		_, err = io.Copy(destination, part)
		if err != nil {
			return err
		}
		os.Remove(partPath)
		defer part.Close()
	}

	return nil
}

func (d *Downloader) GetFileSize() int64 {
	return d.size
}

func (d *Downloader) GetFilename() string {
	return d.filename
}
func (d *Downloader) GetUrl() string {
	return d.url
}

func (d *Downloader) SetUrl(url string) {
	d.url = url
}

func (d *Downloader) SetHeader(header map[string]string) {
	d.header = header
}

func (d *Downloader) SetConcurrency(concurrency int) {
	d.concurrency = concurrency
}

func (d *Downloader) SetCopyBufferSize(size int) {
	d.copyBufferSize = size
}

func (d *Downloader) SetRootPath(path string) {
	d.rootPath = path
}

func (d *Downloader) SetShowProgress(show bool) {
	d.showProgress = show
}

func (d *Downloader) GetPath() string {
	return d.rootPath + "/" + d.filename
}

func (d *Downloader) SetDebug(debug bool) {
	d.debug = debug
}

func (d *Downloader) SetHook(hook Hook) {
	d.Hook = hook
}

func (d *Downloader) GetOriginUrl() string {
	return d.originUrl
}
