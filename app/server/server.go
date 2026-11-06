// Package server defines the server and Redis commands
package server

import (
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/codecrafters-io/redis-starter-go/app/resp"
)

type value struct {
	data   string
	expiry time.Time
}

type Server struct {
	Cmds    map[string]func(conn net.Conn, inputs []string)
	db      map[string]value
	l       map[string][]string
	mu      sync.Mutex
	waiters map[string][]chan string
}

func New() *Server {
	s := Server{
		db:      make(map[string]value, 16),
		l:       make(map[string][]string, 16),
		waiters: make(map[string][]chan string),
	}
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
		"BLPOP":  s.Blpop,
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
	s.mu.Lock()
	s.db[k] = value{data: d, expiry: expiry}
	s.mu.Unlock()

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
	s.mu.Lock()
	v, ok := s.db[k]
	if !ok {
		s.mu.Unlock()
		output := resp.EncodeNullBulkString()
		_, err := conn.Write(output)
		if err != nil {
			return
		}
	}

	now := time.Now()
	expiry := v.expiry
	if now.After(expiry) && !expiry.IsZero() {
		s.mu.Unlock()
		output := resp.EncodeNullBulkString()
		_, err := conn.Write(output)
		if err != nil {
			return
		}
		return
	}
	data := v.data
	s.mu.Unlock()

	output := resp.EncodeBulkString(data)
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

	s.mu.Lock()
	sentToWaiters := 0
	for i := 2; i < n; i++ {
		element := inputs[i]
		if len(s.waiters[key]) > 0 {
			waiterChan := s.waiters[key][0]
			s.waiters[key] = s.waiters[key][1:]
			waiterChan <- element
			sentToWaiters++
		} else {
			s.l[key] = append(s.l[key], element)
		}
	}
	length := len(s.l[key]) + sentToWaiters
	s.mu.Unlock()

	output := resp.EncodeInteger(length)
	_, err := conn.Write(output)
	if err != nil {
		return
	}
}

func (s *Server) Lrange(conn net.Conn, inputs []string) {
	key := inputs[1]

	s.mu.Lock()
	l, ok := s.l[key]
	if !ok {
		s.mu.Unlock()
		output := resp.EncodeArray([]string{})
		_, err := conn.Write(output)
		if err != nil {
			return
		}
		return
	}

	start, err1 := strconv.Atoi(inputs[2])
	stop, err2 := strconv.Atoi(inputs[3])
	if err1 != nil || err2 != nil {
		s.mu.Unlock()
		return
	}

	length := len(l)
	if start < 0 {
		start = max(start+length, 0)
	}
	if stop < 0 {
		stop = max(stop+length, 0)
	}

	res := []string{}
	if start < length || start <= stop {
		res = l[start:min(stop+1, length)]
	}
	s.mu.Unlock()

	output := resp.EncodeArray(res)
	_, err := conn.Write(output)
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
	s.mu.Lock()
	sentToWaiters := 0
	for i := 2; i < n; i++ {
		element := inputs[i]
		if len(s.waiters[key]) > 0 {
			waiterChan := s.waiters[key][0]
			s.waiters[key] = s.waiters[key][1:]
			waiterChan <- element
			sentToWaiters++
		} else {
			s.l[key] = append([]string{element}, s.l[key]...)
		}
	}
	length := len(s.l[key]) + sentToWaiters
	s.mu.Unlock()

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

	s.mu.Lock()
	l := s.l[key]
	s.mu.Unlock()

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
	s.mu.Lock()
	l, ok := s.l[key]

	if !ok {
		s.mu.Unlock()
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
		s.l[key] = l[1:]
		s.mu.Unlock()
		output := resp.EncodeBulkString(result)
		_, err := conn.Write(output)
		if err != nil {
			return
		}
		return
	}

	length := len(l)
	result := l
	if optional <= length {
		result = l[:optional]
	}
	s.l[key] = l[optional:]
	s.mu.Unlock()

	output := resp.EncodeArray(result)
	_, err = conn.Write(output)
	if err != nil {
		return
	}
}

func (s *Server) Blpop(conn net.Conn, inputs []string) {
	n := len(inputs)
	if n < 3 {
		return
	}

	key := inputs[1]
	timeoutSec, err := strconv.ParseFloat(inputs[2], 64)
	if err != nil {
		return
	}

	s.mu.Lock()
	l, ok := s.l[key]
	if ok && len(l) > 0 {
		element := l[0]
		s.l[key] = l[1:]
		s.mu.Unlock()

		output := resp.EncodeArray([]string{key, element})
		_, err := conn.Write(output)
		if err != nil {
			return
		}
		return
	}

	// List is empty or doesn't exist, register as a waiter
	ch := make(chan string, 1)
	s.waiters[key] = append(s.waiters[key], ch)
	s.mu.Unlock()

	// Timeout of 0 means block indefinitely
	if timeoutSec == 0 {
		element := <-ch
		output := resp.EncodeArray([]string{key, element})
		_, err := conn.Write(output)
		if err != nil {
			return
		}
		return
	}

	// Non-zero timeout handling
	timer := time.NewTimer(time.Duration(timeoutSec * float64(time.Second)))
	defer timer.Stop()

	select {
	case element := <-ch:
		output := resp.EncodeArray([]string{key, element})
		_, err := conn.Write(output)
		if err != nil {
			return
		}
	case <-timer.C:
		s.mu.Lock()
		removed := false
		for i, w := range s.waiters[key] {
			if w == ch {
				s.waiters[key] = append(s.waiters[key][:i], s.waiters[key][i+1:]...)
				removed = true
				break
			}
		}
		s.mu.Unlock()

		if removed {
			// Timeout expired before any element was pushed
			output := []byte("*-1\r\n")
			_, err := conn.Write(output)
			if err != nil {
				return
			}
		} else {
			// Race condition: an element was pushed right as the timer fired
			element := <-ch
			output := resp.EncodeArray([]string{key, element})
			_, err := conn.Write(output)
			if err != nil {
				return
			}
		}
	}
}
