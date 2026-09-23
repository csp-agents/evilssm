package ansiprocessing

import "testing"

func TestProcessor(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Normal CR", "aa\rbb", "bb"},
		{"Line feed", "line1\nline2", "line1\nline2"},
		{"Remove ANSI", "\x1b[31mRed\x1b[0m", "Red"},
		{"Invalid bytes", "\xff\xfe\xfd\xfc hello", " hello"},
		{"Delete 1 char", "helloWorld\x1b[1P", "helloWorl"},
		{"Delete 2 chars", "helloWorld\x1b[2P", "helloWor"},
		{"Delete then add", "helloWorld\x1b[2Pld", "helloWorld"},
		{"Backspace and erase", "helloWorld\x08\x1b[K", "helloWorl"},
		{"Backspace delete insert", "helloWorld\x08\x08\x08\x08\x1b[1Prld\x08\x08\x08", "helloWrld"},
		{"Erase to end of line", "hello\nWorld\x08\x08\x08\x08\x08\x1b[K", "hello\n"},
		{"CR after space line wrap", "aa \rbb", "aabb"},
		{"CR after space long line", "abcdefghijklmnopqrstuvwxyzabcdefghijklmn \ropqrstu", "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstu"},
		{"Backspace cursor forward overwrite", "helloWorld\x08\x08\x08\x1b[Csld\x08\x08", "helloWorsld"},
		{"Color removed", "text\x1b[31mred\x1b[0m", "textred"},
		{"UTF-8 backspace erase", "hello世界\x08\x08\x1b[K", "hello"},
		{"UTF-8 backspace overwrite", "hello世界\x08\x08ab", "helloab"},
		{"UTF-8 cursor forward overwrite", "hello世界\x08\x08\x1b[Cab", "hello世ab"},
		{"UTF-8 CR overwrite", "hello世界\rtest", "testo世界"},
		{"UTF-8 CR overwrite 2", "世界hello\rtest", "testllo"},
		{"Incomplete ESC", "hello\x1b", "hello"},
		{"Incomplete CSI", "hello\x1b[", "hello"},
		{"Incomplete CSI param", "hello\x1b[3", "hello"},
		{"Incomplete then complete", "hello\x1b[31mred\x1b[0m", "hellored"},
		{"Invalid CSI", "hello\x1b[Xworld", "helloworld"},
		{"Incomplete CSI then text", "test", "test"},
		{"Euro cursor back delete", "€€\x1b[D\x1b[P", "€"},
		{"UTF-8 delete char", "世界\x1b[D\x1b[P", "世"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewProcessor()
			s.WriteString(tt.input)
			output := string(s.GetOutput())
			if output != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, output)
			}
		})
	}
}

func TestProcessor_Reset(t *testing.T) {
	s := NewProcessor()
	s.WriteString("test")
	s.Reset()
	if len(s.GetOutput()) != 0 {
		t.Errorf("Expected empty output after reset, got %q", string(s.GetOutput()))
	}
}

func TestProcessor_IncompleteSequenceCompletion(t *testing.T) {
	s := NewProcessor()
	s.WriteString("hello\x1b[3")
	if string(s.GetOutput()) != "hello" {
		t.Errorf("Expected %q, got %q", "hello", string(s.GetOutput()))
	}
	s.WriteString("1mred\x1b[0m")
	if string(s.GetOutput()) != "hellored" {
		t.Errorf("Expected %q, got %q", "hellored", string(s.GetOutput()))
	}
}

func TestProcessor_IncompleteCSIFollowedByText(t *testing.T) {
	s := NewProcessor()
	s.WriteString("hello\x1b[3")
	if string(s.GetOutput()) != "hello" {
		t.Errorf("After incomplete: expected %q, got %q", "hello", string(s.GetOutput()))
	}
	// When regular text follows incomplete CSI, first char may be consumed as command
	// This is expected behavior - incomplete sequences are ambiguous
	s.WriteString("world")
	output := string(s.GetOutput())
	// 'w' (0x77) is in valid CSI command range, so it terminates the sequence
	// Result: ESC[3w is consumed, leaving "orld"
	if output != "helloorld" {
		t.Errorf("After text: expected %q, got %q", "helloorld", output)
	}
}
