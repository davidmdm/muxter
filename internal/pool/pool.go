package pool

import (
	"net/http"
	"net/url"
	"sync"

	"github.com/davidmdm/muxter/internal"
)

type ParamPool struct {
	pool *sync.Pool
}

func (p ParamPool) Get() *[]internal.Param {
	params := p.pool.Get().(*[]internal.Param)
	*params = (*params)[:0]
	return params
}

func (p ParamPool) Put(params *[]internal.Param) {
	if params == nil {
		return
	}
	p.pool.Put(params)
}

var Params = ParamPool{
	pool: &sync.Pool{New: func() interface{} {
		p := make([]internal.Param, 12)
		return &p
	}},
}

type RequestPool struct {
	pool *sync.Pool
}

func (pool RequestPool) Get() *http.Request {
	return pool.pool.Get().(*http.Request)
}

func (pool RequestPool) Put(r *http.Request) {
	pool.pool.Put(r)
}

var Requests = RequestPool{
	&sync.Pool{New: func() any { return new(http.Request) }},
}

type URLPool struct {
	pool *sync.Pool
}

func (pool URLPool) Get() *url.URL {
	return pool.pool.Get().(*url.URL)
}

func (pool URLPool) Put(r *url.URL) {
	pool.pool.Put(r)
}

var URL = URLPool{
	&sync.Pool{New: func() any { return new(url.URL) }},
}
