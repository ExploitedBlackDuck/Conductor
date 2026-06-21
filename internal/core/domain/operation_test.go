package domain

import "testing"

func TestServerSideEligible(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  Endpoint
		dst  Endpoint
		want bool
	}{
		{"same remote", Endpoint{Remote: "s3", Path: "a"}, Endpoint{Remote: "s3", Path: "b"}, true},
		{"different remotes", Endpoint{Remote: "s3", Path: "a"}, Endpoint{Remote: "b2", Path: "b"}, false},
		{"local source", Endpoint{Path: "/local"}, Endpoint{Remote: "s3", Path: "b"}, false},
		{"both local", Endpoint{Path: "/a"}, Endpoint{Path: "/b"}, false},
	}
	for _, c := range cases {
		if got := ServerSideEligible(c.src, c.dst); got != c.want {
			t.Errorf("%s: ServerSideEligible = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestParseEndpointRoundTrip(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want Endpoint
	}{
		{"s3:bucket/path", Endpoint{Remote: "s3", Path: "bucket/path"}},
		{"gdrive:", Endpoint{Remote: "gdrive", Path: ""}},
		{"/local/backup", Endpoint{Path: "/local/backup"}},
		{"relative/dir", Endpoint{Path: "relative/dir"}},
		{"/odd/path:with-colon", Endpoint{Path: "/odd/path:with-colon"}},
		{"", Endpoint{}},
	}
	for _, c := range cases {
		got := ParseEndpoint(c.in)
		if got != c.want {
			t.Errorf("ParseEndpoint(%q) = %+v, want %+v", c.in, got, c.want)
		}
		// A remote-qualified endpoint must render back to its input.
		if got.Remote != "" && got.String() != c.in {
			t.Errorf("round-trip %q: String() = %q", c.in, got.String())
		}
	}
}
