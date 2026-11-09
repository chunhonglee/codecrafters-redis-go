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

func EncodeSimpleError(err string) []byte {
	return fmt.Appendf(nil, "-ERR %s\r\n", err)
}

// EncodeNestedArray encodes a slice of mixed types (strings, []string, []any)
// into a RESP array, recursively handling nested arrays and bulk strings.
func EncodeNestedArray(elements []any) []byte {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*%d\r\n", len(elements)))

	for _, el := range elements {
		switch v := el.(type) {
		case string:
			// Regular string becomes a Bulk String
			sb.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(v), v))
		case []string:
			// A slice of strings becomes a RESP Array of Bulk Strings
			sb.Write(EncodeArray(v))
		case []any:
			// A nested slice of mixed types calls EncodeNestedArray recursively
			sb.Write(EncodeNestedArray(v))
		case nil:
			// Null bulk string or null element representation if needed
			sb.WriteString("$-1\r\n")
		}
	}

	return []byte(sb.String())
}
