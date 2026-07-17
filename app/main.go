package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
)

func main() {
	l, err := net.Listen("tcp", "0.0.0.0:6379")
	if err != nil {
		fmt.Println("Failed to bind to port 6379")
		os.Exit(1)
	}
	defer l.Close()

	for {
		conn, connErr := l.Accept()
		if connErr != nil {
			log.Printf("Error accepting connection: %v", connErr)
			os.Exit(1)
		}
		go func(conn net.Conn) {
			defer conn.Close()

			reader := bufio.NewReader(conn)
			_, readErr := reader.ReadString('\n')
			if readErr != nil {
				log.Printf("Error reading message: %v", readErr)
				return
			}
			_, writeErr := conn.Write([]byte("+PONG\r\n"))
			if writeErr != nil {
				log.Printf("Error writing message: %v", writeErr)
				return
			}

		}(conn)
	}
}
