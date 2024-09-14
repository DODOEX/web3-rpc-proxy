package endpoint

import (
	"slices"
	"sync"
)

type Cache struct {
	endpoints map[string]*Endpoint
	chains    map[uint64][]string
	rwm       sync.RWMutex
}

func NewCache() *Cache {
	s := &Cache{
		chains:    make(map[uint64][]string),
		endpoints: make(map[string]*Endpoint),
	}

	return s
}

func (c *Cache) Chains() []uint64 {
	c.rwm.RLock()
	defer c.rwm.RUnlock()

	i := 0
	chains := make([]uint64, len(c.chains))
	for k := range c.chains {
		chains[i] = k
		i++
	}
	return chains
}

func (c *Cache) GetAll(chainID uint64) (v []*Endpoint, ok bool) {
	c.rwm.RLock()
	urls, ok := c.chains[chainID]
	c.rwm.RUnlock()
	if !ok {
		return nil, false
	}

	if len(urls) == 0 {
		return nil, false
	}

	c.rwm.RLock()
	defer c.rwm.RUnlock()
	v = make([]*Endpoint, len(urls))
	for i := range urls {
		if _v, ok := c.endpoints[urls[i]]; ok && _v != nil {
			v[i] = _v
		}
	}
	return v, true
}

func (c *Cache) Get(url string) (v *Endpoint, ok bool) {
	c.rwm.RLock()
	defer c.rwm.RUnlock()

	v, ok = c.endpoints[url]
	if !ok {
		return nil, false
	}
	return v, ok
}

func (c *Cache) Put(endpoint *Endpoint) {
	c.rwm.Lock()
	defer c.rwm.Unlock()

	url := endpoint.Url().String()
	if v, ok := c.endpoints[url]; ok {
		if v.ChainID() != endpoint.ChainID() {
			c.chains[endpoint.ChainID()] = slices.DeleteFunc(c.chains[endpoint.ChainID()], func(k string) bool { return k == url })
		}
	} else {
		c.endpoints[url] = endpoint
		if c.chains[endpoint.ChainID()] == nil {
			c.chains[endpoint.ChainID()] = []string{url}
		} else {
			c.chains[endpoint.ChainID()] = append(c.chains[endpoint.ChainID()], url)
		}
	}
}

func (c *Cache) Remove(url string) {
	c.rwm.Lock()
	defer c.rwm.Unlock()

	if v, ok := c.endpoints[url]; ok {
		delete(c.endpoints, url)
		c.chains[v.ChainID()] = slices.DeleteFunc(c.chains[v.ChainID()], func(k string) bool { return k == url })
	}
}

func (c *Cache) Purge(chainID uint64) int {
	c.rwm.Lock()
	defer c.rwm.Unlock()

	if urls, ok := c.chains[chainID]; ok {
		for i := range urls {
			delete(c.endpoints, urls[i])
		}
		delete(c.chains, chainID)
		return len(urls)
	}
	return 0
}
