package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Println("Logs from your program will appear here!")

	// Uncomment the code below to pass the first stage

	l, err := net.Listen("tcp", "0.0.0.0:6379")
	if err != nil {
		fmt.Println("Failed to bind to port 6379")
		os.Exit(1)
	}
	db := make(map[string]string, 16)

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		go handleConnection(conn, &db)

	}
}

func handleConnection(conn net.Conn, db *map[string]string) {
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
			value := inputs[6]
			(*db)[key] = value
			_, err = conn.Write([]byte("+OK\r\n"))
			if err != nil {
				return
			}
		case "GET":
			key := inputs[4]
			value, ok := (*db)[key]

			if !ok {
				_, err = conn.Write([]byte("$-1\r\n"))
				if err != nil {
					return
				}
			}

			output := formatRESP(value)
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
