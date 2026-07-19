package main

// db is the keyspace (Redis calls the map "dict"). All access goes through
// the owner goroutine started in newDB, reqs is its mailbox.
type db struct {
	reqs chan request
}

type request interface {
	apply(dict map[string]string)
}

func newDB() *db {
	dict := map[string]string{}

	d := &db{
		reqs: make(chan request),
	}

	go d.loop(dict)

	return d
}

func (d *db) loop(dict map[string]string) {
	for req := range d.reqs {
		req.apply(dict)
	}
}

func (d *db) set(key, val string) {
	reply := make(chan setReply, 1)
	d.reqs <- setRequest{
		key:   key,
		val:   val,
		reply: reply,
	}

	<-reply
}

func (d *db) get(key string) (string, bool) {
	reply := make(chan getReply, 1)
	d.reqs <- getRequest{
		key:   key,
		reply: reply,
	}

	r := <-reply
	return r.val, r.ok
}

type setRequest struct {
	key   string
	val   string
	reply chan setReply
}

type setReply struct{}

func (req setRequest) apply(dict map[string]string) {
	dict[req.key] = req.val
	req.reply <- setReply{}
}

type getRequest struct {
	key   string
	reply chan getReply
}

type getReply struct {
	val string
	ok  bool
}

func (req getRequest) apply(dict map[string]string) {
	v, ok := dict[req.key]
	req.reply <- getReply{val: v, ok: ok}
}
