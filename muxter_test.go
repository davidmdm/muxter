package muxter

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
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

			tc.InvokedHandler.ServeHTTPFunc = func(responseWriter http.ResponseWriter, request *http.Request) {
				for key, expected := range tc.ExpectedParams {
					if actual := Param(request, key); actual != expected {
						t.Errorf("expected parameter %q to be %q but got %q", key, expected, actual)
					}
				}

				params, _ := request.Context().Value(paramKey).(map[string]string)
				if len(params) != len(tc.ExpectedParams) {
					t.Errorf("expected %d path parameters but got: %d", len(tc.ExpectedParams), len(params))
				}
			}

			mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", tc.URL, nil))

			if count := len(tc.InvokedHandler.ServeHTTPCalls()); count != 1 {
				t.Fatalf("expected handler to be invoked once but was invoked %d times", count)
			}

		})
	}
}

func TestParams(t *testing.T) {
	mux := New()

	handler := new(HandlerMock)

	mux.Handle("/no/params", handler)
	mux.Handle("/multiple/:p1/params/:p2", handler)

	handler.ServeHTTPFunc = func(responseWriter http.ResponseWriter, request *http.Request) {
		params := Params(request)
		if params != nil {
			t.Errorf("expected params to be nil but got: %v", params)
		}
	}

	mux.ServeHTTP(nil, httptest.NewRequest("GET", "/no/params", nil))

	if callCount := len(handler.ServeHTTPCalls()); callCount != 1 {
		t.Errorf("expected handler to be called once but was called %d times", callCount)
	}

	handler.ServeHTTPFunc = func(responseWriter http.ResponseWriter, request *http.Request) {
		params := Params(request)

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

	if callCount := len(handler.ServeHTTPCalls()); callCount != 2 {
		t.Errorf("expected handler to be called twice but was called %d times", callCount)
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

func TestMatchTrailingSlash(t *testing.T) {
	regular := New()
	matcher := New(MatchTrailingSlash(true))

	handler := new(HandlerMock)
	regular.Handle("/path", handler)
	matcher.Handle("/path", handler)

	r := httptest.NewRequest("GET", "/path/", nil)
	rw := httptest.NewRecorder()

	regular.ServeHTTP(rw, r)

	if calls := len(handler.ServeHTTPCalls()); calls != 0 {
		t.Errorf("expected regular mux to not call handler but handler was called %d time(s)", calls)
	}

	matcher.ServeHTTP(rw, r)
	if calls := len(handler.ServeHTTPCalls()); calls != 1 {
		t.Errorf("expected matcher mux to invoke handler once but handler was called %d time(s)", calls)
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

func TestCustomNotFoundHandler(t *testing.T) {
	mux := New()

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/somewhere", nil)

	mux.ServeHTTP(rw, r)

	if rw.Code != 404 {
		t.Errorf("expected status code to be 404 but got %d", rw.Code)
	}

	expectedBody := http.StatusText(404) + "\n"
	if body := rw.Body.String(); body != expectedBody {
		t.Errorf("expected body to be %q but got %q", expectedBody, body)
	}

	mux.SetNotFoundHandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(404)
		io.WriteString(rw, "you are lost buddy!")
	})

	rw = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/somewhere", nil)

	mux.ServeHTTP(rw, r)

	if rw.Code != 404 {
		t.Errorf("expected status code to be 404 but got %d", rw.Code)
	}

	expectedBody = "you are lost buddy!"
	if body := rw.Body.String(); body != expectedBody {
		t.Errorf("expected body to be %q but got %q", expectedBody, body)
	}
}

func TestRegisterMux(t *testing.T) {
	child := New()

	child.HandleFunc(
		"/child",
		func(rw http.ResponseWriter, r *http.Request) {},
		func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				rw.Header().Add("x-header", "child")
				h.ServeHTTP(rw, r)
			})
		},
	)

	parent := New()

	parent.Use(
		func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				rw.Header().Add("x-header", "parent")
				h.ServeHTTP(rw, r)
			})
		},
	)

	parent.HandleFunc("/parent", func(rw http.ResponseWriter, r *http.Request) {})
	parent.RegisterMux("/registered", child)

	rw, r := httptest.NewRecorder(), httptest.NewRequest("GET", "/parent", nil)

	parent.ServeHTTP(rw, r)

	expectedHeaders := http.Header{
		"X-Header": {"parent"},
	}
	if !reflect.DeepEqual(expectedHeaders, rw.Header()) {
		t.Errorf("expected headers to be %+v but got %+v", expectedHeaders, rw.Header())
	}

	if len(rw.Header()) != len(expectedHeaders) {
		t.Errorf("expected response headers to have length %d but got %d", len(expectedHeaders), len(rw.Header()))
	}

	rw, r = httptest.NewRecorder(), httptest.NewRequest("GET", "/registered/child", nil)

	parent.ServeHTTP(rw, r)

	expectedHeaders = http.Header{
		"X-Header": {"parent", "child"},
	}
	if !reflect.DeepEqual(expectedHeaders, rw.Header()) {
		t.Errorf("expected headers to be %+v but got %+v", expectedHeaders, rw.Header())

	}

	if len(rw.Header()) != len(expectedHeaders) {
		t.Errorf("expected response headers to have length %d but got %d", len(expectedHeaders), len(rw.Header()))
	}
}

func TestRegisterMuxWithOptions(t *testing.T) {
	root := New()

	root.SetNotFoundHandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(404)
		io.WriteString(rw, "are you lost?")
	})

	api := New()

	api.HandleFunc("/crud", func(rw http.ResponseWriter, r *http.Request) {
		io.WriteString(rw, "API CRUD CALLED")
	})

	api.SetNotFoundHandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(404)
		io.WriteString(rw, "no matching api route")
	})

	assets := New(MatchTrailingSlash(true))

	assets.HandleFunc("/image.jpg", func(rw http.ResponseWriter, r *http.Request) {
		io.WriteString(rw, "IMAGE.JPG")
	})

	root.RegisterMux("/api", api)
	root.RegisterMux("/assets", assets)

	testcases := []struct {
		Name                 string
		Path                 string
		ResponseExpectations func(t *testing.T, r *httptest.ResponseRecorder)
	}{
		{
			Name: "calls api successfully",
			Path: "/api/crud",
			ResponseExpectations: func(t *testing.T, r *httptest.ResponseRecorder) {
				if r.Code != 200 {
					t.Errorf("expected response to be 200 but got %d", r.Code)
				}
				expected := "API CRUD CALLED"
				if actual := r.Body.String(); expected != actual {
					t.Errorf("expected body to be %q but got %q", expected, actual)
				}
			},
		},
		{
			Name: "api does not match trailing slash",
			Path: "/api/crud/",
			ResponseExpectations: func(t *testing.T, r *httptest.ResponseRecorder) {
				if r.Code != 404 {
					t.Errorf("expected 404 but got %d", r.Code)
				}
				expected := "no matching api route"
				if actual := r.Body.String(); actual != expected {
					t.Errorf("expected body to be %q but got %q", expected, actual)
				}
			},
		},
		{
			Name: "asset route matches",
			Path: "/assets/image.jpg",
			ResponseExpectations: func(t *testing.T, r *httptest.ResponseRecorder) {
				expected := "IMAGE.JPG"
				if actual := r.Body.String(); actual != expected {
					t.Errorf("expected body to be %q but got %q", expected, actual)
				}
			},
		},
		{
			Name: "asset route matches trailing slash",
			Path: "/assets/image.jpg/",
			ResponseExpectations: func(t *testing.T, r *httptest.ResponseRecorder) {
				expected := "IMAGE.JPG"
				if actual := r.Body.String(); actual != expected {
					t.Errorf("expected body to be %q but got %q", expected, actual)
				}
			},
		},
		{
			Name: "use root not found handler from assets",
			Path: "/assets/unknown.mp4",
			ResponseExpectations: func(t *testing.T, r *httptest.ResponseRecorder) {
				if r.Code != 404 {
					t.Errorf("expected code 404 but got %d", r.Code)
				}
				expected := "are you lost?"
				if actual := r.Body.String(); actual != expected {
					t.Errorf("expected body to be %q but got %q", expected, actual)
				}
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.Path, nil)
			rec := httptest.NewRecorder()
			root.ServeHTTP(rec, req)
			tc.ResponseExpectations(t, rec)
		})
	}

}

func TestRegisterMuxParams(t *testing.T) {
	child := New()

	expectedParams := map[string]string{
		"rootID":  "1",
		"childID": "2",
	}

	handler := &HandlerMock{
		ServeHTTPFunc: func(responseWriter http.ResponseWriter, request *http.Request) {
			for key, expected := range expectedParams {
				if actual := Param(request, key); actual != expected {
					t.Errorf("expected param %q to be %q but got %q", key, expected, actual)
				}
			}
		},
	}

	child.Handle("/child/:childID", handler)

	root := New()

	root.RegisterMux("/root/:rootID", child)

	rw, r := httptest.NewRecorder(), httptest.NewRequest("GET", "/root/1/child/2", nil)

	root.ServeHTTP(rw, r)

	if count := len(handler.ServeHTTPCalls()); count != 1 {
		t.Fatalf("expected handler to be called once but was called %d times", count)
	}
}

func TestRecoverMiddleware(t *testing.T) {

	mux := New()

	panicMsg := "I can't even right now..."

	recoverMiddleware := Recover(func(recovered interface{}, rw http.ResponseWriter, r *http.Request) {
		if recovered != panicMsg {
			t.Errorf("expected recovery value to be: '%v' but got: '%v'", panicMsg, recovered)
		}

		rw.WriteHeader(500)
		io.WriteString(rw, "calm down buddy.")
	})

	mux.Use(recoverMiddleware)

	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		panic(panicMsg)
	})

	rw, r := httptest.NewRecorder(), httptest.NewRequest("GET", "/anywhere", nil)

	mux.ServeHTTP(rw, r)

	if rw.Code != 500 {
		t.Errorf("expected code to be 500 but got: %d", rw.Code)
	}

	expectedPayload := "calm down buddy."
	if actual := rw.Body.String(); actual != expectedPayload {
		t.Errorf("expected response body to be %q but got %q", expectedPayload, actual)
	}
}

func TestGetMiddleware(t *testing.T) {
	mux := New()

	mux.HandleFunc(
		"/",
		func(rw http.ResponseWriter, r *http.Request) {
			rw.Header().Set("X-Custom", "value")
			io.WriteString(rw, "hello!")
		},
		GET,
	)

	// GET
	rw, r := httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil)

	mux.ServeHTTP(rw, r)

	if xcustom := rw.Header().Get("X-Custom"); xcustom != "value" {
		t.Errorf("expected X-Custom header to equal %q but got %q", "value", xcustom)
	}

	expectedBody := "hello!"
	if body := rw.Body.String(); body != expectedBody {
		t.Errorf("expected body to be %q but got %q", expectedBody, body)
	}

	// HEAD
	rw, r = httptest.NewRecorder(), httptest.NewRequest("HEAD", "/", nil)

	mux.ServeHTTP(rw, r)

	if length := rw.Body.Len(); length != 0 {
		t.Errorf("expected length to be empty but got body of length %d", length)
	}

	if xcustom := rw.Header().Get("X-Custom"); xcustom != "value" {
		t.Errorf("expected X-Custom header to equal %q but got %q", "value", xcustom)
	}

	if contentLength := rw.Header().Get("Content-Length"); contentLength != "6" {
		t.Errorf("expected content-length to be 6 but got %q", contentLength)
	}

	// POST
	rw, r = httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil)

	mux.ServeHTTP(rw, r)

	if rw.Code != 405 {
		t.Errorf("expected statusCode to be 405 but got %d", rw.Code)
	}
}

func TestMethodHandler(t *testing.T) {
	mux := New()

	methodHandler := new(MethodHandler)
	*methodHandler = MakeMethodHandler(
		map[string]http.Handler{
			"get": http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				io.WriteString(rw, "GET")
			}),
			"post": http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				io.WriteString(rw, "POST")
			}),
		},
		nil,
	)

	mux.Handle(
		"/methods",
		methodHandler,
	)

	// GET
	rw, r := httptest.NewRecorder(), httptest.NewRequest("get", "/methods", nil)

	mux.ServeHTTP(rw, r)

	expectedBody := "GET"
	if body := rw.Body.String(); body != expectedBody {
		t.Errorf("expected body to be %q but got %q", expectedBody, body)
	}

	// POST
	rw, r = httptest.NewRecorder(), httptest.NewRequest("POST", "/methods", nil)

	mux.ServeHTTP(rw, r)

	expectedBody = "POST"
	if body := rw.Body.String(); body != expectedBody {
		t.Errorf("expected body to be %q but got %q", expectedBody, body)
	}

	// PUT NOT FOUND
	rw, r = httptest.NewRecorder(), httptest.NewRequest("PUT", "/methods", nil)

	mux.ServeHTTP(rw, r)

	expectedBody = "Method Not Allowed\n"
	if body := rw.Body.String(); body != expectedBody {
		t.Errorf("expected body to be %q but got %q", expectedBody, body)
	}

	if rw.Code != 405 {
		t.Errorf("expected statusCode to be 405 but got %d", rw.Code)
	}

	methodHandler.methodNotAllowedHandler = http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(405)
		io.WriteString(rw, "YO YO YO NO")
	})

	// PUT NOT FOUND
	rw, r = httptest.NewRecorder(), httptest.NewRequest("PUT", "/methods", nil)

	mux.ServeHTTP(rw, r)

	expectedBody = "YO YO YO NO"
	if body := rw.Body.String(); body != expectedBody {
		t.Errorf("expected body to be %q but got %q", expectedBody, body)
	}

	if rw.Code != 405 {
		t.Errorf("expected statusCode to be 405 but got %d", rw.Code)
	}
}

func TestDecompress(t *testing.T) {
	mux := New()

	mux.HandleFunc(
		"/",
		func(rw http.ResponseWriter, r *http.Request) {
			io.Copy(rw, r.Body)
		},
		Decompress,
	)

	gzipReader := func(value string) io.Reader {
		buf := new(bytes.Buffer)
		gw := gzip.NewWriter(buf)

		io.WriteString(gw, "hello world!")
		gw.Flush()
		gw.Close()

		return buf
	}

	rw, r := httptest.NewRecorder(), httptest.NewRequest("POST", "/", gzipReader("hello world!"))
	r.Header.Set("Content-Encoding", "gzip")

	mux.ServeHTTP(rw, r)

	expected := "hello world!"
	if actual := rw.Body.String(); actual != expected {
		t.Errorf("expected body to be %q but got %q", expected, actual)
	}

	// Without Content-Encoding header should be skipped
	rw, r = httptest.NewRecorder(), httptest.NewRequest("POST", "/", gzipReader("hello world!"))

	mux.ServeHTTP(rw, r)

	expected = "\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\xcaH\xcd\xc9\xc9W(\xcf/\xcaIQ\x04\x00\x00\x00\xff\xff\x01\x00\x00\xff\xffm´\x03\f\x00\x00\x00"
	if actual := rw.Body.String(); actual != expected {
		t.Errorf("expected body to be %q but got %q", expected, actual)
	}
}

func TestDecompressNoContent(t *testing.T) {
	mux := New()

	mux.HandleFunc(
		"/",
		func(rw http.ResponseWriter, r *http.Request) {
			io.Copy(rw, r.Body)
		},
		Decompress,
	)

	rw, r := httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil)
	r.Header.Set("Content-Encoding", "gzip")

	mux.ServeHTTP(rw, r)

	expected := ""
	if actual := rw.Body.String(); actual != expected {
		t.Errorf("expected body to be %q but got %q", expected, actual)
	}
}

func TestSkipped(t *testing.T) {
	mux := New()

	cors := Skip(DefaultCORS, func(r *http.Request) bool { return r.Header.Get("origin") == "" })

	mux.Use(cors)
	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {})

	rw, r := httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil)

	mux.ServeHTTP(rw, r)

	expectedAbsentHeaders := []string{"Access-Control-Allow-Origin"}
	for _, header := range expectedAbsentHeaders {
		if value := rw.Header().Get(header); value != "" {
			t.Errorf("expected no value for header %q but got %q", header, value)
		}
	}

	rw, r = httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Origin", "http://locahost.test")

	mux.ServeHTTP(rw, r)

	expectedHeaders := map[string]string{
		"Access-Control-Allow-Origin": "*",
	}
	for header, expected := range expectedHeaders {
		if actual := rw.Header().Get(header); actual != expected {
			t.Errorf("expected header %q to have value %q but got %q", header, expected, actual)
		}
	}
}
