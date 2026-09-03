package service

import "testing"

func TestParsePreviewRange(t *testing.T) {
	tests := []struct {
		name         string
		header       string
		wantStart    int64
		wantEnd      int64
		wantSuffix   int64
		wantStartSet bool
		wantEndSet   bool
		wantErr      bool
	}{
		{name: "empty", header: ""},
		{name: "open ended", header: "bytes=1024-", wantStart: 1024, wantEnd: -1, wantSuffix: -1, wantStartSet: true},
		{name: "bounded", header: "bytes=10-20", wantStart: 10, wantEnd: 20, wantSuffix: -1, wantStartSet: true, wantEndSet: true},
		{name: "suffix", header: "bytes=-500", wantStart: -1, wantEnd: -1, wantSuffix: 500},
		{name: "multiple", header: "bytes=0-10,20-30", wantErr: true},
		{name: "invalid", header: "bytes=20-10", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parsePreviewRange(test.header)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got == nil {
				if test.header != "" {
					t.Fatal("expected a range")
				}
				return
			}
			if got.start != test.wantStart || got.end != test.wantEnd || got.suffix != test.wantSuffix || got.hasStart != test.wantStartSet || got.hasEnd != test.wantEndSet {
				t.Fatalf("range = %+v", got)
			}
		})
	}
}

func TestResolvePreviewRange(t *testing.T) {
	bounded, err := parsePreviewRange("bytes=10-20")
	if err != nil {
		t.Fatal(err)
	}
	start, end, ok := resolvePreviewRange(bounded, 100)
	if !ok || start != 10 || end != 20 {
		t.Fatalf("resolved bounded range = %d-%d, %t", start, end, ok)
	}

	suffix, err := parsePreviewRange("bytes=-10")
	if err != nil {
		t.Fatal(err)
	}
	start, end, ok = resolvePreviewRange(suffix, 100)
	if !ok || start != 90 || end != 99 {
		t.Fatalf("resolved suffix range = %d-%d, %t", start, end, ok)
	}

	invalid, err := parsePreviewRange("bytes=100-")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok = resolvePreviewRange(invalid, 100); ok {
		t.Fatal("expected an unsatisfiable range")
	}
}
