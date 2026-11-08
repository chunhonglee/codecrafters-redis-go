// Package server defines the server and Redis commands
package server

import (
	"errors"
	"fmt"
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

type streamID struct {
	milisecondsTime int
	sequenceNumber  int
}

func (id streamID) String() string {
	return fmt.Sprintf("%d-%d", id.milisecondsTime, id.sequenceNumber)
}

type entry map[streamID]map[string]string

type Server struct {
	Cmds       map[string]func(conn net.Conn, inputs []string)
	db         map[string]value
	l          map[string][]string
	mu         sync.Mutex
	waiters    map[string][]chan string
	streams    map[string]entry
	previousID streamID
}

func New() *Server {
	s := Server{
		db:         make(map[string]value, 16),
		l:          make(map[string][]string, 16),
		waiters:    make(map[string][]chan string),
		streams:    make(map[string]entry, 16),
		previousID: streamID{milisecondsTime: 0, sequenceNumber: 0},
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
		"TYPE":   s.Type,
		"XADD":   s.Xadd,
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

	ch := make(chan string, 1)
	s.waiters[key] = append(s.waiters[key], ch)
	s.mu.Unlock()

	if timeoutSec == 0 {
		element := <-ch
		output := resp.EncodeArray([]string{key, element})
		_, err := conn.Write(output)
		if err != nil {
			return
		}
		return
	}

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
			output := resp.EncodeNullArray()
			_, err := conn.Write(output)
			if err != nil {
				return
			}
		} else {
			element := <-ch
			output := resp.EncodeArray([]string{key, element})
			_, err := conn.Write(output)
			if err != nil {
				return
			}
		}
	}
}

func (s *Server) Type(conn net.Conn, inputs []string) {
	n := len(inputs)
	if n < 1 {
		return
	}

	key := inputs[1]
	output := resp.EncodeSimpleString("none")

	s.mu.Lock()
	_, ok := s.db[key]
	s.mu.Unlock()

	if ok {
		output = resp.EncodeSimpleString("string")
	}

	_, ok = s.streams[key]
	if ok {
		output = resp.EncodeSimpleString("stream")
	}

	_, err := conn.Write(output)
	if err != nil {
		return
	}
}

func (s *Server) Xadd(conn net.Conn, inputs []string) {
	n := len(inputs)
	streamKey := inputs[1]
	_, exist := s.streams[streamKey]
	if !exist {
		s.streams[streamKey] = make(entry, 16)
	}
	id, err := s.validateID(streamKey, inputs[2])
	if err != nil {
		output := resp.EncodeSimpleError(err.Error())
		_, err := conn.Write(output)
		if err != nil {
			return
		}
		return
	}
	s.mu.Lock()
	s.previousID = id
	s.mu.Unlock()

	_, exist = s.streams[streamKey][id]
	if !exist {
		s.streams[streamKey][id] = make(map[string]string, 16)
	}

	for i := 3; i < n; i += 2 {
		key := inputs[i]
		value := inputs[i+1]
		s.streams[streamKey][id][key] = value
	}

	output := resp.EncodeBulkString(id.String())
	_, err = conn.Write(output)
	if err != nil {
		return
	}
}

func (s *Server) validateID(streamKey string, id string) (streamID, error) {
	if id == "*" {
		s.mu.Lock()
		stream := s.streams[streamKey]
		s.mu.Unlock()
		milisecondsTime := int(time.Now().UnixMilli())
		sequenceNumber := 0
		for key := range stream {
			if key.milisecondsTime == milisecondsTime {
				sequenceNumber = max(sequenceNumber, key.sequenceNumber+1)
			}
		}

		return streamID{milisecondsTime: milisecondsTime, sequenceNumber: sequenceNumber}, nil
	}

	parts := strings.Split(id, "-")
	milisecondsTime, err1 := strconv.Atoi(parts[0])

	if parts[1] == "*" {
		s.mu.Lock()
		stream := s.streams[streamKey]
		sequenceNumber := 0
		for key := range stream {
			if key.milisecondsTime == milisecondsTime {
				sequenceNumber = max(sequenceNumber, key.sequenceNumber+1)
			}
		}
		s.mu.Unlock()

		if milisecondsTime <= 0 && sequenceNumber <= 0 {
			return streamID{milisecondsTime: 0, sequenceNumber: 1}, nil
		}

		return streamID{milisecondsTime: milisecondsTime, sequenceNumber: sequenceNumber}, nil
	}

	sequenceNumber, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		// NOTE: Undefined behaviour
		return streamID{}, nil
	}

	if milisecondsTime <= 0 && sequenceNumber <= 0 {
		return streamID{}, errors.New("The ID specified in XADD must be greater than 0-0")
	}

	newID := streamID{milisecondsTime: milisecondsTime, sequenceNumber: sequenceNumber}
	s.mu.Lock()
	previousID := s.previousID
	s.mu.Unlock()

	if (newID.milisecondsTime == previousID.milisecondsTime) && (newID.sequenceNumber > previousID.sequenceNumber) {
		return newID, nil
	}

	if newID.milisecondsTime > previousID.milisecondsTime {
		return newID, nil
	}

	return streamID{}, errors.New("The ID specified in XADD is equal or smaller than the target stream top item")
}
