package httplog

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"

	"github.com/bir/iken/httputil"
	"github.com/bir/iken/logctx"
)

// Reference: https://docs.datadoghq.com/logs/log_configuration/attributes_naming_convention/#http-requests
const (
	Duration            = "duration" // In nanoseconds
	HTTPStatusCode      = "http.status_code"
	HTTPMethod          = "http.method"
	HTTPURLDetailsPath  = "http.url_details.path"
	NetworkBytesWritten = "network.bytes_written"
	Operation           = "op"
	Request             = "request"
	RequestID           = "http.request_id"
	RequestHeaders      = "http.headers"
	RequestSize         = "network.bytes_read"
	Response            = "response"
	TraceID             = "trace_id"
	UserID              = "usr.id"
	Stack               = "error.stack"
)

// MaxRequestBodyLog controls the maximum request body that can be logged.  Anything greater will be truncated.
var MaxRequestBodyLog = 24 * 1024

// now is a utility used for automated testing (overriding the runtime clock).
var now = time.Now

// FnShouldLog given a request, return flags that control logging.
// logRequest will disable the entire request logging middleware, default is true.
// logRequestBody will log the body of the request, default is false.
// logResponseBody will log the body of the response, default is false.  This should be disabled for large or streaming
// results.
type FnShouldLog func(r *http.Request) (logRequest, logRequestBody, logResponseBody bool)

// RequestLogger returns a handler that call initializes Op in the context, and logs each request.
func RequestLogger(next http.Handler, shouldLog FnShouldLog) http.Handler { //nolint: funlen
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := now()

		var logRequest, logRequestBody, logResponse bool
		logRequest = true

		if shouldLog != nil {
			logRequest, logRequestBody, logResponse = shouldLog(r)
		}

		if !logRequest {
			if next != nil {
				next.ServeHTTP(w, r)
			}

			return
		}

		var responseBuffer *bytes.Buffer
		wrappedWriter := httputil.WrapWriter(w)

		if logResponse {
			responseBuffer = bytes.NewBuffer(nil)
			wrappedWriter.Tee(responseBuffer)
		}

		l := hlog.FromRequest(r).With().
			Str(HTTPMethod, r.Method).
			Str(HTTPURLDetailsPath, r.URL.Path).
			Interface(RequestHeaders, httputil.DumpHeader(r))

		if logRequestBody {
			body, err := httputil.DumpBody(r)
			if err != nil {
				panic(err) // Ignore coverage
			}

			size := len(body)
			l = l.Int(RequestSize, size)

			if size > MaxRequestBodyLog {
				l = l.Bytes(Request, body[:MaxRequestBodyLog])
			} else {
				l = l.Bytes(Request, body)
			}
		}

		ctx := logctx.SetOp(r.Context(), fmt.Sprintf("[%s] %s", r.Method, r.URL))
		if next != nil {
			next.ServeHTTP(wrappedWriter, r.WithContext(ctx))
		}

		op := logctx.GetOp(ctx)
		status := wrappedWriter.Status()

		l = l.
			Str(Operation, op).
			Int(HTTPStatusCode, status).
			Int(NetworkBytesWritten, wrappedWriter.BytesWritten()).
			Dur(Duration, now().Sub(start))

		if logResponse {
			l = l.Bytes(Response, responseBuffer.Bytes())
		}

		logger := l.Logger()
		var event *zerolog.Event

		switch {
		case status >= http.StatusInternalServerError:
			event = logger.Error()
		case status >= http.StatusBadRequest:
			event = logger.Warn()
		default:
			event = logger.Info()
		}

		if op != "" {
			event.Msg(op)
		} else {
			event.Msgf("[%s] %s", r.Method, r.URL)
		}
	})
}
