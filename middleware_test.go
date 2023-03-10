package muxter

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecoverMiddleware(t *testing.T) {
	mux := New()

	panicMsg := "I can't even right now..."

	recoverMiddleware := Recover(func(recovered interface{}, w http.ResponseWriter, r *http.Request, c Context) {
		if recovered != panicMsg {
			t.Errorf("expected recovery value to be: '%v' but got: '%v'", panicMsg, recovered)
		}

		w.WriteHeader(500)
		io.WriteString(w, "calm down buddy.")
	})

	mux.Use(recoverMiddleware)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request, c Context) {
		panic(panicMsg)
	})

	w, r := httptest.NewRecorder(), httptest.NewRequest("GET", "/anywhere", nil)

	mux.ServeHTTP(w, r)

	if w.Code != 500 {
		t.Errorf("expected code to be 500 but got: %d", w.Code)
	}

	expectedPayload := "calm down buddy."
	if actual := w.Body.String(); actual != expectedPayload {
		t.Errorf("expected response body to be %q but got %q", expectedPayload, actual)
	}
}

func TestMethodMiddleware(t *testing.T) {
	t.Run("GET", func(t *testing.T) {
		mux := New()
		handler := new(HandlerMock)

		mux.Get("/", handler)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/path", nil)

		mux.ServeHTTP(w, r)

		if len(handler.ServeHTTPxCalls()) > 0 {
			t.Fatalf("expected handler to not be called but was")
		}

		if w.Code != 405 {
			t.Fatalf("expected code to be 405 but got: %d", w.Code)
		}
	})

	t.Run("custom method not allowed handler", func(t *testing.T) {
		mux := New()

		expectedBody := "Custom 405!"

		mux.SetMethodNotAllowedHandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
			w.WriteHeader(405)
			io.WriteString(w, expectedBody)
		})

		mux.PatchFunc("/", func(w http.ResponseWriter, r *http.Request, c Context) {})

		r := httptest.NewRequest("PUT", "/", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)

		if w.Code != 405 {
			t.Errorf("expected error code to be 405 but got %d", w.Code)
		}
		if body := w.Body.String(); body != expectedBody {
			t.Errorf("expected body to be %q but got %q", expectedBody, body)
		}
	})
}

func TestGetMiddleware(t *testing.T) {
	mux := New()

	mux.GetFunc(
		"/",
		func(w http.ResponseWriter, r *http.Request, c Context) {
			w.Header().Set("X-Custom", "value")
			io.WriteString(w, "hello!")
		},
	)

	// GET
	w, r := httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil)

	mux.ServeHTTP(w, r)

	if xcustom := w.Header().Get("X-Custom"); xcustom != "value" {
		t.Errorf("expected X-Custom header to equal %q but got %q", "value", xcustom)
	}

	expectedBody := "hello!"
	if body := w.Body.String(); body != expectedBody {
		t.Errorf("expected body to be %q but got %q", expectedBody, body)
	}

	// HEAD
	w, r = httptest.NewRecorder(), httptest.NewRequest("HEAD", "/", nil)

	mux.ServeHTTP(w, r)

	if length := w.Body.Len(); length != 0 {
		t.Errorf("expected length to be empty but got body of length %d", length)
	}

	if xcustom := w.Header().Get("X-Custom"); xcustom != "value" {
		t.Errorf("expected X-Custom header to equal %q but got %q", "value", xcustom)
	}

	if contentLength := w.Header().Get("Content-Length"); contentLength != "6" {
		t.Errorf("expected content-length to be 6 but got %q", contentLength)
	}

	// POST
	w, r = httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil)

	mux.ServeHTTP(w, r)

	if w.Code != 405 {
		t.Errorf("expected statusCode to be 405 but got %d", w.Code)
	}
}

func TestDecompress(t *testing.T) {
	mux := New()

	mux.HandleFunc(
		"/",
		func(w http.ResponseWriter, r *http.Request, c Context) {
			io.Copy(w, r.Body)
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

	w, r := httptest.NewRecorder(), httptest.NewRequest("POST", "/", gzipReader("hello world!"))
	r.Header.Set("Content-Encoding", "gzip")

	mux.ServeHTTP(w, r)

	expected := "hello world!"
	if actual := w.Body.String(); actual != expected {
		t.Errorf("expected body to be %q but got %q", expected, actual)
	}

	// Without Content-Encoding header should be skipped
	w, r = httptest.NewRecorder(), httptest.NewRequest("POST", "/", gzipReader("hello world!"))

	mux.ServeHTTP(w, r)

	expected = "\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\xcaH\xcd\xc9\xc9W(\xcf/\xcaIQ\x04\x00\x00\x00\xff\xff\x01\x00\x00\xff\xffm´\x03\f\x00\x00\x00"
	if actual := w.Body.String(); actual != expected {
		t.Errorf("expected body to be %q but got %q", expected, actual)
	}
}

func TestDecompressNoContent(t *testing.T) {
	mux := New()

	mux.HandleFunc(
		"/",
		func(w http.ResponseWriter, r *http.Request, c Context) {
			io.Copy(w, r.Body)
		},
		Decompress,
	)

	w, r := httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil)
	r.Header.Set("Content-Encoding", "gzip")

	mux.ServeHTTP(w, r)

	expected := ""
	if actual := w.Body.String(); actual != expected {
		t.Errorf("expected body to be %q but got %q", expected, actual)
	}
}

func TestCompress(t *testing.T) {
	mux := New()

	mux.HandleFunc(
		"/",
		func(w http.ResponseWriter, r *http.Request, c Context) {
			io.WriteString(w, "hello world!")
		},
		Compress(),
	)

	w, r := httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil)

	r.Header.Set("Accept-Encoding", "gzip")
	mux.ServeHTTP(w, r)

	expectedEncoding := "gzip"
	if actualEncoding := w.Header().Get("Content-Encoding"); actualEncoding != expectedEncoding {
		t.Errorf("expected content-encoding to be %q but got %q", expectedEncoding, actualEncoding)
	}

	expectedBody := "\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\xcaH\xcd\xc9\xc9W(\xcf/\xcaIQ\x04\x04\x00\x00\xff\xffm´\x03\f\x00\x00\x00"
	if actualBody := w.Body.String(); expectedBody != actualBody {
		t.Errorf("expected body to be %q but got %q", expectedBody, actualBody)
	}
}

func TestSkipped(t *testing.T) {
	mux := New()

	cors := Skip(DefaultCORS, func(r *http.Request) bool { return r.Header.Get("origin") == "" })

	mux.Use(cors)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request, c Context) {})

	w, r := httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil)

	mux.ServeHTTP(w, r)

	expectedAbsentHeaders := []string{"Access-Control-Allow-Origin"}
	for _, header := range expectedAbsentHeaders {
		if value := w.Header().Get(header); value != "" {
			t.Errorf("expected no value for header %q but got %q", header, value)
		}
	}

	w, r = httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Origin", "http://locahost.test")

	mux.ServeHTTP(w, r)

	expectedHeaders := map[string]string{
		"Access-Control-Allow-Origin": "*",
	}
	for header, expected := range expectedHeaders {
		if actual := w.Header().Get(header); actual != expected {
			t.Errorf("expected header %q to have value %q but got %q", header, expected, actual)
		}
	}
}
