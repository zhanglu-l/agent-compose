package driver

import (
	"strings"
	"unicode/utf8"
)

const invalidUTF8Replacement = "\uFFFD"

// utf8StreamDecoder turns arbitrarily split byte fragments into valid UTF-8
// text. A suffix that can still become a valid rune is held until the next
// fragment; bytes that can no longer form valid UTF-8 are replaced.
type utf8StreamDecoder struct {
	pending []byte
}

func (d *utf8StreamDecoder) Write(fragment []byte) string {
	if len(fragment) == 0 {
		return ""
	}

	data := fragment
	if len(d.pending) > 0 {
		data = make([]byte, 0, len(d.pending)+len(fragment))
		data = append(data, d.pending...)
		data = append(data, fragment...)
		d.pending = d.pending[:0]
	}

	completeEnd := len(data)
	for offset := 0; offset < len(data); {
		r, size := utf8.DecodeRune(data[offset:])
		if r == utf8.RuneError && size == 1 && !utf8.FullRune(data[offset:]) {
			completeEnd = offset
			d.pending = append(d.pending, data[offset:]...)
			break
		}
		offset += size
	}

	return strings.ToValidUTF8(string(data[:completeEnd]), invalidUTF8Replacement)
}

func (d *utf8StreamDecoder) Finish() string {
	if len(d.pending) == 0 {
		return ""
	}
	text := strings.ToValidUTF8(string(d.pending), invalidUTF8Replacement)
	d.pending = nil
	return text
}
