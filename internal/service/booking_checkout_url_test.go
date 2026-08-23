package service

import "testing"

func TestWithReference(t *testing.T) {
	cases := []struct{ name, raw, want string }{
		{"placeholder is substituted, not appended",
			"https://kalingaspa.com/book?checkout={reference}",
			"https://kalingaspa.com/book?checkout=CS_ABC123"},
		{"placeholder substituted alongside other params",
			"https://kalingaspa.com/book?checkout={reference}&cancelled=1",
			"https://kalingaspa.com/book?checkout=CS_ABC123&cancelled=1"},
		{"appended when no placeholder and no query",
			"https://kalingaspa.com/book",
			"https://kalingaspa.com/book?checkout=CS_ABC123"},
		{"appended when no placeholder but query exists",
			"https://kalingaspa.com/book?cancelled=1",
			"https://kalingaspa.com/book?cancelled=1&checkout=CS_ABC123"},
		{"empty stays empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := withReference(c.raw, "CS_ABC123"); got != c.want {
				t.Fatalf("withReference(%q)\n got %q\nwant %q", c.raw, got, c.want)
			}
		})
	}
}
