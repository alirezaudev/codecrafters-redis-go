package main

import "fmt"

type handler interface {
	serveCommand(c *client, argv [][]byte) error
}

type handlerFunc func(c *client, argv [][]byte) error

func (f handlerFunc) serveCommand(c *client, argv [][]byte) error {
	return f(c, argv)
}

type command struct {
	name  string
	arity int
	h     handler
}

var commandTable = map[string]command{
	"PING": {name: "ping", arity: -1, h: handlerFunc(pingCommand)},
	"ECHO": {name: "echo", arity: 2, h: handlerFunc(echoCommand)},
	"SET":  {name: "set", arity: -3, h: handlerFunc(setCommand)},
	"GET":  {name: "get", arity: 2, h: handlerFunc(getCommand)},
}

func processCommand(c *client, argv [][]byte) {
	if len(argv) == 0 {
		return
	}

	name := argv[0]
	buf, err := commandToUpper(name)
	if err != nil {
		c.addReplyError(err.Error())
		return
	}

	cmd, ok := commandTable[string(buf[:len(name)])]
	if !ok {
		c.addReplyError(fmt.Sprintf("ERR unknown command '%s'", name))
		return
	}

	if (cmd.arity > 0 && cmd.arity != len(argv)) || len(argv) < -cmd.arity {
		c.addReplyError(fmt.Sprintf("ERR wrong number of arguments for '%s' command", cmd.name))
		return
	}

	if err := cmd.h.serveCommand(c, argv); err != nil {
		c.addReplyError("ERR " + err.Error())
	}
}

func commandToUpper(name []byte) ([32]byte, error) {
	var buf [32]byte
	if len(name) > len(buf) {
		return buf, fmt.Errorf("ERR unknown command '%s'", name)
	}

	for i, b := range name {
		if 'a' <= b && b <= 'z' {
			b -= 'a' - 'A'
		}
		buf[i] = b
	}
	return buf, nil
}

func pingCommand(c *client, argv [][]byte) error {
	if len(argv) > 2 {
		return fmt.Errorf("wrong number of arguments for 'ping' command")
	}

	if len(argv) == 2 {
		c.addReplyBulk(argv[1])
		return nil
	}

	c.addReply(shared.pong)
	return nil
}

func echoCommand(c *client, argv [][]byte) error {
	c.addReplyBulk(argv[1])
	return nil
}
