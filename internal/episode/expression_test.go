package episode

import "testing"

func TestParseAndResolve(t *testing.T) {
	eps := []Episode{
		{ID: "e1", Number: "1", SortKey: 1.0},
		{ID: "e2", Number: "2", SortKey: 2.0},
		{ID: "e3", Number: "3", SortKey: 3.0},
		{ID: "e4", Number: "4", SortKey: 4.0},
		{ID: "e5", Number: "5", SortKey: 5.0},
		{ID: "e7", Number: "7", SortKey: 7.0},
		{ID: "e10", Number: "10", SortKey: 10.0},
		{ID: "e11", Number: "11", SortKey: 11.0},
		{ID: "e12", Number: "12", SortKey: 12.0},
		{ID: "e7_5", Number: "7.5", SortKey: 7.5},
	}

	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"single", "1", []string{"1"}, false},
		{"range", "1-3", []string{"1", "2", "3"}, false},
		{"comma", "1,3,5", []string{"1", "3", "5"}, false},
		{"mixed", "1-3,7,10-12", []string{"1", "2", "3", "7", "10", "11", "12"}, false},
		{"all", "all", []string{"1", "2", "3", "4", "5", "7", "10", "11", "12", "7.5"}, false},
		{"whitespace", " 1 , 3-5 ", []string{"1", "3", "4", "5"}, false},
		{"decimals", "7.5", []string{"7.5"}, false},
		{"empty", "", nil, true},
		{"reverse_range", "4-1", nil, true},
		{"double_comma", "1,,3", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Parser{}
			req, err := p.Parse(tt.input)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr {
				return
			}

			got := Resolve(req, eps)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Number != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i].Number, tt.want[i])
				}
			}
		})
	}
}