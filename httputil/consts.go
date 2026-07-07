package httputil

var (
	// ContentType and Accept header value.
	ContentType = "Content-Type"
	Accept      = "Accept"
	// ApplicationJSON content-type.
	ApplicationJSON = "application/json"
	// TextHTML content-type.
	TextHTML = "text/html"
	// TextPlain content-type.
	TextPlain = "text/plain; charset=utf-8"
)

type Error string

func (e Error) Error() string {
	return string(e)
}

const (
	// ErrNotFound represents failure when authenticating a request.
	ErrNotFound = Error("not found")
)
