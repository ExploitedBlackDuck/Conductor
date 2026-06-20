package domain

import "testing"

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
