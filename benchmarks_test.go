package muxter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	"github.com/labstack/echo/v4"
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

func BenchmarkRoutingParamsHttpRouter(b *testing.B) {
	mux := httprouter.New()

	mux.GET("/some/deeply/:nested/path/:id", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {})

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/some/deeply/nested/path/id", nil)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(rw, r)
	}
}

func BenchmarkRoutingParamsGin(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	mux := gin.New()

	mux.GET("/some/deeply/:nested/path/:id", func(ctx *gin.Context) {})

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/some/deeply/nested/path/id", nil)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(rw, r)
	}
}

func BenchmarkRoutingParamsEcho(b *testing.B) {
	mux := echo.New()

	mux.GET("/some/deeply/:nested/path/:id", func(c echo.Context) error { return nil })

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/some/deeply/nested/path/id", nil)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(rw, r)
	}
}

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

func BenchmarkRealistic(b *testing.B) {
	libs := map[string]func(routes []string) http.Handler{
		"muxter": func(routes []string) http.Handler {
			mux := New()
			for _, route := range routes {
				mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request, c Context) {})
			}
			return mux
		},
		"httprouter": func(routes []string) http.Handler {
			router := httprouter.New()
			for _, route := range routes {
				router.GET(route, func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {})
			}
			return router
		},
		"gin": func(routes []string) http.Handler {
			gin.SetMode(gin.ReleaseMode)
			engine := gin.New()
			for _, route := range routes {
				engine.GET(route, func(ctx *gin.Context) {})
			}
			return engine
		},
		"echo": func(routes []string) http.Handler {
			router := echo.New()
			for _, route := range routes {
				router.GET(route, func(c echo.Context) error { return nil })
			}
			return router
		},
	}

	cases := []struct {
		Name   string
		Routes []string
		Path   string
	}{
		{
			Name: "no params",
			Routes: []string{
				"/public/assets",
				"/app",
				"/api/v1/resource",
				"/api/v2/result",
				"/api/v2/results",
			},
			Path: "/api/v2/results",
		},
		{
			Name: "with params",
			Routes: []string{
				"/public/assets",
				"/app",
				"/api/:version",
				"/api/:version/resource",
				"/api/:version/result",
				"/api/:version/results/:id",
			},
			Path: "/api/v3/results/23",
		},
	}

	for lib, genHandler := range libs {
		b.Run(lib, func(b *testing.B) {
			for _, tc := range cases {
				b.Run(tc.Name, func(b *testing.B) {
					h := genHandler(tc.Routes)
					r := httptest.NewRequest("GET", tc.Path, nil)
					w := httptest.NewRecorder()

					b.ResetTimer()

					for i := 0; i < b.N; i++ {
						h.ServeHTTP(w, r)
					}
				})
			}
		})
	}
}
