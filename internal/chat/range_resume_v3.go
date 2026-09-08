package chat

import (
	"encoding/json"
	"os"
	"sort"
)

type ByteRange struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}
type ResumeStateV3 struct {
	TransferID, PeerDeviceID, TargetPath, SHA256 string
	FileSize, SourceMtime                        int64
	Completed                                    []ByteRange
	SessionID                                    string
	UpdatedAt                                    int64
}

func MergeRanges(in []ByteRange) []ByteRange {
	if len(in) == 0 {
		return nil
	}
	a := append([]ByteRange(nil), in...)
	sort.Slice(a, func(i, j int) bool { return a[i].Start < a[j].Start })
	out := a[:1]
	for _, r := range a[1:] {
		last := &out[len(out)-1]
		if r.Start <= last.End {
			if r.End > last.End {
				last.End = r.End
			}
		} else {
			out = append(out, r)
		}
	}
	return out
}
func MissingRanges(size int64, done []ByteRange) []ByteRange {
	done = MergeRanges(done)
	var out []ByteRange
	pos := int64(0)
	for _, r := range done {
		if r.Start > pos {
			out = append(out, ByteRange{pos, r.Start})
		}
		if r.End > pos {
			pos = r.End
		}
	}
	if pos < size {
		out = append(out, ByteRange{pos, size})
	}
	return out
}
func SaveResumeV3(path string, s ResumeStateV3) error {
	b, e := json.Marshal(s)
	if e != nil {
		return e
	}
	tmp := path + ".tmp"
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func LoadResumeV3(path string) (ResumeStateV3, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return ResumeStateV3{}, e
	}
	var s ResumeStateV3
	e = json.Unmarshal(b, &s)
	return s, e
}
