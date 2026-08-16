package serve

import (
	"net"
	"strconv"
	"testing"
)

func TestDisplayURL(t *testing.T) {
	cases := []struct {
		name string
		host string
		port int
		want string
	}{
		{"localhost keeps localhost", "localhost", 4080, "http://localhost:4080"},
		{"loopback IPv4 becomes localhost", "127.0.0.1", 4080, "http://localhost:4080"},
		{"other 127.0.0.0/8 address becomes localhost", "127.0.0.5", 4080, "http://localhost:4080"},
		{"IPv6 loopback gets brackets, not localhost", "::1", 4080, "http://[::1]:4080"},
		{"other IPv6 gets brackets", "2001:db8::1", 4080, "http://[2001:db8::1]:4080"},
		{"IPv4-mapped IPv6 gets brackets, not localhost (parity with Node/Python's syntactic classification)", "::ffff:127.0.0.1", 4080, "http://[::ffff:127.0.0.1]:4080"},
		{"LAN IPv4 is printed as-is", "192.168.3.137", 4080, "http://192.168.3.137:4080"},
		{"wildcard IPv4 is printed as-is", "0.0.0.0", 4080, "http://0.0.0.0:4080"},
		{"non-IP hostname is printed as-is", "example.internal", 4080, "http://example.internal:4080"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DisplayURL(c.host, c.port)
			if got != c.want {
				t.Errorf("DisplayURL(%q, %d) = %q, want %q", c.host, c.port, got, c.want)
			}
		})
	}
}

func TestIsLoopbackHost(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"127.0.0.5", true},
		{"::1", true},
		{"0.0.0.0", false},
		{"192.168.3.137", false},
		{"2001:db8::1", false},
	}
	for _, c := range cases {
		if got := IsLoopbackHost(c.host); got != c.want {
			t.Errorf("IsLoopbackHost(%q) = %v, want %v", c.host, got, c.want)
		}
	}
}

// TestListenOnIPv6Loopback proves the addr-formatting fix by really binding
// a socket, not by reading source: before this ML, Start() built the address
// with fmt.Sprintf("%s:%d", "::1", port), producing the unparseable
// "::1:PORT" ("too many colons in address"). This uses the exact
// construction Start() now uses (net.JoinHostPort) and confirms the bind
// succeeds by connecting to it.
func TestListenOnIPv6Loopback(t *testing.T) {
	addr := net.JoinHostPort("::1", "0") // port 0 = let the OS pick a free port
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Skipf("IPv6 loopback unavailable in this environment: %v", err)
	}
	defer ln.Close()

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", ln.Addr().String(), err)
	}
	port, _ := strconv.Atoi(portStr)

	conn, err := net.Dial("tcp", net.JoinHostPort("::1", portStr))
	if err != nil {
		t.Fatalf("dial ::1:%d: %v", port, err)
	}
	conn.Close()
}
