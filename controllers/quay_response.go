package controllers

import "net/http"

// quayHTTPResponseMissing reports whether a Quay API *http.Response is safe to
// read. The Quay client may return (nil, err) when building/sending the request
// fails; callers must not dereference StatusCode until this is false.
func quayHTTPResponseMissing(resp *http.Response) bool {
	return resp == nil
}
