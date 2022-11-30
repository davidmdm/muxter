package muxter

import (
	"net/http"
	"strings"
)

type MethodHandler struct {
	GET                     Handler
	POST                    Handler
	PUT                     Handler
	PATCH                   Handler
	HEAD                    Handler
	DELETE                  Handler
	MethodNotAllowedHandler Handler
}

func (mh MethodHandler) getHandler(method string) (handler Handler) {
	defer func() {
		if handler == nil {
			if mh.MethodNotAllowedHandler == nil {
				handler = defaultMethodNotAllowedHandler
			} else {
				handler = mh.MethodNotAllowedHandler
			}
		}
	}()

	switch strings.ToUpper(method) {
	case "GET":
		return mh.GET
	case "POST":
		return mh.POST
	case "DELETE":
		return mh.DELETE
	case "PUT":
		return mh.PUT
	case "PATCH":
		return mh.PATCH
	case "HEAD":
		return mh.HEAD
	default:
		return nil
	}
}

func (mh MethodHandler) ServeHTTPx(w http.ResponseWriter, r *http.Request, c Context) {
	mh.getHandler(r.Method).ServeHTTPx(w, r, c)
}
