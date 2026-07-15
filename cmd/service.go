package cmd

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/SeaArt-Infra/sandbox-go/core"
)

const (
	ConnectProtocolVersion = "1"
)

type Service struct {
	baseURL     *url.URL
	accessToken string
	httpClient  *http.Client
	userAgent   string
}

func NewService(baseURL, accessToken string) (*Service, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, ErrBaseURLEmpty
	}

	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, &url.Error{Op: "parse", URL: baseURL, Err: ErrInvalidBaseURL}
	}

	return &Service{
		baseURL:     parsed,
		accessToken: strings.TrimSpace(accessToken),
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		userAgent:   core.UserAgent("seainfra-sandbox-go-cmd"),
	}, nil
}

func (c *Service) BaseURL() string {
	return c.baseURL.String()
}

func (c *Service) newRequest(
	ctx context.Context,
	method, path string,
	query url.Values,
	body io.Reader,
	contentType string,
	accept string,
	opts *RequestOptions,
) (*http.Request, error) {
	endpoint, err := c.resolve(path, query)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}

	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("User-Agent", c.userAgent)
	if c.accessToken != "" {
		req.Header.Set("X-Access-Token", c.accessToken)
	}
	if opts != nil {
		for key, values := range opts.Headers {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}
	return req, nil
}

func (c *Service) doJSON(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	out any,
	contentType string,
	accept string,
	opts *RequestOptions,
	expectedStatus ...int,
) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	if contentType == "" && body != nil {
		contentType = "application/json"
	}
	if accept == "" {
		accept = "application/json"
	}

	resp, err := c.do(ctx, method, path, query, reader, contentType, accept, opts, expectedStatus...)
	if err != nil {
		return nil, err
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return resp, nil
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Service) do(
	ctx context.Context,
	method, path string,
	query url.Values,
	body io.Reader,
	contentType string,
	accept string,
	opts *RequestOptions,
	expectedStatus ...int,
) (*http.Response, error) {
	req, err := c.newRequest(ctx, method, path, query, body, contentType, accept, opts)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if statusAllowed(resp.StatusCode, expectedStatus) {
		return resp, nil
	}

	defer resp.Body.Close()
	return nil, core.DecodeAPIError(resp)
}

func (c *Service) resolve(path string, query url.Values) (string, error) {
	normalized := strings.TrimSpace(path)
	if normalized == "" {
		normalized = "/"
	}
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}

	base := *c.baseURL
	base.Path = joinBasePath(c.baseURL.Path, normalized)
	base.RawPath = ""
	base.RawQuery = ""
	base.Fragment = ""
	if len(query) > 0 {
		base.RawQuery = query.Encode()
	}
	return base.String(), nil
}

func statusAllowed(statusCode int, expected []int) bool {
	for _, code := range expected {
		if statusCode == code {
			return true
		}
	}
	return false
}

func withConnectRPC(opts *RequestOptions) *RequestOptions {
	return cloneOptionsWithHeaders(opts, http.Header{
		"Connect-Protocol-Version": []string{ConnectProtocolVersion},
	})
}

func withBasicUsername(opts *RequestOptions) *RequestOptions {
	if opts == nil || strings.TrimSpace(opts.Username) == "" {
		return opts
	}
	cloned := cloneOptionsWithHeaders(opts, nil)
	if cloned.Headers.Get("Authorization") == "" {
		cloned.Headers.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(opts.Username)+":")))
	}
	return cloned
}

func fileQuery(path string, opts *RequestOptions) (url.Values, error) {
	if strings.TrimSpace(path) == "" {
		return nil, ErrPathEmpty
	}
	query := make(url.Values)
	query.Set("path", path)
	if opts == nil {
		return query, nil
	}
	if username := strings.TrimSpace(opts.Username); username != "" {
		query.Set("username", username)
	}
	if signature := strings.TrimSpace(opts.Signature); signature != "" {
		query.Set("signature", signature)
	}
	if opts.SignatureExpiration != nil {
		query.Set("signature_expiration", strconv.FormatInt(*opts.SignatureExpiration, 10))
	}
	return query, nil
}

func cloneOptionsWithHeaders(opts *RequestOptions, extra http.Header) *RequestOptions {
	if opts == nil && extra == nil {
		return nil
	}

	cloned := &RequestOptions{}
	if opts != nil {
		*cloned = *opts
	}
	cloned.Headers = make(http.Header)
	if opts != nil {
		for key, values := range opts.Headers {
			copied := append([]string(nil), values...)
			cloned.Headers[key] = copied
		}
	}
	for key, values := range extra {
		for _, value := range values {
			cloned.Headers.Add(key, value)
		}
	}
	return cloned
}

func queryFromOptions(opts *RequestOptions) url.Values {
	query := make(url.Values)
	if opts == nil {
		return query
	}
	if username := strings.TrimSpace(opts.Username); username != "" {
		query.Set("username", username)
	}
	if signature := strings.TrimSpace(opts.Signature); signature != "" {
		query.Set("signature", signature)
	}
	if opts.SignatureExpiration != nil {
		query.Set("signature_expiration", strconv.FormatInt(*opts.SignatureExpiration, 10))
	}
	return query
}

func joinBasePath(basePath, reqPath string) string {
	trimmedBase := strings.TrimRight(strings.TrimSpace(basePath), "/")
	trimmedReq := strings.TrimLeft(strings.TrimSpace(reqPath), "/")

	switch {
	case trimmedBase == "" && trimmedReq == "":
		return "/"
	case trimmedBase == "":
		return "/" + trimmedReq
	case trimmedReq == "":
		return trimmedBase
	default:
		return trimmedBase + "/" + trimmedReq
	}
}

func gzipBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeMultipart(parts []MultipartFile) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for _, part := range parts {
		fieldName := strings.TrimSpace(part.FieldName)
		if fieldName == "" {
			fieldName = "file"
		}

		var w io.Writer
		if strings.TrimSpace(part.FileName) != "" {
			header := make(textproto.MIMEHeader)
			header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, fieldName, part.FileName))
			if strings.TrimSpace(part.ContentType) != "" {
				header.Set("Content-Type", part.ContentType)
			}
			var err error
			w, err = writer.CreatePart(header)
			if err != nil {
				_ = writer.Close()
				return nil, "", err
			}
		} else {
			var err error
			w, err = writer.CreateFormField(fieldName)
			if err != nil {
				_ = writer.Close()
				return nil, "", err
			}
		}

		if _, err := w.Write(part.Data); err != nil {
			_ = writer.Close()
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return &buf, writer.FormDataContentType(), nil
}
