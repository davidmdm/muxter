package pool

import (
	"net/http"
	"net/url"
	"sync"
)

type ParamPool struct {
	pool *sync.Pool
}

func (p ParamPool) Get() map[string]string {
	return p.pool.Get().(map[string]string)
}

func (p ParamPool) Put(params map[string]string) {
	if params == nil {
		return
	}
	for k := range params {
		delete(params, k)
	}
	p.pool.Put(params)
}

var Params = ParamPool{
	pool: &sync.Pool{New: func() interface{} { return make(map[string]string) }},
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
