package muxter

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkSTD(b *testing.B) {
	mux := http.NewServeMux()

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/some/deeply/nested/path/id", nil)

	mux.HandleFunc("/some/deeply/nested/path/id", func(rw http.ResponseWriter, r *http.Request) {})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(rw, r)
	}
}

func BenchmarkRouting(b *testing.B) {
	mux := New()

	mux.HandleFunc("/some/deeply/nested/path/id", func(rw http.ResponseWriter, r *http.Request) {})

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/some/deeply/nested/path/id", nil)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(rw, r)
	}
}

func BenchmarkRoutingParams(b *testing.B) {
	mux := New()

	mux.HandleFunc("/some/deeply/:nested/path/:id", func(rw http.ResponseWriter, r *http.Request) {})

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/some/deeply/nested/path/id", nil)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(rw, r)
	}
}
