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

	mux.HandleFunc("/some/deeply/nested/path/id", func(rw http.ResponseWriter, r *http.Request, c Context) {})

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/some/deeply/nested/path/id", nil)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(rw, r)
	}
}

func BenchmarkStandardRouting(b *testing.B) {
	mux := New()

	mux.StandardHandle("/some/deeply/nested/path/id", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {}))

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/some/deeply/nested/path/id", nil)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(rw, r)
	}
}

func BenchmarkAdaptorNoContextRouting(b *testing.B) {
	mux := New()

	hander := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {})
	mux.Handle("/some/deeply/nested/path/id", Adaptor(hander, NoContext))

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/some/deeply/nested/path/id", nil)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(rw, r)
	}
}

func BenchmarkRoutingParams(b *testing.B) {
	mux := New()

	mux.HandleFunc("/some/deeply/:nested/path/:id", func(rw http.ResponseWriter, r *http.Request, c Context) {})

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/some/deeply/nested/path/id", nil)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(rw, r)
	}
}

// func BenchmarkRoutingParamsHttpRouter(b *testing.B) {
// 	mux := httprouter.New()

// 	mux.GET("/some/deeply/:nested/path/:id", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {})

// 	rw := httptest.NewRecorder()
// 	r := httptest.NewRequest("GET", "/some/deeply/nested/path/id", nil)

// 	b.ResetTimer()

// 	for i := 0; i < b.N; i++ {
// 		mux.ServeHTTP(rw, r)
// 	}
// }

func BenchmarkRoutingParamsNestedMuxes(b *testing.B) {
	child := New()
	child.HandleFunc("/path/:id", func(w http.ResponseWriter, r *http.Request, c Context) {})

	root := New()
	root.Handle("/some/deeply/:nested/", StripDepth(3, child))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/some/deeply/nested/path/id", nil)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		root.ServeHTTP(w, r)
	}
}
