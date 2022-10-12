package muxter

import (
	"io"
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

			tc.InvokedHandler.ServeHTTPxFunc = func(responseWriter http.ResponseWriter, request *http.Request, c Context) {
				for key, expected := range tc.ExpectedParams {
					if actual := c.Param(key); actual != expected {
						t.Errorf("expected parameter %q to be %q but got %q", key, expected, actual)
					}
				}

				params := c.Params()
				if len(params) != len(tc.ExpectedParams) {
					t.Errorf("expected %d path parameters but got: %d", len(tc.ExpectedParams), len(params))
				}
			}

			mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", tc.URL, nil))

			if count := len(tc.InvokedHandler.ServeHTTPxCalls()); count != 1 {
				t.Fatalf("expected handler to be invoked once but was invoked %d times", count)
			}
		})
	}
}

func TestSubdirHandlerOnParam(t *testing.T) {
	m := New()

	handler := new(HandlerMock)

	m.Handle("/api/", handler)
	m.HandleFunc("/api/context/:ctx/resource/:id", func(w http.ResponseWriter, r *http.Request, c Context) {})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/context/conf", nil)

	m.ServeHTTP(w, r)

	if actual := len(handler.ServeHTTPxCalls()); actual != 1 {
		t.Errorf("expected handler to be called once but was called %d time(s)", actual)
	}
}

func TestParams(t *testing.T) {
	mux := New()

	handler := new(HandlerMock)

	mux.Handle("/no/params", handler)
	mux.Handle("/multiple/:p1/params/:p2", handler)

	handler.ServeHTTPxFunc = func(responseWriter http.ResponseWriter, request *http.Request, c Context) {
		params := Params(request)
		if len(params) != 0 {
			t.Errorf("expected params to be empty but got %v elements", len(params))
		}
	}

	mux.ServeHTTP(nil, httptest.NewRequest("GET", "/no/params", nil))

	if callCount := len(handler.ServeHTTPxCalls()); callCount != 1 {
		t.Errorf("expected handler to be called once but was called %d times", callCount)
	}

	handler.ServeHTTPxFunc = func(responseWriter http.ResponseWriter, request *http.Request, c Context) {
		params := c.Params()

		if len(params) != 2 {
			t.Errorf("expected params to have two entries but got %d", len(params))
		}

		expected := "A"
		if actual := params["p1"]; actual != expected {
			t.Errorf("expected params p1 to be %q but got %q", expected, actual)
		}

		expected = "B"
		if actual := params["p2"]; actual != expected {
			t.Errorf("expected params p2 tp be %q but got %q", expected, actual)
		}
	}

	mux.ServeHTTP(nil, httptest.NewRequest("GET", "/multiple/A/params/B", nil))

	if callCount := len(handler.ServeHTTPxCalls()); callCount != 2 {
		t.Errorf("expected handler to be called twice but was called %d times", callCount)
	}
}

func TestSubdirRedirect(t *testing.T) {
	mux := New()
	mux.HandleFunc("/dir/", func(w http.ResponseWriter, r *http.Request, c Context) {})

	w, r := httptest.NewRecorder(), httptest.NewRequest("POST", "/dir", nil)

	mux.ServeHTTP(w, r)

	if w.Code != 301 {
		t.Errorf("expected status code to be 301 but got %d", w.Code)
	}

	if location := w.Header().Get("Location"); location != "/dir/" {
		t.Errorf("expected location to be %q but got %q", "/dir/", location)
	}
}

func TestMatchTrailingSlash(t *testing.T) {
	t.Run("no params", func(t *testing.T) {
		regular := New()
		matcher := New(MatchTrailingSlash(true))

		handler := new(HandlerMock)
		regular.Handle("/path", handler)
		matcher.Handle("/path", handler)

		r := httptest.NewRequest("GET", "/path/", nil)
		w := httptest.NewRecorder()

		regular.ServeHTTP(w, r)

		if calls := len(handler.ServeHTTPxCalls()); calls != 0 {
			t.Errorf("expected regular mux to not call handler but handler was called %d time(s)", calls)
		}

		matcher.ServeHTTP(w, r)
		if calls := len(handler.ServeHTTPxCalls()); calls != 1 {
			t.Errorf("expected matcher mux to invoke handler once but handler was called %d time(s)", calls)
		}
	})

	t.Run("with params", func(t *testing.T) {
		regular := New()
		matcher := New(MatchTrailingSlash(true))

		handler := new(HandlerMock)
		regular.Handle("/path/:id", handler)
		matcher.Handle("/path/:id", handler)

		r := httptest.NewRequest("GET", "/path/value/", nil)
		w := httptest.NewRecorder()

		regular.ServeHTTP(w, r)

		if calls := len(handler.ServeHTTPxCalls()); calls != 0 {
			t.Errorf("expected regular mux to not call handler but handler was called %d time(s)", calls)
		}

		matcher.ServeHTTP(w, r)
		if calls := len(handler.ServeHTTPxCalls()); calls != 1 {
			t.Errorf("expected matcher mux to invoke handler once but handler was called %d time(s)", calls)
		}
	})
}

func TestMiddlewareCompisition(t *testing.T) {
	var (
		m1 Middleware = func(h Handler) Handler {
			return HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
				w.Header().Set("m1", "m1")
				h.ServeHTTPx(w, r, c)
			})
		}
		m2 Middleware = func(h Handler) Handler {
			return HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
				w.Header().Set("m2", "m2")
				h.ServeHTTPx(w, r, c)
			})
		}
		m3 Middleware = func(h Handler) Handler {
			return HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
				w.Header().Set("m1", "m3")
				w.Header().Set("m3", "m3")
				h.ServeHTTPx(w, r, c)
			})
		}
	)

	mux := New()
	handler := new(HandlerMock)

	mux.Handle("/", handler, m1, m2, m3)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/path", nil)

	mux.ServeHTTP(w, r)

	expectedHeaders := map[string]string{
		"m1": "m3",
		"m2": "m2",
		"m3": "m3",
	}

	for key, expected := range expectedHeaders {
		if actual := w.Header().Get(key); actual != expected {
			t.Errorf("expected header %q to have value %q but got %q", key, expected, actual)
		}
	}
}

func TestUseMiddleware(t *testing.T) {
	mux := New()
	handler := new(HandlerMock)

	mux.Handle("/pre-use", handler)

	mux.Use(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
			w.Header().Set("x-middleware", "ok")
			h.ServeHTTPx(w, r, c)
		})
	})

	mux.Handle("/post-use", handler)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/pre-use", nil)

	mux.ServeHTTP(w, r)

	if xMiddleware := w.Header().Get("x-middleware"); xMiddleware != "" {
		t.Errorf("expected middle to not be called on pre-use but got %q", xMiddleware)
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/post-use", nil)

	mux.ServeHTTP(w, r)

	if xMiddleware := w.Header().Get("x-middleware"); xMiddleware != "ok" {
		t.Errorf("expected middle to be called on post-use and set x-middleware to %q but got %q", "ok", xMiddleware)
	}
}

func TestCustomNotFoundHandler(t *testing.T) {
	mux := New()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/somewhere", nil)

	mux.ServeHTTP(w, r)

	if w.Code != 404 {
		t.Errorf("expected status code to be 404 but got %d", w.Code)
	}

	expectedBody := http.StatusText(404) + "\n"
	if body := w.Body.String(); body != expectedBody {
		t.Errorf("expected body to be %q but got %q", expectedBody, body)
	}

	mux.SetNotFoundHandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
		w.WriteHeader(404)
		io.WriteString(w, "you are lost buddy!")
	})

	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/somewhere", nil)

	mux.ServeHTTP(w, r)

	if w.Code != 404 {
		t.Errorf("expected status code to be 404 but got %d", w.Code)
	}

	expectedBody = "you are lost buddy!"
	if body := w.Body.String(); body != expectedBody {
		t.Errorf("expected body to be %q but got %q", expectedBody, body)
	}
}

func TestMethodHandler(t *testing.T) {
	t.Run("happy", func(t *testing.T) {
		mux := New()

		methodHandler := MethodHandler{
			GET: HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
				io.WriteString(w, "GET")
			}),
			POST: HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
				io.WriteString(w, "POST")
			}),
		}

		mux.Handle("/methods", methodHandler)

		// GET
		w, r := httptest.NewRecorder(), httptest.NewRequest("get", "/methods", nil)

		mux.ServeHTTP(w, r)

		expectedBody := "GET"
		if body := w.Body.String(); body != expectedBody {
			t.Errorf("expected body to be %q but got %q", expectedBody, body)
		}

		// POST
		w, r = httptest.NewRecorder(), httptest.NewRequest("POST", "/methods", nil)

		mux.ServeHTTP(w, r)

		expectedBody = "POST"
		if body := w.Body.String(); body != expectedBody {
			t.Errorf("expected body to be %q but got %q", expectedBody, body)
		}

		// PUT NOT FOUND
		w, r = httptest.NewRecorder(), httptest.NewRequest("PUT", "/methods", nil)

		mux.ServeHTTP(w, r)

		expectedBody = "Method Not Allowed\n"
		if body := w.Body.String(); body != expectedBody {
			t.Errorf("expected body to be %q but got %q", expectedBody, body)
		}

		if w.Code != 405 {
			t.Errorf("expected statusCode to be 405 but got %d", w.Code)
		}
	})

	t.Run("custom not found handler", func(t *testing.T) {
		mux := New()

		mux.Handle("/", MethodHandler{
			MethodNotAllowedHandler: HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
				w.WriteHeader(405)
				io.WriteString(w, "YO YO YO NO")
			}),
		})

		w, r := httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil)

		mux.ServeHTTP(w, r)

		expectedBody := "YO YO YO NO"
		if body := w.Body.String(); body != expectedBody {
			t.Errorf("expected body to be %q but got %q", expectedBody, body)
		}

		if w.Code != 405 {
			t.Errorf("expected statusCode to be 405 but got %d", w.Code)
		}
	})
}

func TestNestedMuxes(t *testing.T) {
	child := New()
	child.HandleFunc("/path/:id", func(w http.ResponseWriter, r *http.Request, c Context) {
		p := c.Params()
		if len(p) != 2 {
			t.Errorf("expected 2 params but got: %d", len(p))
		}
		if id := c.Param("id"); id != "id" {
			t.Errorf("expected id param to equal id but got: %s", id)
		}
		if nested := c.Param("nested"); nested != "nested" {
			t.Errorf("expected nested param to equal nested but got: %s", nested)
		}
	})

	root := New()
	root.Handle("/some/deeply/:nested/", StripDepth(3, child))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/some/deeply/nested/path/id", nil)

	root.ServeHTTP(w, r)

	if code := w.Code; code != 200 {
		t.Errorf("expected code 200 but got %d", code)
	}
}
