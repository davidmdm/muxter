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

func TestMethodMiddleware(t *testing.T) {
	mux := New()
	handler := new(HandlerMock)

	mux.Handle("/", handler, Method("GET"))

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/path", nil)

	mux.ServeHTTP(rw, r)

	if len(handler.ServeHTTPCalls()) > 0 {
		t.Fatalf("expected handler to not be called but was")
	}

	if rw.Code != 405 {
		t.Fatalf("expected code to be 405 but got: %d", rw.Code)
	}
}

func TestMiddlewareCompisition(t *testing.T) {
	var (
		m1 Middleware = func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				rw.Header().Set("m1", "m1")
				h.ServeHTTP(rw, r)
			})
		}
		m2 Middleware = func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				rw.Header().Set("m2", "m2")
				h.ServeHTTP(rw, r)
			})
		}
		m3 Middleware = func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				rw.Header().Set("m1", "m3")
				rw.Header().Set("m3", "m3")
				h.ServeHTTP(rw, r)
			})
		}
	)

	mux := New()
	handler := new(HandlerMock)

	mux.Handle("/", handler, m1, m2, m3)

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/path", nil)

	mux.ServeHTTP(rw, r)

	expectedHeaders := map[string]string{
		"m1": "m3",
		"m2": "m2",
		"m3": "m3",
	}

	for key, expected := range expectedHeaders {
		if actual := rw.Header().Get(key); actual != expected {
			t.Errorf("expected header %q to have value %q but got %q", key, expected, actual)
		}
	}
}

func TestUseMiddleware(t *testing.T) {
	mux := New()
	handler := new(HandlerMock)

	mux.Handle("/pre-use", handler)

	mux.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.Header().Set("x-middleware", "ok")
			h.ServeHTTP(rw, r)
		})
	})

	mux.Handle("/post-use", handler)

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/pre-use", nil)

	mux.ServeHTTP(rw, r)

	if xMiddleware := rw.Header().Get("x-middleware"); xMiddleware != "" {
		t.Errorf("expected middle to not be called on pre-use but got %q", xMiddleware)
	}

	rw = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/post-use", nil)

	mux.ServeHTTP(rw, r)

	if xMiddleware := rw.Header().Get("x-middleware"); xMiddleware != "ok" {
		t.Errorf("expected middle to be called on post-use and set x-middleware to %q but got %q", "ok", xMiddleware)
	}
}
