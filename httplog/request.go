package httplog

import (
	"bytes"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"github.com/bir/iken/httputil"
	"github.com/bir/iken/logctx"
)

// Reference: https://docs.datadoghq.com/logs/log_configuration/attributes_naming_convention/#http-requests
const (
	Duration            = "duration" // In nanoseconds
	HTTPStatusCode      = "http.status_code"
	HTTPMethod          = "http.method"
	HTTPURLDetailsPath  = "http.url_details.path"
	NetworkBytesRead    = "network.bytes_read"
	NetworkBytesWritten = "network.bytes_written"
	Operation           = "op"
	Request             = "request"
	RequestID           = "http.request_id"
	RequestHeaders      = "request.headers"
	RequestError        = "request.body_error"
	Response            = "response"
	TraceID             = "trace_id"
	UserID              = "usr.id"
)

// MaxBodyLog controls the maximum request/response body that can be logged.  Anything greater will be truncated.
var MaxBodyLog uint32 = 24 * 1024

// now is a utility used for automated testing (overriding the runtime clock).
var now = time.Now

// stackSkip defines the lines to skip in the stack logger - this is determined by the structure of this code.
const stackSkip = 3

type FnToLogLevel func(r *http.Request, status int) zerolog.Level

func StatusToLogLevel(_ *http.Request, status int) zerolog.Level {
	switch {
	case status >= http.StatusInternalServerError:
		return zerolog.ErrorLevel
	case status >= http.StatusBadRequest:
		return zerolog.WarnLevel
	default:
		return zerolog.InfoLevel
	}
}

// FnShouldLog given a request, return flags that control logging.
// logRequest will disable the entire request logging middleware, default is true.
// logRequestBody will log the body of the request, default is false.
// logResponseBody will log the body of the response, default is false.  This should be disabled for large or streaming
// results.
type FnShouldLog func(r *http.Request) (logRequest, logRequestBody, logResponseBody bool, toLogLevel FnToLogLevel)

func LogRequestBody(_ *http.Request) (bool, bool, bool, FnToLogLevel) {
	return true, true, false, StatusToLogLevel
}

func LogAll(_ *http.Request) (bool, bool, bool, FnToLogLevel) {
	return true, true, true, StatusToLogLevel
}

// RequestLogger logs optional data, as specified by the shouldLog func.
// NOTE: The zerolog context logger MUST be initialized prior to this handler invocation.   This is generally done by
// using the recover logger, or by using the zerolog/hlog.NewHandler directly.
func RequestLogger(shouldLog FnShouldLog) func(http.Handler) http.Handler { //nolint: funlen
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := now()

			var logRequest, logRequestBody, logResponse bool
			logRequest = true
			toLogLevel := StatusToLogLevel

			if shouldLog != nil {
				logRequest, logRequestBody, logResponse, toLogLevel = shouldLog(r)
			}

			requestID := r.Header.Get(httputil.RequestIDHeader)

			r = r.WithContext(logctx.SetID(r.Context(), requestID))

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

			zerolog.Ctx(r.Context()).UpdateContext(func(logContext zerolog.Context) zerolog.Context {
				if logRequestBody {
					logContext = logBody(logContext, r)
				}

				if requestID != "" {
					logContext = logContext.Str(RequestID, requestID)
				}

				return logContext.
					Str(HTTPMethod, r.Method).
					Str(HTTPURLDetailsPath, r.URL.Path).
					Interface(RequestHeaders, httputil.DumpHeader(r))
			})

			if next != nil {
				next.ServeHTTP(wrappedWriter, r)
			}

			status := wrappedWriter.Status()

			if logResponse {
				logctx.AddBytesToContext(r.Context(), Response, responseBuffer.Bytes(), MaxBodyLog)
			}

			zerolog.Ctx(r.Context()).WithLevel(toLogLevel(r, status)).
				Ctx(r.Context()).
				Int(HTTPStatusCode, status).
				Int(NetworkBytesWritten, wrappedWriter.BytesWritten()).
				Dur(Duration, now().Sub(start)).Msgf("%d %s %s", status, r.Method, r.URL)
		})
	}
}

func logBody(l zerolog.Context, r *http.Request) zerolog.Context {
	body, err := httputil.DumpBody(r)
	if err != nil {
		l = l.Str(RequestError, err.Error())
	} else {
		size := len(body)
		l = l.Int(NetworkBytesRead, size)

		l = logctx.AddBytes(l, Request, body, MaxBodyLog)
	}

	return l
}
