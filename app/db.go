package main

import "time"

// db is the keyspace (Redis calls the map "dict"). All access goes through
// the owner goroutine started in newDB, reqs is its mailbox.
type db struct {
	reqs chan request
}

type entry struct {
	value     string
	expiresAt time.Time
}

type request interface {
	apply(dict map[string]entry)
}

func newDB() *db {
	dict := map[string]entry{}

	d := &db{
		reqs: make(chan request),
	}

	go d.loop(dict)

	return d
}

func (d *db) loop(dict map[string]entry) {
	for req := range d.reqs {
		req.apply(dict)
	}
}

func (d *db) set(key, val string, ttl time.Duration) {
	reply := make(chan setReply, 1)
	req := setRequest{
		key:   key,
		val:   val,
		reply: reply,
	}

	if ttl > 0 {
		req.expiresAt = time.Now().Add(ttl)
	}

	d.reqs <- req

	<-reply
}

func (d *db) get(key string) (string, bool) {
	reply := make(chan getReply, 1)
	d.reqs <- getRequest{
		key:   key,
		reply: reply,
	}

	r := <-reply
	return r.entry.value, r.ok
}

type setRequest struct {
	key       string
	val       string
	expiresAt time.Time
	reply     chan setReply
}

type setReply struct{}

func (req setRequest) apply(dict map[string]entry) {
	dict[req.key] = entry{value: req.val, expiresAt: req.expiresAt}
	req.reply <- setReply{}
}

type getRequest struct {
	key   string
	reply chan getReply
}

type getReply struct {
	entry entry
	ok    bool
}

func (req getRequest) apply(dict map[string]entry) {
	v, ok := dict[req.key]
	if !v.expiresAt.IsZero() && time.Now().After(v.expiresAt) {
		v = entry{}
		ok = false
		delete(dict, req.key)
	}
	req.reply <- getReply{entry: v, ok: ok}
}
