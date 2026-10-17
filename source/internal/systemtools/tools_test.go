package systemtools

import (
	"context"
	"testing"
)

func TestTenUniqueToolIDs(t *testing.T) {
	if len(Tools) != 10 {
		t.Fatal(len(Tools))
	}
	m := map[string]bool{}
	for _, t1 := range Tools {
		if m[t1.ID] || t1.Name == "" {
			t.Fatal(t1)
		}
		m[t1.ID] = true
		if _, ok := Find(t1.ID); !ok {
			t.Fatal(t1.ID)
		}
	}
	if _, ok := Find("cmd.exe"); ok {
		t.Fatal("arbitrary executable accepted")
	}
}
func TestHostValidation(t *testing.T) {
	for _, s := range []string{"example.com", "localhost", "192.0.2.1", "2001:db8::1", "::1", "a-b.example"} {
		if e := ValidateHost(s); e != nil {
			t.Fatal(s, e)
		}
	}
	for _, s := range []string{"", "-n", "a&whoami", "foo:443", "https://a.com", "a..com", "a;cmd", "a b", "a/com", "a\\com", "%PATH%", "a-.com", "a\nwhoami", "a.com."} {
		if ValidateHost(s) == nil {
			t.Fatal("invalid target accepted", s)
		}
	}
}
func TestUnknownNetworkKind(t *testing.T) {
	if _, e := CheckNetwork(context.Background(), "exec", "localhost"); e == nil {
		t.Fatal("unknown kind accepted")
	}
}
