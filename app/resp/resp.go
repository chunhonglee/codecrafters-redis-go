// Package resp contains the RESP encoders
package resp

import (
	"fmt"
	"strings"
)

func EncodeSimpleString(s string) []byte {
	return fmt.Appendf(nil, "+%s\r\n", s)
}

func EncodeBulkString(s string) []byte {
	n := len(s)
	return fmt.Appendf(nil, "$%d\r\n%s\r\n", n, s)
}

func EncodeNullBulkString() []byte {
	return fmt.Appendf(nil, "$-1\r\n")
}

func EncodeInteger(i int) []byte {
	return fmt.Appendf(nil, ":%d\r\n", i)
}

func EncodeArray(arr []string) []byte {
	n := len(arr)
	var output strings.Builder
	fmt.Fprintf(&output, "*%d\r\n", n)
	for _, element := range arr {
		length := len(element)
		fmt.Fprintf(&output, "$%d\r\n%s\r\n", length, element)
	}
	return []byte(output.String())
}

func EncodeNullArray() []byte {
	return fmt.Appendf(nil, "*-1\r\n")
}
