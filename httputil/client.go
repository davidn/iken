package httputil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type DoClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type UnexpectedResponseError struct {
	Resp *http.Response
	URL  string
	Body []byte
}

func (e UnexpectedResponseError) Error() string {
	return fmt.Sprintf("url: %q, status: %d, body: %q: unexpected response status code", e.URL, e.Resp.StatusCode, e.Body)
}

type Client struct {
	c DoClient
}

func NewClient(c DoClient) *Client {
	return &Client{c: c}
}

func (c *Client) Delete(ctx context.Context, route string) error {
	return Delete(ctx, c.c, route)
}

func (c *Client) DeleteWithHeaders(ctx context.Context, route string, headers http.Header) error {
	return DeleteWithHeaders(ctx, c.c, route, headers)
}

func (c *Client) GetJSON(ctx context.Context, route string, response any) error {
	return GetJSON(ctx, c.c, route, response)
}

func (c *Client) GetJSONWithHeaders(ctx context.Context, route string, headers http.Header, response any) error {
	return GetJSONWithHeaders(ctx, c.c, route, headers, response)
}

func (c *Client) PostJSON(ctx context.Context, route string, request any, response any) error {
	return PostJSON(ctx, c.c, route, request, response)
}

func (c *Client) PostJSONWithHeaders(
	ctx context.Context, route string, headers http.Header, request any, response any,
) error {
	return PostJSONWithHeaders(ctx, c.c, route, headers, request, response)
}

func (c *Client) PutJSON(ctx context.Context, route string, request any, response any) error {
	return PutJSON(ctx, c.c, route, request, response)
}

func (c *Client) PutJSONWithHeaders(
	ctx context.Context, route string, headers http.Header, request any, response any,
) error {
	return PutJSONWithHeaders(ctx, c.c, route, headers, request, response)
}

func (c *Client) PatchJSON(ctx context.Context, route string, request any, response any) error {
	return PatchJSON(ctx, c.c, route, request, response)
}

func (c *Client) PatchJSONWithHeaders(
	ctx context.Context, route string, headers http.Header, request any, response any,
) error {
	return PatchJSONWithHeaders(ctx, c.c, route, headers, request, response)
}

// Delete Simple HTTP DELETE.
func Delete(ctx context.Context, client DoClient, route string) error {
	return DeleteWithHeaders(ctx, client, route, nil)
}

// DeleteWithHeaders Simple HTTP DELETE with optional headers.
func DeleteWithHeaders(ctx context.Context, client DoClient, route string, headers http.Header) error {
	req, err := NewRequest(ctx, http.MethodDelete, route, headers, nil)
	if err != nil {
		return err
	}

	_, err = Do(client, req)
	if err != nil {
		return err
	}

	return nil
}

// GetJSON Simple HTTP GET with an optional JSON response.
func GetJSON(ctx context.Context, client DoClient, route string, response any) error {
	return GetJSONWithHeaders(ctx, client, route, nil, response)
}

// GetJSONWithHeaders Simple HTTP GET with a optional headers and JSON request/response.
func GetJSONWithHeaders(ctx context.Context, client DoClient, route string, headers http.Header, response any) error {
	req, err := NewRequest(ctx, http.MethodGet, route, headers, nil)
	if err != nil {
		return err
	}

	req.Header.Add(Accept, ApplicationJSON)

	body, err := Do(client, req)
	if err != nil {
		return err
	}

	if len(body) > 0 && response != nil {
		err = json.Unmarshal(body, response)
		if err != nil {
			return fmt.Errorf("failed to decode JSON response: %w", err)
		}
	}

	return nil
}

// PostJSON Simple HTTP POST with an optional JSON request/response.
func PostJSON(ctx context.Context, client DoClient, route string, request any, response any) error {
	return MutateJSONWithHeaders(ctx, client, http.MethodPost, route, nil, request, response)
}

// PostJSONWithHeaders Simple HTTP POST with a optional headers and JSON request/response.
func PostJSONWithHeaders(
	ctx context.Context, client DoClient, route string, headers http.Header, request any, response any,
) error {
	return MutateJSONWithHeaders(ctx, client, http.MethodPost, route, headers, request, response)
}

// PutJSON Simple HTTP PUT with an optional JSON request/response.
func PutJSON(ctx context.Context, client DoClient, route string, request any, response any) error {
	return MutateJSONWithHeaders(ctx, client, http.MethodPut, route, nil, request, response)
}

// PutJSONWithHeaders Simple HTTP PUT with a optional headers and JSON request/response.
func PutJSONWithHeaders(
	ctx context.Context, client DoClient, route string, headers http.Header, request any, response any,
) error {
	return MutateJSONWithHeaders(ctx, client, http.MethodPut, route, headers, request, response)
}

// PatchJSON Simple HTTP PATCH with an optional JSON request/response.
func PatchJSON(ctx context.Context, client DoClient, route string, request any, response any) error {
	return MutateJSONWithHeaders(ctx, client, http.MethodPatch, route, nil, request, response)
}

// PatchJSONWithHeaders Simple HTTP PATCH with a optional headers and JSON request/response.
func PatchJSONWithHeaders(
	ctx context.Context, client DoClient, route string, headers http.Header, request any, response any,
) error {
	return MutateJSONWithHeaders(ctx, client, http.MethodPatch, route, headers, request, response)
}

// MutateJSON HTTP request with a body and optional JSON request/response.
func MutateJSON(ctx context.Context, client DoClient, method, route string, request any, response any) error {
	return MutateJSONWithHeaders(ctx, client, method, route, nil, request, response)
}

// MutateJSONWithHeaders HTTP request with a body and optional headers and JSON request/response.
func MutateJSONWithHeaders(
	ctx context.Context, client DoClient, method, route string, headers http.Header, request any, response any,
) error {
	b := new(bytes.Buffer)

	err := json.NewEncoder(b).Encode(request)
	if err != nil {
		return fmt.Errorf("failed to encode request as JSON: %w", err)
	}

	req, err := NewRequest(ctx, method, route, headers, b)
	if err != nil {
		return err
	}

	req.Header.Set(ContentType, ApplicationJSON)
	req.Header.Add(Accept, ApplicationJSON)

	body, err := Do(client, req)
	if err != nil {
		return err
	}

	if len(body) > 0 && response != nil {
		err = json.Unmarshal(body, response)
		if err != nil {
			return fmt.Errorf("failed to decode response as JSON: %w", err)
		}
	}

	return nil
}

func NewRequest(
	ctx context.Context, method string, route string, headers http.Header, body io.Reader,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, route, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, values := range headers {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}

	return req, nil
}

func Do(client DoClient, req *http.Request) ([]byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if !httpSuccess(resp.StatusCode) {
		err := UnexpectedResponseError{
			Resp: resp,
			Body: body,
		}
		if req.URL != nil {
			err.URL = req.URL.String()
		}

		return nil, err
	}

	return body, nil
}

func httpSuccess(status int) bool {
	return status/100 == 2
}
