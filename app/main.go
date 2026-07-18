package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
)

var (
	ping = []byte("PING")
	echo = []byte("ECHO")
	set  = []byte("SET")
	get  = []byte("GET")
)

var storage = map[string]string{}
var mu sync.RWMutex

func main() {
	l, err := net.Listen("tcp", "0.0.0.0:6379")
	if err != nil {
		fmt.Println("[ERROR] Failed to bind to port 6379")
		os.Exit(1)
	}
	defer l.Close()

	for {
		conn, connErr := l.Accept()
		if connErr != nil {
			log.Printf("[Error] accepting connection: %v", connErr)
			os.Exit(1)
		}
		go newClient(conn).serve()
	}
}

func respond(out []byte, args [][]byte) ([]byte, error) {
	if len(args) == 0 {
		return out, nil
	}

	switch {
	case bytes.EqualFold(args[0], ping):
		out = append(out, shared.pong...)

	case bytes.EqualFold(args[0], echo):
		if len(args) != 2 {
			return out, fmt.Errorf("echo command expected 1 argument, got %d", len(args)-1)
		}
		out = appendBulkString(out, args[1])

	case bytes.EqualFold(args[0], set):
		if len(args) < 3 {
			return out, fmt.Errorf("set command expected at least 2 arguments, got %d", len(args)-1)
		}

		mu.Lock()
		storage[string(args[1])] = string(args[2])
		mu.Unlock()

		out = append(out, shared.ok...)

	case bytes.EqualFold(args[0], get):
		if len(args) != 2 {
			return out, fmt.Errorf("get command expected 1 argument, got %d", len(args)-1)
		}

		mu.RLock()
		defer mu.RUnlock()
		if v, exist := storage[string(args[1])]; exist {
			out = appendBulkString(out, []byte(v))
			return out, nil
		}

		return appendNullBulkString(out), nil

	default:
		return out, fmt.Errorf("unknown command %q", args[0])
	}

	return out, nil
}
