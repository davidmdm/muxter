package muxter

import "sync"

type paramPool struct {
	pool *sync.Pool
}

func (p paramPool) Get() map[string]string {
	return p.pool.Get().(map[string]string)
}

func (p paramPool) Put(params map[string]string) {
	if params == nil {
		return
	}
	for k := range params {
		delete(params, k)
	}
	p.pool.Put(params)
}

var pool = paramPool{
	pool: &sync.Pool{New: func() interface{} { return make(map[string]string) }},
}
