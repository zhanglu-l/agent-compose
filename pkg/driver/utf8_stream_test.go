package driver

import (
	"testing"
	"unicode/utf8"
)

func TestUTF8StreamDecoderPreservesSplitRunes(t *testing.T) {
	decoder := &utf8StreamDecoder{}
	data := []byte("A中🙂B")
	fragments := [][]byte{
		data[:2],
		data[2:4],
		data[4:7],
		data[7:],
	}

	var got string
	for _, fragment := range fragments {
		chunk := decoder.Write(fragment)
		if !utf8.ValidString(chunk) {
			t.Fatalf("Write(%x) returned invalid UTF-8 %q", fragment, chunk)
		}
		got += chunk
	}
	got += decoder.Finish()
	if got != "A中🙂B" {
		t.Fatalf("decoded text = %q, want %q", got, "A中🙂B")
	}
}

func TestUTF8StreamDecoderReplacesInvalidAndIncompleteInput(t *testing.T) {
	decoder := &utf8StreamDecoder{}

	if got := decoder.Write([]byte{'o', 'k', 0xff}); got != "ok\uFFFD" {
		t.Fatalf("invalid byte decoded as %q, want %q", got, "ok\uFFFD")
	}
	if got := decoder.Write([]byte{0xe4, 0xb8}); got != "" {
		t.Fatalf("incomplete rune decoded before Finish as %q", got)
	}
	if got := decoder.Finish(); got != "\uFFFD" {
		t.Fatalf("incomplete rune Finish() = %q, want %q", got, "\uFFFD")
	}
	if got := decoder.Finish(); got != "" {
		t.Fatalf("second Finish() = %q, want empty", got)
	}
}
