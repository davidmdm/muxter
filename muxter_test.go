package muxter

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouting(t *testing.T) {
	mux := New()

	defaultHandler := new(HandlerMock)
	subdirHandler := new(HandlerMock)

	resetHandlers := func() {
		*defaultHandler = HandlerMock{}
		*subdirHandler = HandlerMock{}
	}

	mux.Handle("/api/v1/books", defaultHandler)
	mux.Handle("/api/v1/books/", subdirHandler)
	mux.Handle("/resource/:resourceID/subresource/:subID", defaultHandler)

	testCases := []struct {
		Name           string
		URL            string
		InvokedHandler *HandlerMock
		ExpectedParams map[string]string
	}{
		{
			Name:           "gets fixed route",
			URL:            "/api/v1/books",
			InvokedHandler: defaultHandler,
		},
		{
			Name:           "get subtree route",
			URL:            "/api/v1/books/cats_cradle",
			InvokedHandler: subdirHandler,
		},
		{
			Name:           "match params",
			URL:            "/resource/my_resource/subresource/my_sub",
			InvokedHandler: defaultHandler,
			ExpectedParams: map[string]string{
				"resourceID": "my_resource",
				"subID":      "my_sub",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			resetHandlers()

			mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", tc.URL, nil))

			if count := len(tc.InvokedHandler.ServeHTTPCalls()); count != 1 {
				t.Fatalf("expected handler to be invoked once but was invoked %d times", count)
			}

			req := tc.InvokedHandler.ServeHTTPCalls()[0].Request
			for key, expected := range tc.ExpectedParams {
				if actual := Param(req, key); actual != expected {
					t.Errorf("expected parameter %q to be %q but got %q", key, expected, actual)
				}
			}

			params, _ := req.Context().Value(paramKey).(map[string]string)
			if len(params) != len(tc.ExpectedParams) {
				t.Errorf("expected %d path parameters but got: %d", len(tc.ExpectedParams), len(params))
			}
		})
	}
}

func TestSubdirRedirect(t *testing.T) {
	mux := New()
	mux.HandleFunc("/dir/", func(rw http.ResponseWriter, r *http.Request) {})

	rw, r := httptest.NewRecorder(), httptest.NewRequest("POST", "/dir", nil)

	mux.ServeHTTP(rw, r)

	if rw.Code != 301 {
		t.Errorf("expected status code to be 301 but got %d", rw.Code)
	}

	if location := rw.Header().Get("Location"); location != "/dir/" {
		t.Errorf("expected location to be %q but got %q", "/dir/", location)
	}
}

func TestMiddleware(t *testing.T) {

}
