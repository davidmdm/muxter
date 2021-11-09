package muxter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouting(t *testing.T) {

	mux := New()

	mux.Get("/api/v1/books", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		io.WriteString(rw, "books")
	}))

	mux.Get("/api/v1/books/", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		io.WriteString(rw, "books subtree")
	}))

	testCases := []struct {
		Name     string
		Method   string
		URL      string
		Expected string
	}{
		{
			Name:     "gets fixed route",
			Method:   "GET",
			URL:      "/api/v1/books",
			Expected: "books",
		},
		{
			Name:     "get subtree route",
			Method:   "GET",
			URL:      "/api/v1/books/cats_cradle",
			Expected: "books subtree",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {

			req := httptest.NewRequest(tc.Method, tc.URL, nil)
			rw := httptest.NewRecorder()

			mux.ServeHTTP(rw, req)

			if actual := rw.Body.String(); actual != tc.Expected {
				t.Fatalf("expected %q but got %q", tc.Expected, actual)
			}
		})
	}
}

func TestParamsMatching(t *testing.T) {
	mux := New()

	mux.Get("/resource/:id/key/:key", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		id, key := Param(r, "id"), Param(r, "key")
		io.WriteString(rw, id+" "+key)
	}))

	req := httptest.NewRequest("GET", "/resource/1/key/2", nil)
	rw := httptest.NewRecorder()

	mux.ServeHTTP(rw, req)

	if actual := rw.Body.String(); actual != "1 2" {
		t.Fatalf("expected %q but got %q", "1 2", actual)
	}
}
