package resourcecache

import (
	"container/list"
	"sync"
	"time"
)

const MaxKeyLength = 4 * 1024

type Item struct {
	Value     any
	ExpiresAt time.Time
}

type LRU struct {
	mu       sync.RWMutex
	entries  map[string]*list.Element
	order    *list.List
	capacity int
}

type entry struct {
	key  string
	item Item
}

func New(capacity int) *LRU {
	if capacity < 1 {
		capacity = 1
	}
	return &LRU{
		entries:  make(map[string]*list.Element, capacity+1),
		order:    list.New(),
		capacity: capacity,
	}
}

func (c *LRU) Get(key string) (Item, bool) {
	if !ValidKey(key) {
		return Item{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	element, ok := c.entries[key]
	if !ok {
		return Item{}, false
	}
	item := element.Value.(*entry).item
	if time.Now().After(item.ExpiresAt) {
		c.remove(element)
		return Item{}, false
	}
	c.order.MoveToFront(element)
	return item, true
}

func (c *LRU) Set(key string, item Item) bool {
	if !ValidKey(key) {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.entries[key]; ok {
		element.Value.(*entry).item = item
		c.order.MoveToFront(element)
		return true
	}
	c.entries[key] = c.order.PushFront(&entry{key: key, item: item})
	for len(c.entries) > c.capacity {
		c.remove(c.order.Back())
	}
	return true
}

func (c *LRU) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func (c *LRU) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*list.Element, c.capacity+1)
	c.order.Init()
}

func (c *LRU) InvalidatePrefixes(prefixes ...string) {
	if len(prefixes) == 0 {
		c.Reset()
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, element := range c.entries {
		for _, prefix := range prefixes {
			if prefix == "" || len(key) >= len(prefix) && key[:len(prefix)] == prefix {
				delete(c.entries, key)
				c.order.Remove(element)
				break
			}
		}
	}
}

func (c *LRU) remove(element *list.Element) {
	if element == nil {
		return
	}
	current := element.Value.(*entry)
	delete(c.entries, current.key)
	c.order.Remove(element)
}

func ValidKey(key string) bool {
	return len(key) <= MaxKeyLength
}
