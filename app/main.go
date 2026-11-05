package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/codecrafters-io/redis-starter-go/app/resp"
)

type value struct {
	data   string
	expiry time.Time
}

type server struct {
	db map[string]value
	l  map[string][]string
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
	s := server{db: make(map[string]value, 16), l: make(map[string][]string, 16)}

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
			break
		}

		inputs := parseInput(string(buf))
		s.processInput(conn, inputs)
	}
}

func parseInput(buf string) []string {
	inputs := []string{}
	buffer := strings.Split(string(buf), "\r\n")
	n := len(buffer)
	for i := 2; i < n-1; i += 2 {
		element := buffer[i]
		inputs = append(inputs, element)
	}

	return inputs
}

func (s *server) processInput(conn net.Conn, inputs []string) {
	n := len(inputs)

	if n < 1 {
		return
	}
	command := strings.ToUpper(inputs[0])

	switch command {
	case "PING":
		output := resp.EncodeSimpleString("PONG")
		_, err := conn.Write(output)
		if err != nil {
			return
		}
	case "ECHO":
		if n < 2 {
			return
		}
		input := inputs[1]
		output := resp.EncodeBulkString(input)
		_, err := conn.Write(output)
		if err != nil {
			return
		}
	case "SET":
		if n < 3 {
			return
		}
		k := inputs[1]
		d := inputs[2]

		expiry := time.Time{}
		if n > 3 && n < 5 {
			return
		}

		if n > 3 && n < 6 {
			expireOption := strings.ToUpper(inputs[3])
			expireTime, err := strconv.Atoi(inputs[4])
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

		output := resp.EncodeSimpleString("OK")
		_, err := conn.Write(output)
		if err != nil {
			return
		}
	case "GET":
		if n < 2 {
			return
		}

		k := inputs[1]
		v, ok := s.db[k]
		if !ok {
			output := resp.EncodeNullBulkString()
			_, err := conn.Write(output)
			if err != nil {
				return
			}
		}

		now := time.Now()
		expiry := v.expiry
		if now.After(expiry) && !expiry.IsZero() {
			output := resp.EncodeNullBulkString()
			_, err := conn.Write(output)
			if err != nil {
				return
			}
			return
		}

		output := resp.EncodeBulkString(v.data)
		_, err := conn.Write(output)
		if err != nil {
			return
		}

	case "RPUSH":
		if n < 3 {
			return
		}

		key := inputs[1]
		for i := 2; i < n; i++ {
			element := inputs[i]
			s.l[key] = append(s.l[key], element)
		}

		length := len(s.l[key])
		output := resp.EncodeInteger(length)
		_, err := conn.Write(output)
		if err != nil {
			return
		}

	case "LRANGE":
		key := inputs[1]
		l, ok := s.l[key]
		res := []string{}
		if !ok {
			output := resp.EncodeArray(res)
			_, err := conn.Write(output)
			if err != nil {
				return
			}
			return
		}

		start, err := strconv.Atoi(inputs[2])
		if err != nil {
			return
		}
		stop, err := strconv.Atoi(inputs[3])
		if err != nil {
			return
		}

		length := len(l)
		switch {
		case start >= length:
			res = []string{}
		case start > stop:
			res = []string{}
		default:
			res = l[start:min(stop+1, length+1)]
		}

		output := resp.EncodeArray(res)
		_, err = conn.Write(output)
		if err != nil {
			return
		}

	default:
		return
	}
}
