package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/codecrafters-io/redis-starter-go/app/server"
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
	defer func() {
		err := l.Close()
		if err != nil {
			fmt.Println("Error closing listener")
		}
	}()
	s := server.New()

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		go handleConnection(s, conn)
	}
}

func handleConnection(s *server.Server, conn net.Conn) {
	buf := make([]byte, 1024)
	for {
		_, err := conn.Read(buf)
		if err != nil {
			break
		}

		inputs := parseInput(string(buf))
		processInput(s, conn, inputs)
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

func processInput(s *server.Server, conn net.Conn, inputs []string) {
	n := len(inputs)
	if n < 1 {
		return
	}
	command := strings.ToUpper(inputs[0])

	commandFunction, ok := s.Cmds[command]
	if !ok {
		fmt.Println("Invalid command")
		return
	}
	commandFunction(conn, inputs)
}
