package desktop

import "testing"

func TestShouldUseUI(t *testing.T) {
	cases := []struct {
		noui bool
		goos, display string
		want          bool
	}{
		{true, "windows", "", false},
		{false, "windows", "", true},
		{false, "linux", ":0", true},
		{false, "linux", "", false},
		{true, "linux", ":0", false},
		{false, "darwin", "", true},
		{true, "darwin", "", false},
	}
	for _, c := range cases {
		got := ShouldUseUI(c.noui, c.goos, c.display)
		if got != c.want {
			t.Fatalf("%+v got %v", c, got)
		}
	}
}
