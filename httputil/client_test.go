package httputil_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bir/iken/httputil"
)

func TestUnexpectedResponseError_Error(t *testing.T) {
	err := httputil.UnexpectedResponseError{
		Resp: &http.Response{StatusCode: http.StatusNotFound},
		URL:  "http://example.com/foo",
		Body: []byte("not found"),
	}

	assert.Equal(t,
		`url: "http://example.com/foo", status: 404, body: "not found": unexpected response status code`,
		err.Error())
}
