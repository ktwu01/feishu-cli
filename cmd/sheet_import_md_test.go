package cmd

import (
	"reflect"
	"testing"
)

func TestExtractGFMTables_Standard(t *testing.T) {
	md := `# Header

Some prose.

| Name | Age | City |
| ---- | --- | ---- |
| Alice | 30 | NYC |
| Bob | 25 | LA |

More prose.
`
	tables := extractGFMTables(md)
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	want := [][]string{
		{"Name", "Age", "City"},
		{"Alice", "30", "NYC"},
		{"Bob", "25", "LA"},
	}
	if !reflect.DeepEqual(tables[0], want) {
		t.Errorf("table mismatch:\n got: %v\nwant: %v", tables[0], want)
	}
}

func TestExtractGFMTables_NoLeadingTrailingPipe(t *testing.T) {
	md := `Name | Age
--- | ---
Alice | 30
Bob | 25
`
	tables := extractGFMTables(md)
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	want := [][]string{
		{"Name", "Age"},
		{"Alice", "30"},
		{"Bob", "25"},
	}
	if !reflect.DeepEqual(tables[0], want) {
		t.Errorf("table mismatch: got %v want %v", tables[0], want)
	}
}

func TestExtractGFMTables_AlignmentColons(t *testing.T) {
	md := `| L | C | R |
| :--- | :---: | ---: |
| a | b | c |
`
	tables := extractGFMTables(md)
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	if !reflect.DeepEqual(tables[0], [][]string{{"L", "C", "R"}, {"a", "b", "c"}}) {
		t.Errorf("alignment colons should be ignored, got %v", tables[0])
	}
}

func TestExtractGFMTables_EscapedPipe(t *testing.T) {
	md := `| key | value |
| --- | --- |
| price | a \| b |
| range | 1\|2\|3 |
`
	tables := extractGFMTables(md)
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	want := [][]string{
		{"key", "value"},
		{"price", "a | b"},
		{"range", "1|2|3"},
	}
	if !reflect.DeepEqual(tables[0], want) {
		t.Errorf("escaped pipe mismatch: got %v want %v", tables[0], want)
	}
}

func TestExtractGFMTables_Multiple(t *testing.T) {
	md := `# A

| a | b |
| - | - |
| 1 | 2 |

text in between

| x | y |
| - | - |
| 9 | 8 |
| 7 | 6 |
`
	tables := extractGFMTables(md)
	if len(tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(tables))
	}
	if !reflect.DeepEqual(tables[0], [][]string{{"a", "b"}, {"1", "2"}}) {
		t.Errorf("table 0 mismatch: %v", tables[0])
	}
	if !reflect.DeepEqual(tables[1], [][]string{{"x", "y"}, {"9", "8"}, {"7", "6"}}) {
		t.Errorf("table 1 mismatch: %v", tables[1])
	}
}

func TestExtractGFMTables_None(t *testing.T) {
	md := `# Just text

no tables here

just | a single | pipe but no separator line
`
	tables := extractGFMTables(md)
	if len(tables) != 0 {
		t.Fatalf("expected 0 tables, got %d", len(tables))
	}
}

func TestExtractGFMTables_RaggedRowsPad(t *testing.T) {
	md := `| a | b | c |
| - | - | - |
| 1 | 2 |
| 3 | 4 | 5 | 6 |
`
	tables := extractGFMTables(md)
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	want := [][]string{
		{"a", "b", "c"},
		{"1", "2", ""},  // 短行补空
		{"3", "4", "5"}, // 长行截断到 colCount=3
	}
	if !reflect.DeepEqual(tables[0], want) {
		t.Errorf("ragged rows mismatch:\n got: %v\nwant: %v", tables[0], want)
	}
}

func TestExtractGFMTables_EmptyCells(t *testing.T) {
	md := `| a | b | c |
| - | - | - |
|  |  |  |
| x |  | y |
`
	tables := extractGFMTables(md)
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	want := [][]string{
		{"a", "b", "c"},
		{"", "", ""},
		{"x", "", "y"},
	}
	if !reflect.DeepEqual(tables[0], want) {
		t.Errorf("empty cells mismatch: got %v", tables[0])
	}
}

func TestLooksLikeTableLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"only whitespace", "   ", false},
		{"plain text", "hello world", false},
		{"single pipe", "|", true},
		{"with pipe", "a | b", true},
		{"escaped pipe only", `a \| b`, false},
		{"escaped + real pipe", `a \| b | c`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeTableLine(tt.in)
			if got != tt.want {
				t.Errorf("looksLikeTableLine(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsSeparatorLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"standard", "| --- | --- |", true},
		{"with alignment", "| :--- | :---: | ---: |", true},
		{"single col", "|---|", true},
		{"no pipe", "---", true}, // splitTableRow 一行无管道时返回 1 个 cell
		{"data row not separator", "| a | b |", false},
		{"mixed cells", "| --- | a |", false},
		{"all empty cells", "|  |  |", false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSeparatorLine(tt.in)
			if got != tt.want {
				t.Errorf("isSeparatorLine(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsSeparatorCell(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"---", true},
		{"-", true},
		{":---", true},
		{"---:", true},
		{":---:", true},
		{":-:", true},
		{"::", false},   // 没有 -
		{"---a", false}, // 含字母
		{"", false},     // 空
		{":", false},    // 只有冒号
		{"::---", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := isSeparatorCell(tt.in)
			if got != tt.want {
				t.Errorf("isSeparatorCell(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSplitTableRow(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"both pipes", "| a | b | c |", []string{"a", "b", "c"}},
		{"no pipes at boundary", "a | b | c", []string{"a", "b", "c"}},
		{"trim whitespace", "|  a   |   b  |", []string{"a", "b"}},
		{"empty cells", "|  | x |", []string{"", "x"}},
		{"escaped pipe", `| a \| b | c |`, []string{"a | b", "c"}},
		{"multiple escaped", `| 1\|2\|3 | x |`, []string{"1|2|3", "x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitTableRow(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitTableRow(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestPadRow(t *testing.T) {
	tests := []struct {
		name string
		row  []string
		col  int
		want []string
	}{
		{"exact fit", []string{"a", "b", "c"}, 3, []string{"a", "b", "c"}},
		{"pad short", []string{"a"}, 3, []string{"a", "", ""}},
		{"truncate long", []string{"a", "b", "c", "d"}, 2, []string{"a", "b"}},
		{"empty input", nil, 3, []string{"", "", ""}},
		{"zero cols", []string{"a"}, 0, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := padRow(tt.row, tt.col)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("padRow(%v, %d) = %v, want %v", tt.row, tt.col, got, tt.want)
			}
		})
	}
}
