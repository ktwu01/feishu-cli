package client

import "testing"

func TestParseFlagItemType(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"", flagItemTypeDefault, false},
		{"default", flagItemTypeDefault, false},
		{"thread", flagItemTypeThread, false},
		{"msg_thread", flagItemTypeMsgThread, false},
		{"unknown", 0, true},
	}
	for _, c := range cases {
		got, err := ParseFlagItemType(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseFlagItemType(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if err == nil && got != c.want {
			t.Errorf("ParseFlagItemType(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseFlagFlagType(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"", flagFlagTypeMessage, false},
		{"message", flagFlagTypeMessage, false},
		{"feed", flagFlagTypeFeed, false},
		{"unknown", 0, true},
	}
	for _, c := range cases {
		got, err := ParseFlagFlagType(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseFlagFlagType(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if err == nil && got != c.want {
			t.Errorf("ParseFlagFlagType(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
