package downloader

import (
	"net/http"

	"github.com/schollz/progressbar/v3"
)

type Hook func(resp *http.Response, progressbar *progressbar.ProgressBar, err error) error
