package downloader

import "time"

type Config struct {
	Url            string
	ShowProgress   bool
	Concurrency    int
	RootPath       string
	CopyBufferSize int
	RetryWaitMin   time.Duration // Minimum time to wait
	RetryWaitMax   time.Duration // Maximum time to wait
	RetryMax       int           // Maximum number of retries
	Debug          bool
}
