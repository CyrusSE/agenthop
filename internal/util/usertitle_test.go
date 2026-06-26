package util

import "testing"

func TestUserTitleFromText(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{
			in:   "<command-message>pro-pentest-operator</command-message>\n<command-name>/pro-pentest-operator</command-name>",
			want: "/pro-pentest-operator",
			ok:   true,
		},
		{in: "<command-name>/model</command-name>", want: "", ok: false},
		{in: "<command-name>/pro-pentest-operator</command-name>", want: "/pro-pentest-operator", ok: true},
		{in: "<local-command-caveat>Caveat</local-command-caveat>", want: "", ok: false},
		{in: "hello", want: "hello", ok: true},
		{in: "<user_query>fix auth</user_query>", want: "", ok: true}, // skipped entirely
	}
	for _, tc := range tests {
		got, ok := UserTitleFromText(tc.in)
		if tc.in == "<user_query>fix auth</user_query>" {
			if ok {
				t.Fatalf("expected skip for cursor wrapper")
			}
			continue
		}
		if ok != tc.ok || got != tc.want {
			t.Fatalf("UserTitleFromText(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestTitlePickerPrefersPlain(t *testing.T) {
	p := NewTitlePicker(80)
	p.Note("<command-name>/model</command-name>")
	p.Note("<command-name>/brainstorming</command-name>")
	p.Note("real user question here")
	if got := p.Title(); got != "real user question here" {
		t.Fatalf("Title() = %q", got)
	}
}
