package streamer

import (
	"io"
)

type Mux interface {
	Appender(label int) io.WriteCloser

	Reader(labels ...int) io.Reader
}
