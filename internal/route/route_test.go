package route

import "testing"

func TestUnwrap(t *testing.T) {
	cases := []struct{ in, want string }{
		{
			"https://www.google.com/url?q=https://dev.workapp.example/x&sa=D",
			"https://dev.workapp.example/x",
		},
		{
			"https://google.com/url?url=https://example.com/doc",
			"https://example.com/doc",
		},
		{
			"https://nam12.safelinks.protection.outlook.com/?url=https%3A%2F%2Fapp.workapp.example%2Fx&data=05",
			"https://app.workapp.example/x",
		},
		// not a redirector: unchanged
		{"https://www.google.com/search?q=hello", "https://www.google.com/search?q=hello"},
		{"https://example.com/url?q=https://evil", "https://example.com/url?q=https://evil"},
		// wrapper with a non-web target: unchanged
		{"https://www.google.com/url?q=javascript:alert(1)", "https://www.google.com/url?q=javascript:alert(1)"},
		{"not a url at all", "not a url at all"},
	}
	for _, c := range cases {
		if got := Unwrap(c.in); got != c.want {
			t.Errorf("Unwrap(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
