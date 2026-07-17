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
	"strconv"
)

const (
	crlf             = "\r\n"
	delimiter        = '\n'
	arrayPrefix      = '*'
	bulkStringPrefix = '$'
)

var (
	crlfBytes    = []byte(crlf)
	ping         = []byte("PING")
	echo         = []byte("ECHO")
	pongResponse = []byte("+PONG" + crlf)
)

var errProtocol = errors.New("protocol violation")

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
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Error] recovered from panic: %v", r)
		}
	}()
	defer conn.Close()

	reader := bufio.NewReader(conn)

	for { // connection lifetime
		args, readErr := readCommand(reader)
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				log.Printf("[EOF] Connection closed by remote host")
				return
			}
			log.Printf("[Error] failed to read command: %v", readErr)
			return
		}

		if err := respond(conn, args); err != nil {
			log.Printf("[Error] failed to write response: %v", err)
			return
		}
	}
}

func respond(conn net.Conn, args [][]byte) error {
	if len(args) == 0 {
		return nil
	}

	if bytes.EqualFold(args[0], ping) {
		if _, err := conn.Write(pongResponse); err != nil {
			return fmt.Errorf("writing pong: %w", err)
		}
	} else if bytes.EqualFold(args[0], echo) {
		if len(args) != 2 {
			return fmt.Errorf("echo command expected 1 argument, got %d", len(args)-1)
		}
		if _, err := conn.Write(encodeBulkString(args[1])); err != nil {
			return fmt.Errorf("writing echo: %w", err)
		}
	}

	return nil
}

// readCommand reads one complete RESP command (an array of bulk strings,
// e.g. "*1\r\n$4\r\nPING\r\n") and returns its arguments without framing.
func readCommand(reader *bufio.Reader) ([][]byte, error) {
	header, headerErr := reader.ReadBytes(delimiter)
	if headerErr != nil {
		return nil, fmt.Errorf("reading array header: %w", headerErr)
	}

	if header[0] != arrayPrefix {
		return nil, fmt.Errorf("%w: invalid array prefix %q", errProtocol, header[0])
	}

	n, ok := readDigits(header[1:])
	if !ok {
		return nil, fmt.Errorf("%w: bad array size %q", errProtocol, header[1:])
	}

	args := make([][]byte, n)
	for i := 0; i < n; i++ {
		sizeLine, sizeErr := reader.ReadBytes(delimiter)
		if sizeErr != nil {
			return nil, fmt.Errorf("reading bulk string size: %w", sizeErr)
		}

		if sizeLine[0] != bulkStringPrefix {
			return nil, fmt.Errorf("%w: invalid bulk string prefix %q", errProtocol, sizeLine[0])
		}

		m, ok := readDigits(sizeLine[1:])
		if !ok {
			return nil, fmt.Errorf("%w: bad bulk string size %q", errProtocol, sizeLine[1:])
		}

		buf := make([]byte, m+2) // +2: to read \r\n
		if _, err := io.ReadFull(reader, buf); err != nil {
			return nil, fmt.Errorf("reading bulk string payload: %w", err)
		}

		if !bytes.Equal(buf[m:], crlfBytes) {
			return nil, fmt.Errorf("%w: bulk string payload not terminated by CRLF", errProtocol)
		}

		args[i] = buf[:m]
	}

	return args, nil
}

func encodeBulkString(arg []byte) []byte {
	n := len(arg)

	// Maximum decimal digits for an int64 is 20.
	buf := make([]byte, 0, 1+20+2+n+2)

	buf = append(buf, bulkStringPrefix)
	buf = strconv.AppendInt(buf, int64(n), 10)
	buf = append(buf, crlf...)
	buf = append(buf, arg...)
	buf = append(buf, crlf...)

	return buf
}

// readDigits parses a decimal number from a "<digits>\r\n" slice without
// converting to string. ok is false unless every byte before the CRLF
// terminator is a digit.
func readDigits(digits []byte) (int, bool) {
	if len(digits) < 3 {
		return 0, false
	}

	l := len(digits) - 1
	if !bytes.Equal(digits[l-1:], crlfBytes) {
		return 0, false
	}

	n := 0
	for _, c := range digits[:l-1] {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}

	return n, true
}
