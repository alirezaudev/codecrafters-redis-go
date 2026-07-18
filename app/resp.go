package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// RESP protocol vocabulary.
const (
	crlf               = "\r\n"
	delimiter          = '\n'
	simpleStringPrefix = '+'
	errorPrefix        = '-'
	bulkStringPrefix   = '$'
	arrayPrefix        = '*'
)

var crlfBytes = []byte(crlf)

var errProtocol = errors.New("protocol violation")

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

// appendNullBulkString appends the RESP2 null bulk string ("$-1\r\n") to dst.
func appendNullBulkString(dst []byte) []byte {
	return append(dst, "$-1"+crlf...)
}

// appendError appends msg to dst as a RESP error
// ("-" + <msg> + "\r\n")
func appendError(dst []byte, msg string) []byte {
	dst = append(dst, errorPrefix)
	dst = append(dst, msg...)
	dst = append(dst, crlf...)
	return dst
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
