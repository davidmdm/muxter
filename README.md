# muxter

## What is muxter?

Muxter is a HTTP request multiplexer.

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
- Mux's can be easily composed. A mux can register another mux.
- It's small.
- It's a hundred percent standard library compatible.

And most importantly it does not seek to do or become anything more,
or have many options or be framework-y in anyway.

Maybe provide some highly desired middlewares in the future... Maybe.
But that's it. Don't murder me. Maybe.

### Caveats

Are there differences with the standard library?

Small ones.

- It does not parse or handle hosts/ports.
- mux.Handle accepts variadic middlewares.

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
		func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != os.Getenv("API_KEY") {
					http.Error(rw, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
					return
				}
				h.ServeHTTP(rw, r)
			})
		},
		// Add logger middleware
		func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				fmt.Println(r.Method, r.URL.Path)
				h.ServeHTTP(rw, r)
			})
		},
	)

	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		io.WriteString(rw, "hello world!")
	})

	// muxter matches path params
	mux.HandleFunc("/resource/:id", func(rw http.ResponseWriter, r *http.Request) {
		id := muxter.Param(r, "id")
		io.WriteString(rw, id)
	})

	// muxter accepts middlewares and provides basic ones for Method matching.
	mux.HandleFunc(
		"/resource",
		func(rw http.ResponseWriter, r *http.Request) {
			io.WriteString(rw, "hello world!")
		},
		muxter.POST, // Returns 405 if method is not POST
	)

	// Can register another mux to extend the current mux
	mux.RegisterMux("/api/v1", GetAPIV1Mux(), V1AuthMiddleware)
	mux.RegisterMux("/api/v2", GetAPIV2Mux(), V2AuthMiddleware)

	// Add a custom not found handler.
	mux.NotFoundHandler = func(rw http.ResponseWriter, r *http.Request) {
		// custom not found logic
	}

	http.ListenAndServe(":8080", mux)
}
```
