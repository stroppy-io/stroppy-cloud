package activities

import (
	"bufio"
	"strings"
	"testing"
)

func TestReadLogLineCutsLongLines(t *testing.T) {
	long := strings.Repeat("x", 3<<20)
	br := bufio.NewReaderSize(strings.NewReader("first\r\n"+long+"\nlast"), 64*1024)
	var got []string
	for {
		line, ok, err := readLogLine(br, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		got = append(got, line)
	}
	if len(got) != 3 || got[0] != "first" || got[2] != "last" {
		t.Fatalf("lines = %d, first %q, last %q", len(got), got[0], got[len(got)-1])
	}
	if !strings.HasPrefix(got[1], strings.Repeat("x", 1<<20)+"… [") || !strings.HasSuffix(got[1], " bytes cut]") {
		t.Fatalf("long line kept %d bytes", len(got[1]))
	}
}
