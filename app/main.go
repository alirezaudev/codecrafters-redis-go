package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync"
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
