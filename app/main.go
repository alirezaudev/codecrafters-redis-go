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
	"sync"
)

const (
	crlf               = "\r\n"
	delimiter          = '\n'
	arrayPrefix        = '*'
	bulkStringPrefix   = '$'
	simpleStringPrefix = '+'

	replyChunkBytes = 4 << 10
)

var (
	crlfBytes = []byte(crlf)
	ping      = []byte("PING")
	pong      = []byte("PONG")
	echo      = []byte("ECHO")
	set       = []byte("SET")
	get       = []byte("GET")
	ok        = []byte("OK")
	null      = []byte("$-1\r\n")
)

var errProtocol = errors.New("protocol violation")

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
	out := make([]byte, 0, replyChunkBytes)
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

		var err error
		out, err = respond(out[:0], args)
		if err != nil {
			log.Printf("[Error] failed to respond: %v", err)
			return
		}

		if _, err := conn.Write(out); err != nil {
			log.Printf("[Error] failed to write response: %v", err)
			return
		}
	}
}

func respond(out []byte, args [][]byte) ([]byte, error) {
	if len(args) == 0 {
		return out, nil
	}

	switch {
	case bytes.EqualFold(args[0], ping):
		out = appendSimpleString(out, pong)

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

		out = appendSimpleString(out, ok)

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

		return appendNullString(out), nil

	default:
		return out, fmt.Errorf("unknown command %q", args[0])
	}

	return out, nil
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

// appendBulkString appends arg to dst as a RESP bulk string
// ("$" + <len> + "\r\n" + <payload> + "\r\n")
func appendBulkString(dst, arg []byte) []byte {
	dst = append(dst, bulkStringPrefix)
	dst = strconv.AppendInt(dst, int64(len(arg)), 10)
	dst = append(dst, crlf...)
	dst = append(dst, arg...)
	dst = append(dst, crlf...)
	return dst
}

// appendSimpleString appends msg to dst as a RESP simple string
// ("+" + <msg> + "\r\n")
func appendSimpleString(dst, msg []byte) []byte {
	dst = append(dst, simpleStringPrefix)
	dst = append(dst, msg...)
	dst = append(dst, crlf...)
	return dst
}

func appendNullString(dst []byte) []byte {
	return append(dst, null...)
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
