package downloader

import (
	"github.com/cavaliergopher/grab/v3"
)

type Hook func(response *grab.Response)
