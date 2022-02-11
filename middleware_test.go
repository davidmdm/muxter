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
