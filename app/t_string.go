package main

import (
	"bytes"
	"errors"
	"time"
)

func setCommand(c *client, argv [][]byte) error {
	var ttl time.Duration
	for i := 3; i < len(argv); i++ {
		token := argv[i]
		if bytes.EqualFold(token, []byte("PX")) {
			if i+1 >= len(argv) {
				return errors.New("syntax error")
			}

			n, ok := readDigits(append(argv[i+1], crlfBytes...))
			if !ok {
				return errors.New("value is not an integer or out of range")
			}

			if n <= 0 {
				return errors.New("invalid expire time in 'set' command")
			}

			ttl = time.Duration(n) * time.Millisecond
			i++
		} else {
			return errors.New("syntax error")
		}
	}

	c.db.set(string(argv[1]), string(argv[2]), ttl)
	c.addReply(shared.ok)
	return nil
}

func getCommand(c *client, argv [][]byte) error {
	v, ok := c.db.get(string(argv[1]))
	if !ok {
		c.addReplyNull()
		return nil
	}

	c.addReplyBulk([]byte(v))
	return nil
}
