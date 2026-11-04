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
	value string
	time  time.Time
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
	// TODO: Change db structure to use struct
	db := make(map[string]value, 16)

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		go handleConnection(conn, &db)

	}
}

func handleConnection(conn net.Conn, db *map[string]value) {
	buf := make([]byte, 1024)
	for {
		_, err := conn.Read(buf)
		if err != nil {
			break
		}

		inputs := parseRESP(buf)
		command := strings.ToUpper(inputs[2])

		switch command {
		case "PING":
			_, err = conn.Write([]byte("+PONG\r\n"))
			if err != nil {
				return
			}
		case "ECHO":
			output := formatRESP(inputs[4])
			_, err = conn.Write(output)
			if err != nil {
				return
			}
		case "SET":
			key := inputs[4]
			v := inputs[6]
			(*db)[key] = value{value: v}
			// TODO: Add expiry
			// Check for length of inputs
			if len(inputs) < 10 {
				(*db)[key] = value{value: v, time: time.Time{}}
			} else {
				expireOption := strings.ToUpper(inputs[8])
				expireTime, err := strconv.Atoi(string(inputs[10]))
				if err != nil {
					return
				}

				switch expireOption {
				case "EX":
					now := time.Now()
					expire := time.Duration(expireTime) * time.Second
					expirationTime := now.Add(expire)
					(*db)[key] = value{value: v, time: expirationTime}
				case "PX":
					now := time.Now()
					expire := time.Duration(expireTime) * time.Millisecond
					expirationTime := now.Add(expire)
					(*db)[key] = value{value: v, time: expirationTime}
				default:
					fmt.Println("Invalid expiry option")
					return
				}
			}

			fmt.Println(db)
			_, err = conn.Write([]byte("+OK\r\n"))
			if err != nil {
				return
			}
		case "GET":
			key := inputs[4]
			v, ok := (*db)[key]
			if !ok {
				_, err = conn.Write([]byte("$-1\r\n"))
				if err != nil {
					return
				}
			}

			now := time.Now()
			expirationTime := v.time
			if now.After(expirationTime) && !expirationTime.IsZero() {
				_, err = conn.Write([]byte("$-1\r\n"))
				if err != nil {
					return
				}
				return
			}

			output := formatRESP(v.value)
			_, err = conn.Write(output)
			if err != nil {
				return
			}
		default:
			fmt.Println("Unknown command")
			return
		}
	}
}

func parseRESP(buf []byte) []string {
	output := strings.Split(string(buf), "\r\n")

	return output
}

func formatRESP(input string) []byte {
	length := len(input)
	output := fmt.Sprintf("$%d\r\n%s\r\n", length, input)

	return []byte(output)
}
