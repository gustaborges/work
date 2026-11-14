// Package jsonx holds small helpers for strict decoding of Work's own files.
package jsonx

import (
	"encoding/json"
	"io"
)

// TrailingData reports whether dec has input left after the value it just
// decoded. Only whitespace may follow a complete document.
//
// Decoder.More is not a substitute at the top level: it reports false before a
// closing '}' or ']', so a stray delimiter after the document would be
// accepted. Token returns io.EOF only when nothing but whitespace remains and
// an error for any other byte, which is exactly the distinction needed.
func TrailingData(dec *json.Decoder) bool {
	_, err := dec.Token()
	return err != io.EOF
}
