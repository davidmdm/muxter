package muxter

import (
	"net/http"

	"github.com/davidmdm/muxter/internal/pool"
)

func StripDepth(depth int, handler Handler) Handler {
	return HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
		r2 := pool.Requests.Get()
		defer pool.Requests.Put(r2)

		*r2 = *r

		r2.URL = pool.URL.Get()
		defer pool.URL.Put(r2.URL)

		*r2.URL = *r.URL

		r2.URL.Path = stripDepth(r.URL.Path, depth)

		handler.ServeHTTPx(w, r2, c)
	})
}

func stripDepth(value string, depth int) string {
	if depth == 0 {
		return value
	}

	var seen int
	var i int

	for i = range value {
		if i != 0 && value[i] == '/' {
			seen++
		}
		if seen == depth {
			break
		}
	}
	if i == len(value)-1 {
		return "/"
	}
	return value[i:]
}
