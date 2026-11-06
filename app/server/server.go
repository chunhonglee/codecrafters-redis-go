// Package server defines the server and Redis commands
package server

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/codecrafters-io/redis-starter-go/app/resp"
)

type value struct {
	data   string
	expiry time.Time
}

type Server struct {
	Cmds map[string]func(conn net.Conn, inputs []string)
	db   map[string]value
	l    map[string][]string
}

func New() *Server {
	s := Server{db: make(map[string]value, 16), l: make(map[string][]string, 16)}
	s.Cmds = map[string]func(conn net.Conn, inputs []string){
		"PING":   s.Ping,
		"ECHO":   s.Echo,
		"SET":    s.Set,
		"GET":    s.Get,
		"RPUSH":  s.Rpush,
		"LRANGE": s.Lrange,
		"LPUSH":  s.Lpush,
		"LLEN":   s.Llen,
		"LPOP":   s.Lpop,
	}

	return &s
}

func (s *Server) Ping(conn net.Conn, inputs []string) {
	output := resp.EncodeSimpleString("PONG")
	_, err := conn.Write(output)
	if err != nil {
		return
	}
}

func (s *Server) Echo(conn net.Conn, inputs []string) {
	n := len(inputs)
	if n < 2 {
		return
	}
	input := inputs[1]
	output := resp.EncodeBulkString(input)
	_, err := conn.Write(output)
	if err != nil {
		return
	}
}

func (s *Server) Set(conn net.Conn, inputs []string) {
	n := len(inputs)
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
}

func (s *Server) Get(conn net.Conn, inputs []string) {
	n := len(inputs)
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
}

func (s *Server) Rpush(conn net.Conn, inputs []string) {
	n := len(inputs)
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
}

func (s *Server) Lrange(conn net.Conn, inputs []string) {
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
	if start < 0 {
		start = max(start+length, 0)
	}

	if stop < 0 {
		stop = max(stop+length, 0)
	}

	if start < length || start <= stop {
		res = l[start:min(stop+1, length)]
	}

	output := resp.EncodeArray(res)
	_, err = conn.Write(output)
	if err != nil {
		return
	}
}

func (s *Server) Lpush(conn net.Conn, inputs []string) {
	n := len(inputs)
	if n < 3 {
		return
	}

	key := inputs[1]
	for i := 2; i < n; i++ {
		element := inputs[i]
		s.l[key] = append([]string{element}, s.l[key]...)
	}

	length := len(s.l[key])
	output := resp.EncodeInteger(length)
	_, err := conn.Write(output)
	if err != nil {
		return
	}
}

func (s *Server) Llen(conn net.Conn, inputs []string) {
	n := len(inputs)
	if n < 2 {
		return
	}
	key := inputs[1]
	l := s.l[key]
	length := len(l)
	output := resp.EncodeInteger(length)
	_, err := conn.Write(output)
	if err != nil {
		return
	}
}

func (s *Server) Lpop(conn net.Conn, inputs []string) {
	n := len(inputs)
	if n < 2 {
		return
	}

	key := inputs[1]
	l, ok := s.l[key]
	if !ok {
		output := resp.EncodeNullBulkString()
		_, err := conn.Write(output)
		if err != nil {
			return
		}
		return
	}

	optional, err := strconv.Atoi(inputs[2])
	if err != nil {
		result := l[0]
		output := resp.EncodeBulkString(result)
		_, err := conn.Write(output)
		if err != nil {
			return
		}
		s.l[key] = l[1:]
		return
	}

	length := len(l)
	result := l
	if optional > length {
		result = l
		output := resp.EncodeArray(result)
		_, err := conn.Write(output)
		if err != nil {
			return
		}
		s.l[key] = []string{}
		return
	}

	result = l[:optional]
	output := resp.EncodeArray(result)
	_, err = conn.Write(output)
	if err != nil {
		return
	}
	s.l[key] = l[optional:]
}
