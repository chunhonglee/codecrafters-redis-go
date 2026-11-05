// Package resp contains the RESP encoders
package resp

import "fmt"

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
