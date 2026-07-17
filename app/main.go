package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

const (
	crlf             = "\r\n"
	delimiter        = '\n'
	arrayPrefix      = '*'
	bulkStringPrefix = '$'
)

var (
	ping         = []byte("PING")
	pongResponse = []byte("+PONG" + crlf)
)

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
		go func(conn net.Conn) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Error] recovered from panic: %v", r)
				}
			}()
			defer conn.Close()
			reader := bufio.NewReader(conn)

			for { // connection lifetime

				header, headerErr := reader.ReadBytes(delimiter)
				if headerErr != nil {
					if errors.Is(headerErr, io.EOF) {
						log.Printf("[EOF] Connection closed by remote host")
						return
					}
					log.Printf("[Error] Failed to read header: %v", headerErr)
					return
				}

				if header[0] != arrayPrefix {
					log.Printf("[Error] Invalid array prefix: %v", header[0])
					return
				}

				n := readDigits(header[1:]) // array size
				args := make([][]byte, n)
				for i := 0; i < n; i++ {
					sizeLine, sizeLineErr := reader.ReadBytes(delimiter)
					if sizeLineErr != nil {
						log.Printf("[Error] Failed to read arg size: %v", sizeLineErr)
						return
					}

					if sizeLine[0] != bulkStringPrefix {
						log.Printf("[Error] Invalid bulk string prefix: %v", sizeLine[0])
						return
					}

					m := readDigits(sizeLine[1:])
					buf := make([]byte, m+2) // +2: to read \r\n
					_, argReadErr := io.ReadFull(reader, buf)
					if argReadErr != nil {
						log.Printf("[Error] Failed to read arg: %v", argReadErr)
						return
					}
					args[i] = buf[:m]
				}
				if len(args) > 0 {
					if bytes.EqualFold(args[0], ping) {
						_, writeErr := conn.Write(pongResponse)
						if writeErr != nil {
							log.Printf("[Error] Failed to write pong: %v", writeErr)
							return
						}
					}
				}
			}

		}(conn)
	}
}

// readDigits to prevent converting like bytes -> string -> int
func readDigits(digits []byte) int {
	n := 0
	for _, c := range digits {
		if c < '0' || c > '9' {
			return n
		}

		n = n*10 + int(c-'0')
	}

	return n
}
