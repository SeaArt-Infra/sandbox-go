package core

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDecodeAPIErrorTreatsGoneAsNotFound(t *testing.T) {
	err := DecodeAPIError(&http.Response{
		StatusCode: http.StatusGone,
		Status:     "410 Gone",
		Body:       io.NopCloser(strings.NewReader(`{"error":"sandbox has been deleted"}`)),
	})
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if apiErr.Kind != APIErrorKindNotFound || apiErr.StatusCode != http.StatusGone {
		t.Fatalf("API error = %#v", apiErr)
	}
}
