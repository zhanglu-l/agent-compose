//go:build cgo

package driver

import "testing"

func TestMicrosandboxExecCollectorMapsStdioStreams(t *testing.T) {
	var streamed []ExecChunk
	collector := &microsandboxExecCollector{stream: func(chunk ExecChunk) {
		streamed = append(streamed, chunk)
	}}
	collector.writeChunk(ExecChunk{Text: "out"})
	collector.writeChunk(ExecChunk{Text: "err", Stream: StdioStderr})

	if collector.stdout.String() != "out" {
		t.Fatalf("stdout = %q", collector.stdout.String())
	}
	if collector.stderr.String() != "err" {
		t.Fatalf("stderr = %q", collector.stderr.String())
	}
	if collector.output.String() != "outerr" {
		t.Fatalf("output = %q", collector.output.String())
	}
	want := []ExecChunk{{Text: "out"}, {Text: "err", Stream: StdioStderr}}
	if len(streamed) != len(want) {
		t.Fatalf("streamed chunks = %#v", streamed)
	}
	for i := range want {
		if streamed[i] != want[i] {
			t.Fatalf("streamed[%d] = %#v, want %#v", i, streamed[i], want[i])
		}
	}
}
