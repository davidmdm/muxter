# muxter

## What is muxter?

Muxter is a HTTP request multiplexer.

The main inspiration behind muxter is `httprouter` by julienschmidt but with an API and routing strategy that more closely resembles the standard library.

## Why muxter?

The go community generally likes to keep dependencies to a minimum. I do too.
Every week a new gopher will ask what dependency they should use for web development.
Should they use gorilla / gin / echo / httprouter / standard lib?

What is the answer? The standard library.

Truth be told, I agree whole heartedly.
I want to use the net/http ServeMux for my servers.
However it does not match path params and that makes it just not viable to use all the time.

So why muxter?

- It aims to route and work exactly as the standard library's http.ServeMux.
- It matches path params.
- It supports middleware.

And most importantly it does not seek to do or become anything more,
or become a framework. It is simply a routing library with some common middlewares.

### Caveats / Differences with the standard library

Are there differences with the standard library?

Small ones.

- muxter.HandlerFunc signature has a muxter.Context as a third parameter, similiar to httprouter's param argument.
- It does not parse or handle hosts/ports like that standard library.
- mux.Handle accepts variadic middlewares.

### Why diverge from http.Handler / http.HandlerFunc signature?

In the first versions of muxter the router simply registered http.Handlers and put params and pattern matching within the (\*http.Request).Context, however this operation necessarily must allocate a new request and context, and although performance would remain comparable to the standard library, it could in no way compete with other high-performance routers.

This is why most routers have their own signature (echo, gin, httprouter, and so on). By extending the Handler Signature you avoid storing values within the request's context, and avoid unnecessary allocations.

With muxter, I wanted to stay as close to the standard library as possible and not absorb the \*http.Request and http.ResponseWriter values into a single object like some other libraries have done. Therefore the muxter HandlerFunc signature is simply:

```go
type HandlerFunc func (w http.ResponseWriter, r *http.Request, c muxter.Context)
```

Where context allows you to get any matched Params, and the matched route pattern.

Standard http.Handlers can be adapted to be used with Muxter using either the Adapter or StandardHandle APIs at the cost of injecting context into the request and allocating:

```go
var handler http.Handler

mux.Handle("/", muxter.Adaptor(handler))
mux.StandardHandle("/", handler)
```

The main difference between these APIs is that with the Adaptor API you can opt out of injecting the context, saving the allocation.

```go
mux.Handle("/", muxter.Adaptor(handler, muxter.NoContext))
```

### Differences from httprouter and other common frameworks

Muxter routes in the same way that the standard library's http.ServeMux does. The concept of rooted subtrees is carried over, and longest path matching still holds.

Another big difference from `httprouter` (muxter's main source of inspiration other than that standard library) and other common frameworks like `gin` is that you can register static and wildcard segments for similar paths and the static route shall be preferred, for example:

```go
mux := muxter.New()

mux.HandleFunc("/user/:id", HandleUserID)
mux.HandleFunc("/user/me", HandleMe)
```

Although the first route will match a request with incoming path `/user/me`, the second handler will be chosen as muxter prefers to walk static segments over doing wildcard matching. One must keep in mind that muxter follows static segments and does not try wildcards if it does not find a handler. Consider the following example:

```go
mux := muxter.New()

mux.HandleFunc("/user/:id", HandleUserID)
mux.HandleFunc("/user/:id/posts", HandleUserPosts)
mux.HandleFunc("/user/me", HandleMe)
```

A request with path `/user/me/posts` will result in a 404 because paths that start with `/user/me` will match against `/user/me` over `/user/:id`.

### Performance

Simple micro-benchmarks show muxter to be similar in routing performance as other more mainstream routers like `httprouter`, `echo` and `gin`.

For a simple benchmark that tests routing routing performance for paths with two wildcards gives these results:

`muxter benchmark code`

```go
func BenchmarkRoutingParamsMuxter(b *testing.B) {
	mux := muxter.New()

	mux.HandleFunc("/some/deeply/:nested/path/:id", func(rw http.ResponseWriter, r *http.Request, c Context) {})

	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/some/deeply/nested/path/id", nil)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mux.ServeHTTP(rw, r)
	}
}
```

(similar tests with muxter swapped out are included in the benchmarks branch)

```
BenchmarkRoutingParamsMuxter-16                 24841117                46.91 ns/op
BenchmarkRoutingParamsHttpRouter-16             11504378                87.89 ns/op
BenchmarkRoutingParamsGin-16                    20096968                54.21 ns/op
BenchmarkRoutingParamsEcho-16                   15106918                68.29 ns/op
```

## Examples

```go
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/davidmdm/muxter"
)

func main() {
	mux := muxter.New()

	// Register middlewares.
	// (Registered handlers before a call to muxter.Use are not affected but handlers registered after are)
	mux.Use(
		// Add auth middleware
		func(h muxter.Handler) muxter.Handler {
			return muxter.HandlerFunc(func(w http.ResponseWriter, r *http.Request, c muxter.Context) {
				if r.Header.Get("Authorization") != os.Getenv("API_KEY") {
					http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
					return
				}
				h.ServeHTTP(w, r)
			})
		},
		// ... continue adding middlewares variadically
	)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request, c muxter.Context) {
		io.WriteString(w, "hello world!")
	})

	// muxter matches path params
	mux.HandleFunc("/resource/:id", func(w http.ResponseWriter, r *http.Request, c muxter.Context) {
		id := c.Param(r, "id")
		io.WriteString(w, id)
	})

	// muxter matches catchalls
	mux.HandleFunc("/resource/*name", func(w http.ResponseWriter, r *http.Request, c muxter.Context) {
		name := c.Param(r, "name")
		io.WriteString(w, id)
	})

	// muxter matches pattern params
	mux.HandleFunc("/resource/:id", func(w http.ResponseWriter, r *http.Request, c muxter.Context) {
		fmt.Println("pattern:", c.Pattern())
		id := c.Param(r, "id")
		io.WriteString(w, id)
	})

	// muxter accepts middlewares and provides basic ones for Method matching.
	mux.HandleFunc(
		"/resource",
		func(w http.ResponseWriter, r *http.Request, c muxter.Context) {
			io.WriteString(w, "hello world!")
		},
		muxter.POST, // Returns 405 if method is not POST
	)

	// Muxes can be composed since a mux is simple a muxter.Handler
	mux.Handle("/api/v1/", GetAPIV1Mux(), V1AuthMiddleware)
	mux.Handle("/api/v2/", GetAPIV2Mux(), V2AuthMiddleware)

	// Register different method handlers to the same route pattern
	mux.Handle(
		"/resource/:id",
		muxter.MethodHandler{
			GET: muxter.HandlerFunc(func(w http.ResponseWriter, r *http.Request, c muxter.Context) {
				// get resource
			}),
			PUT: muxter.HandlerFunc(func(w http.ResponseWriter, r *http.Request, c muxter.Context) {
				// put resource
			}),
			DELETE: muxter.HandlerFunc(func(w http.ResponseWriter, r *http.Request, c muxter.Context) {
				// delete resource
			}),
			MethodNotAllowedHandler: muxter.HandlerFunc(func(w http.ResponseWriter, r *http.Request, c muxter.Context) {
				// custom method not allowed handler
 			})
		},
	)

	// Add a custom not found handler.
	mux.NotFoundHandler = func(w http.ResponseWriter, r *http.Request, c muxter.Context) {
		// custom not found logic
	}

	http.ListenAndServe(":8080", mux)
}
```

## Middlewares

Muxter provides a couple of convenience middlewares. Middlewares are defined as:

```go
type Middleware = func(http.Hander) http.Handler
```

Notice the type alias. This means that middlewares are not of a specific type of the muxter package,
and any function that takes a handler and returns a handler is considered valid middleware.

Muxter provides middlewares for guarding routes for specific Request Methods

- muxter.GET
- muxter.POST
- muxter.DELETE
- muxter.GET
- muxter.PATCH
- muxter.HEAD
- muxter.Method(method string)

A simple logging middleware:

- muxter.Logger(w io.Writer, fn func(overview muxter.RespOverview) string)

A middleware from recovering from panics:

- muxter.Recover(handler func(recovered interface{}, w http.ResponseWriter, r \*http.Request))

a middleware for enabling CORS

- muxter.CORS(options muxter.AccessControlOptions)
- muxter.DefaultCORS // a default permissive cors cofiguration
