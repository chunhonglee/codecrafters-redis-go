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

func EncodeNestedArray(elements []any) []byte {
	var sb strings.Builder
	fmt.Fprintf(&sb, "*%d\r\n", len(elements))

	for _, el := range elements {
		switch v := el.(type) {
		case string:
			fmt.Fprintf(&sb, "$%d\r\n%s\r\n", len(v), v)
		case []string:
			sb.Write(EncodeArray(v))
		case []any:
			sb.Write(EncodeNestedArray(v))
		case nil:
			sb.WriteString("$-1\r\n")
		}
	}

	return []byte(sb.String())
}
