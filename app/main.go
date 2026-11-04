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

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		go handleConnection(conn)

	}
}

func handleConnection(conn net.Conn) {
	buf := make([]byte, 1024)
	for {
		_, err := conn.Read(buf)
		if err != nil {
			break
		}

		inputs := parseRESP(buf)
		command := strings.ToLower(inputs[2])

		switch command {
		case "ping":
			output := formatRESP("+PONG\r\n")
			_, err = conn.Write(output)
			if err != nil {
				return
			}
		case "echo":
			output := formatRESP(inputs[4])
			_, err = conn.Write(output)
			if err != nil {
				return
			}

		}
	}
}

func parseRESP(buf []byte) []string {
	output := strings.Split(string(buf), "\r\n")

	return output
}

func formatRESP(input string) []byte {
	length := len(input)
	output := fmt.Sprintf("%d\r\n%s\r\n", length, input)

	return []byte(output)
}
