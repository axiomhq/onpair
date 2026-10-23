package onpair

import (
	"bytes"
	"math"
	"slices"
	"strings"
	"testing"
)

func TestDecoder(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []string
	}{
		{name: "empty", rows: []string{}},
		{name: "empty rows", rows: []string{"", "", ""}},
		{name: "mixed", rows: []string{"", "alpha", "東京\x00\xff", "alpha", strings.Repeat("longer-token/", 12)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := mustEncode(NewEncoder(WithMaxTokenLength(16)), tc.rows)
			var wire bytes.Buffer
			if _, err := a.WriteTo(&wire); err != nil {
				t.Fatal(err)
			}
			loaded := &Archive{}
			if _, err := loaded.ReadFrom(&wire); err != nil {
				t.Fatal(err)
			}
			for _, archive := range []*Archive{a, loaded} {
				checkDecoder(t, archive, tc.rows)
			}
		})
	}
	checkDecoder(t, gatherEdgeArchive(), []string{"ab", "cdeab"})
	long := strings.Repeat("x", 33)
	checkDecoder(t, &Archive{
		CompressedData: []uint16{0, 0}, StringBoundaries: []int{0, 1, 2},
		Dictionary: []byte(long), TokenBoundaries: []uint32{0, 33},
	}, []string{long, long})
}

func checkDecoder(t *testing.T, a *Archive, rows []string) {
	t.Helper()
	d, err := a.Decoder()
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{-1, len(rows)} {
		if _, err := d.DecodedLen(index); err == nil {
			t.Fatalf("DecodedLen(%d) accepted invalid index", index)
		}
		prefix := []byte("keep")
		got, err := d.AppendRow(prefix, index)
		if err == nil || !bytes.Equal(got, prefix) {
			t.Fatalf("AppendRow(%d) = %q, %v", index, got, err)
		}
	}
	for range 3 {
		t.Run("concurrent", func(t *testing.T) {
			t.Parallel()
			for _, spare := range []int{0, 16} {
				want := "prefix:" + strings.Join(rows, "")
				dst := make([]byte, len("prefix:"), len(want)+spare)
				copy(dst, "prefix:")
				got, err := d.AppendAll(dst)
				if err != nil || string(got) != want {
					t.Fatalf("AppendAll = %q, %v; want %q", got, err, want)
				}
				retained := slices.Clone(got)
				var out []byte
				var selected strings.Builder
				for i := len(rows) - 1; i >= 0; i-- {
					n, err := d.DecodedLen(i)
					if err != nil || n != len(rows[i]) {
						t.Fatalf("DecodedLen(%d) = %d, %v", i, n, err)
					}
					for range 2 {
						out, err = d.AppendRow(out, i)
						if err != nil {
							t.Fatal(err)
						}
						selected.WriteString(rows[i])
					}
				}
				if string(out) != selected.String() || !bytes.Equal(got, retained) {
					t.Fatal("selected decode or retained output changed")
				}
			}
		})
	}
}

func TestDecoderRejectsCorruption(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Archive)
	}{
		{name: "missing boundaries", mutate: func(a *Archive) { a.StringBoundaries = nil }},
		{name: "negative start", mutate: func(a *Archive) { a.StringBoundaries[0] = -1 }},
		{name: "backward row", mutate: func(a *Archive) { a.StringBoundaries[1] = 5 }},
		{name: "uncovered codes", mutate: func(a *Archive) { a.StringBoundaries[2]-- }},
		{name: "bad code", mutate: func(a *Archive) { a.CompressedData[0] = math.MaxUint16 }},
		{name: "backward token", mutate: func(a *Archive) { a.TokenBoundaries[2] = 1 }},
		{name: "token outside dictionary", mutate: func(a *Archive) { a.TokenBoundaries[3] = math.MaxUint32 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := gatherEdgeArchive()
			tc.mutate(a)
			if _, err := a.Decoder(); err == nil {
				t.Fatal("accepted corrupt archive")
			}
		})
	}
	var a *Archive
	if _, err := a.Decoder(); err == nil {
		t.Fatal("accepted nil archive")
	}
}
