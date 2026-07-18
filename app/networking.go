package main

import (
	"bufio"
	"errors"
	"io"
	"log"
	"net"
)

const replyChunkBytes = 4 << 10

var shared = struct{ ok, pong, nullBulk []byte }{
	ok:       []byte("+OK" + crlf),
	pong:     []byte("+PONG" + crlf),
	nullBulk: []byte("$-1" + crlf),
}

// client is both the connection state and the response writer
type client struct {
	conn   net.Conn
	reader *bufio.Reader
	reply  []byte
	db     *db
}

func newClient(conn net.Conn, database *db) *client {
	return &client{
		conn:   conn,
		reader: bufio.NewReader(conn),
		reply:  make([]byte, 0, replyChunkBytes),
		db:     database,
	}
}

// serve reads commands until it is closed or violates the
// protocol, and writing back the reply.
func (c *client) serve() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Error] recovered from panic: %v", r)
		}
	}()
	defer c.conn.Close()

	for { // connection lifetime
		argv, readErr := readCommand(c.reader)
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				log.Printf("[EOF] Connection closed by remote host")
				return
			}
			log.Printf("[Error] failed to read command: %v", readErr)
			return
		}

		c.reply = c.reply[:0]
		processCommand(c, argv)

		if _, err := c.conn.Write(c.reply); err != nil {
			log.Printf("[Error] failed to write response: %v", err)
			return
		}
	}
}

// addReply appends precomputed wire bytes (shared.*) to the reply buffer.
func (c *client) addReply(p []byte) {
	c.reply = append(c.reply, p...)
}

func (c *client) addReplyBulk(p []byte) {
	c.reply = appendBulkString(c.reply, p)
}

func (c *client) addReplyNull() {
	c.reply = append(c.reply, shared.nullBulk...)
}

func (c *client) addReplyError(msg string) {
	c.reply = appendError(c.reply, msg)
}
