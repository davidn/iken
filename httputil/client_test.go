package httputil

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errConn = errors.New("connection refused")

type reqBody struct {
	Name string `json:"name"`
}

type resBody struct {
	Name string `json:"name"`
}

type mockDoer struct {
	status   int
	body     string
	err      error
	gotReq   *http.Request
	sentResp *http.Response
}

func (m *mockDoer) Do(req *http.Request) (*http.Response, error) {
	m.gotReq = req

	if m.err != nil {
		return nil, m.err
	}

	m.sentResp = &http.Response{
		StatusCode: m.status,
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Header:     make(http.Header),
	}

	return m.sentResp, nil
}

func mergeHeaders(headers ...http.Header) http.Header {
	merged := http.Header{}

	for _, h := range headers {
		for k, v := range h {
			merged[k] = slices.Clone(v)
		}
	}

	return merged
}

func assertWantedErrors(t *testing.T, err error, wantErrIs error, wantErrAs any, gotResp *http.Response) {
	t.Helper()

	switch {
	case wantErrIs != nil:
		assert.ErrorIs(t, err, wantErrIs)
	case wantErrAs != nil:
		assert.ErrorAs(t, err, wantErrAs)
		if respErr, ok := wantErrAs.(*UnexpectedResponseError); ok {
			assert.Same(t, gotResp, respErr.Resp)
		}
	default:
		assert.NoError(t, err)
	}
}

func readAsString(t *testing.T, b io.ReadCloser) string {
	t.Helper()

	if b == nil {
		return ""
	}

	raw, err := io.ReadAll(b)
	require.NoError(t, err)

	return string(raw)
}

type call func(ctx context.Context, c DoClient, route string, headers http.Header, got *resBody) error

func TestClient(t *testing.T) {
	const route = "http://example.test/resource"

	acceptHeader := http.Header{Accept: {ApplicationJSON}}
	acceptAndContentTypeHeader := http.Header{Accept: {ApplicationJSON}, ContentType: {ApplicationJSON}}
	reqJSON := `{"name":"req"}` + "\n"

	endpoints := []struct {
		name            string
		method          string
		wantBody        string
		wantHeaders     http.Header
		supportsHeaders bool
		invoke          call
	}{
		{
			name: "Delete", method: http.MethodDelete, wantHeaders: http.Header{},
			invoke: func(ctx context.Context, c DoClient, route string, _ http.Header, _ *resBody) error {
				return Delete(ctx, c, route)
			},
		},
		{
			name: "Client.Delete", method: http.MethodDelete, wantHeaders: http.Header{},
			invoke: func(ctx context.Context, c DoClient, route string, _ http.Header, _ *resBody) error {
				return NewClient(c).Delete(ctx, route)
			},
		},
		{
			name: "DeleteWithHeaders", method: http.MethodDelete, wantHeaders: http.Header{}, supportsHeaders: true,
			invoke: func(ctx context.Context, c DoClient, route string, h http.Header, _ *resBody) error {
				return DeleteWithHeaders(ctx, c, route, h)
			},
		},
		{
			name: "Client.DeleteWithHeaders", method: http.MethodDelete, wantHeaders: http.Header{}, supportsHeaders: true,
			invoke: func(ctx context.Context, c DoClient, route string, h http.Header, _ *resBody) error {
				return NewClient(c).DeleteWithHeaders(ctx, route, h)
			},
		},

		{
			name: "GetJSON", method: http.MethodGet, wantHeaders: acceptHeader,
			invoke: func(ctx context.Context, c DoClient, route string, _ http.Header, got *resBody) error {
				return GetJSON(ctx, c, route, got)
			},
		},
		{
			name: "Client.GetJSON", method: http.MethodGet, wantHeaders: acceptHeader,
			invoke: func(ctx context.Context, c DoClient, route string, _ http.Header, got *resBody) error {
				return NewClient(c).GetJSON(ctx, route, got)
			},
		},
		{
			name: "GetJSONWithHeaders", method: http.MethodGet, wantHeaders: acceptHeader, supportsHeaders: true,
			invoke: func(ctx context.Context, c DoClient, route string, h http.Header, got *resBody) error {
				return GetJSONWithHeaders(ctx, c, route, h, got)
			},
		},
		{
			name: "Client.GetJSONWithHeaders", method: http.MethodGet, wantHeaders: acceptHeader, supportsHeaders: true,
			invoke: func(ctx context.Context, c DoClient, route string, h http.Header, got *resBody) error {
				return NewClient(c).GetJSONWithHeaders(ctx, route, h, got)
			},
		},

		{
			name: "PostJSON", method: http.MethodPost, wantBody: reqJSON, wantHeaders: acceptAndContentTypeHeader,
			invoke: func(ctx context.Context, c DoClient, route string, _ http.Header, got *resBody) error {
				return PostJSON(ctx, c, route, reqBody{Name: "req"}, got)
			},
		},
		{
			name: "Client.PostJSON", method: http.MethodPost, wantBody: reqJSON, wantHeaders: acceptAndContentTypeHeader,
			invoke: func(ctx context.Context, c DoClient, route string, _ http.Header, got *resBody) error {
				return NewClient(c).PostJSON(ctx, route, reqBody{Name: "req"}, got)
			},
		},
		{
			name: "PostJSONWithHeaders", method: http.MethodPost, wantBody: reqJSON, wantHeaders: acceptAndContentTypeHeader, supportsHeaders: true,
			invoke: func(ctx context.Context, c DoClient, route string, h http.Header, got *resBody) error {
				return PostJSONWithHeaders(ctx, c, route, h, reqBody{Name: "req"}, got)
			},
		},
		{
			name: "Client.PostJSONWithHeaders", method: http.MethodPost, wantBody: reqJSON, wantHeaders: acceptAndContentTypeHeader, supportsHeaders: true,
			invoke: func(ctx context.Context, c DoClient, route string, h http.Header, got *resBody) error {
				return NewClient(c).PostJSONWithHeaders(ctx, route, h, reqBody{Name: "req"}, got)
			},
		},

		{
			name: "PutJSON", method: http.MethodPut, wantBody: reqJSON, wantHeaders: acceptAndContentTypeHeader,
			invoke: func(ctx context.Context, c DoClient, route string, _ http.Header, got *resBody) error {
				return PutJSON(ctx, c, route, reqBody{Name: "req"}, got)
			},
		},
		{
			name: "Client.PutJSON", method: http.MethodPut, wantBody: reqJSON, wantHeaders: acceptAndContentTypeHeader,
			invoke: func(ctx context.Context, c DoClient, route string, _ http.Header, got *resBody) error {
				return NewClient(c).PutJSON(ctx, route, reqBody{Name: "req"}, got)
			},
		},
		{
			name: "PutJSONWithHeaders", method: http.MethodPut, wantBody: reqJSON, wantHeaders: acceptAndContentTypeHeader, supportsHeaders: true,
			invoke: func(ctx context.Context, c DoClient, route string, h http.Header, got *resBody) error {
				return PutJSONWithHeaders(ctx, c, route, h, reqBody{Name: "req"}, got)
			},
		},
		{
			name: "Client.PutJSONWithHeaders", method: http.MethodPut, wantBody: reqJSON, wantHeaders: acceptAndContentTypeHeader, supportsHeaders: true,
			invoke: func(ctx context.Context, c DoClient, route string, h http.Header, got *resBody) error {
				return NewClient(c).PutJSONWithHeaders(ctx, route, h, reqBody{Name: "req"}, got)
			},
		},

		{
			name: "PatchJSON", method: http.MethodPatch, wantBody: reqJSON, wantHeaders: acceptAndContentTypeHeader,
			invoke: func(ctx context.Context, c DoClient, route string, _ http.Header, got *resBody) error {
				return PatchJSON(ctx, c, route, reqBody{Name: "req"}, got)
			},
		},
		{
			name: "Client.PatchJSON", method: http.MethodPatch, wantBody: reqJSON, wantHeaders: acceptAndContentTypeHeader,
			invoke: func(ctx context.Context, c DoClient, route string, _ http.Header, got *resBody) error {
				return NewClient(c).PatchJSON(ctx, route, reqBody{Name: "req"}, got)
			},
		},
		{
			name: "PatchJSONWithHeaders", method: http.MethodPatch, wantBody: reqJSON, wantHeaders: acceptAndContentTypeHeader, supportsHeaders: true,
			invoke: func(ctx context.Context, c DoClient, route string, h http.Header, got *resBody) error {
				return PatchJSONWithHeaders(ctx, c, route, h, reqBody{Name: "req"}, got)
			},
		},
		{
			name: "Client.PatchJSONWithHeaders", method: http.MethodPatch, wantBody: reqJSON, wantHeaders: acceptAndContentTypeHeader, supportsHeaders: true,
			invoke: func(ctx context.Context, c DoClient, route string, h http.Header, got *resBody) error {
				return NewClient(c).PatchJSONWithHeaders(ctx, route, h, reqBody{Name: "req"}, got)
			},
		},
	}

	tests := []struct {
		name      string
		jsonOnly  bool // requires a decoded response (skipped for Delete)
		headers   http.Header
		status    int
		body      string
		doErr     error
		wantErrIs error
		wantErrAs any
		wantResp  resBody
	}{
		{name: "success 200", jsonOnly: true, status: http.StatusOK, body: `{"name":"foo"}`, wantResp: resBody{Name: "foo"}},
		{name: "no content 204", status: http.StatusNoContent},
		{name: "not found 404", status: http.StatusNotFound, wantErrIs: ErrNotFound},
		{name: "unauthorized 401", status: http.StatusUnauthorized, wantErrAs: &UnexpectedResponseError{}},
		{name: "redirect 302", status: http.StatusFound, body: "go", wantErrAs: &UnexpectedResponseError{}},
		{name: "server error 500", status: http.StatusInternalServerError, wantErrAs: &UnexpectedResponseError{}},
		{name: "informational 100", status: http.StatusContinue, wantErrAs: &UnexpectedResponseError{}},
		{name: "connection error", doErr: errConn, wantErrIs: errConn},
		{name: "invalid json", jsonOnly: true, status: http.StatusOK, body: "notjson", wantErrAs: new(*json.SyntaxError)},
		{name: "with header", headers: http.Header{"X-Test": {"test-value"}}, status: http.StatusOK},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			for _, tc := range tests {
				if tc.headers != nil && !ep.supportsHeaders {
					continue // endpoint cannot forward caller headers
				}
				if tc.jsonOnly && ep.method == http.MethodDelete {
					continue // Delete never decodes a response body
				}

				t.Run(tc.name, func(t *testing.T) {
					m := mockDoer{status: tc.status, body: tc.body, err: tc.doErr}

					var got resBody
					err := ep.invoke(context.Background(), &m, route, tc.headers, &got)

					assertWantedErrors(t, err, tc.wantErrIs, tc.wantErrAs, m.sentResp)
					require.NotNil(t, m.gotReq)
					assert.Equal(t, ep.method, m.gotReq.Method)
					assert.Equal(t, mustParseURL(route), m.gotReq.URL)
					assert.Equal(t, ep.wantBody, readAsString(t, m.gotReq.Body))
					assert.Equal(t, mergeHeaders(ep.wantHeaders, tc.headers), m.gotReq.Header)
					assert.Equal(t, tc.wantResp, got)
				})
			}
		})
	}
}
