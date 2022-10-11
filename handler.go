package muxter

import (
	"context"
	"net/http"
)

type Context struct {
	ogReqPath string
	pattern   string
	params    map[string]string
}

func (c Context) Param(key string) string {
	if c.params == nil {
		return ""
	}
	return c.params[key]
}

func (c Context) Params() map[string]string {
	if c.params == nil {
		return nil
	}
	cpy := make(map[string]string, len(c.params))
	for k, v := range c.params {
		cpy[k] = v
	}

	return cpy
}

func (c Context) MatchedPath() string {
	return c.pattern
}

//go:generate moq -out handler_mock_test.go --stub -pkg muxter . Handler
type Handler interface {
	ServeHTTPx(w http.ResponseWriter, r *http.Request, c Context)
}

type HandlerFunc func(w http.ResponseWriter, r *http.Request, c Context)

func (fn HandlerFunc) ServeHTTPx(w http.ResponseWriter, r *http.Request, c Context) {
	fn(w, r, c)
}

func StdAdaptor(h http.Handler) Handler {
	return HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
		*r = *r.WithContext(context.WithValue(r.Context(), paramKey, c.params))
		h.ServeHTTP(w, r)
	})
}
