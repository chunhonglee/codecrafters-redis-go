package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type value struct {
	data   string
	expiry time.Time
}

type server struct {
	db map[string]value
}

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Println("Logs from your program will appear here!")

	// Uncomment the code below to pass the first stage
	l, err := net.Listen("tcp", "0.0.0.0:6379")
	if err != nil {
		fmt.Println("Failed to bind to port 6379")
		os.Exit(1)
	}
	defer func() {
		err := l.Close()
		if err != nil {
			fmt.Println("Error closing listener")
		}
	}()
	s := server{db: make(map[string]value, 16)}

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		go s.handleConnection(conn)
	}
}

func (s *server) handleConnection(conn net.Conn) {
	buf := make([]byte, 1024)
	for {
		_, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Error reading buffer: ", err.Error())
			break
		}

		inputs := strings.Split(string(buf), "\r\n")
		s.processInput(conn, inputs)
	}
}

func (s *server) processInput(conn net.Conn, inputs []string) {
	n := len(inputs)

	if n < 3 {
		return
	}
	command := strings.ToUpper(inputs[2])

	switch command {
	case "PING":
		output := encodeSimpleString("PONG")
		_, err := conn.Write(output)
		if err != nil {
			return
		}
	case "ECHO":
		if n < 5 {
			return
		}
		input := inputs[4]
		output := encodeBulkString(input)
		_, err := conn.Write(output)
		if err != nil {
			return
		}
	case "SET":
		if n < 7 {
			return
		}
		k := inputs[4]
		d := inputs[6]

		expiry := time.Time{}
		if n > 8 && n < 11 {
			return
		}

		if n > 8 && n < 13 {
			expireOption := strings.ToUpper(inputs[8])
			expireTime, err := strconv.Atoi(inputs[10])
			if err != nil {
				return
			}

			duration := time.Duration(expireTime)
			switch expireOption {
			case "EX":
				duration = duration * time.Second
			case "PX":
				duration = duration * time.Millisecond
			default:
				return
			}
			now := time.Now()
			expiry = now.Add(duration)
		}
		s.db[k] = value{data: d, expiry: expiry}

		output := encodeSimpleString("OK")
		_, err := conn.Write(output)
		if err != nil {
			return
		}
	case "GET":
		if n < 5 {
			return
		}

		fmt.Println(s.db)
		k := inputs[4]
		v, ok := s.db[k]
		if !ok {
			output := encodeNullBulkString()
			_, err := conn.Write(output)
			if err != nil {
				return
			}
		}

		now := time.Now()
		expiry := v.expiry
		if now.After(expiry) && !expiry.IsZero() {
			output := encodeNullBulkString()
			_, err := conn.Write(output)
			if err != nil {
				return
			}
			return
		}

		output := encodeBulkString(v.data)
		_, err := conn.Write(output)
		if err != nil {
			return
		}

	default:
		return
	}
}

func encodeSimpleString(s string) []byte {
	return fmt.Appendf(nil, "+%s\r\n", s)
}

func encodeBulkString(s string) []byte {
	n := len(s)
	return fmt.Appendf(nil, "$%d\r\n%s\r\n", n, s)
}

func encodeNullBulkString() []byte {
	return fmt.Appendf(nil, "$-1\r\n")
}
