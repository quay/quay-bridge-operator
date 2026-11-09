package controllers

import (
	"net/http"
	"testing"
)

func TestQuayHTTPResponseMissing(t *testing.T) {
	cases := []struct {
		name string
		resp *http.Response
		want bool
	}{
		{
			name: "nil response is missing",
			resp: nil,
			want: true,
		},
		{
			name: "non-nil response is present",
			resp: &http.Response{StatusCode: 201},
			want: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := quayHTTPResponseMissing(c.resp); got != c.want {
				t.Fatalf("quayHTTPResponseMissing() = %v, want %v", got, c.want)
			}
		})
	}
}
