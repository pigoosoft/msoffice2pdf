package converter

import "testing"

func TestOFDWorkerArgs(t *testing.T) {
	got := ofdWorkerArgs("C:\\a.ofd", "C:\\a.pdf")
	want := []string{"convert-worker", "--src", "C:\\a.ofd", "--dst", "C:\\a.pdf", "--engine", EngineOFD}
	if len(got) != len(want) {
		t.Fatalf("%q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%q", got)
		}
	}
}
