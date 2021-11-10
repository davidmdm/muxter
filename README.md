# muxter

## What is muxter?

Muxter is a HTTP request multiplexer.

## Why muxter?

The go community generally likes to keep dependencies to a minimum.
Every week a new gopher will ask what dependency they should use for web development.
Should they use gorilla / gin / echo / httprouter?

What is the answer? The standard library. Truth be told, I agree whole heartedly.
I want to use the net/http ServeMux for my servers. However it does not match path params,
and that makes it just not viable to use for anything other than play.

So why muxter?

- It aims to route and work exactly as the standard library's http.ServeMux.
- It's small.
- It's a hundred percent standard library compatible.

There are no special handler function types. No such thing as a muxter.Context that hides http request and response write types.

### Caveats

Are there differences with the standard library?

Small ones.

- It does not parse or handle hosts/ports.
- mux.Handle accepts variadic middlewares.

## Examples

```go
package main

import (
    "io"
    "net/http"
    "github.com/davidmdm/muxter"
)

func main() {
    mux := muxter.New()

    mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
        io.WriteString("hello world!")
    })

    // muxter matches path params
    mux.HandleFunc("/resource/:id", func(rw http.ResponseWriter, r *http.Request) {
        id := muxter.Param("id")
        io.WriteString(id)
    })

    // muxter accepts middlewares and provides basic ones for Method matching.
    mux.HandleFunc(
        "/resource",
        func(rw http.ResponseWriter, r *http.Request) {
            io.WriteString("hello world!")
        },
        muxter.POST,
    )

    http.ListenAndServe(":8080", mux)
}
```
