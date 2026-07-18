package main

import "sync"

// db is the keyspace (Redis calls the map "dict").
type db struct {
	mu   sync.RWMutex
	dict map[string]string
}

func newDB() *db {
	return &db{dict: map[string]string{}}
}

func (d *db) set(key, val string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.dict[key] = val
}

func (d *db) get(key string) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	v, ok := d.dict[key]
	return v, ok
}
