package httputil

import (
	"fmt"
	"net/http"
)

type UnexpectedResponseError struct {
	Resp *http.Response
	URL  string
	Body []byte
}

func (e UnexpectedResponseError) Error() string {
	return fmt.Sprintf("url: %q, status: %d, body: %q: unexpected response status code", e.URL, e.Resp.StatusCode, e.Body)
}
