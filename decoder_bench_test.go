package onpair

import (
	"bytes"
	"testing"
)

func BenchmarkDecoder(b *testing.B) {
	rows := benchLines(b)
	maxLen := 0
	for _, row := range rows {
		maxLen = max(maxLen, len(row))
	}
	a := mustEncode(NewEncoder(WithMaxTokenLength(16)), rows)
	d, err := a.Decoder()
	if err != nil {
		b.Fatal(err)
	}
	want, err := a.AppendAll(nil)
	if err != nil {
		b.Fatal(err)
	}
	got, err := d.AppendAll(nil)
	if err != nil || !bytes.Equal(got, want) {
		b.Fatalf("prepared decode differs: %v", err)
	}
	b.Run("Prepare", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := a.Decoder(); err != nil {
				b.Fatal(err)
			}
		}
	})
	for _, tc := range []struct {
		name string
		row  func([]byte, int) ([]byte, error)
		all  func([]byte) ([]byte, error)
	}{
		{name: "checked", row: a.AppendRow, all: a.AppendAll},
		{name: "prepared", row: d.AppendRow, all: d.AppendAll},
	} {
		b.Run("decoder="+tc.name, func(b *testing.B) {
			b.Run("op=Row", func(b *testing.B) {
				b.ReportAllocs()
				dst := make([]byte, 0, maxLen+decodeSlack)
				i := 0
				for b.Loop() {
					dst, err = tc.row(dst[:0], i%len(rows))
					if err != nil {
						b.Fatal(err)
					}
					i++
				}
			})
			b.Run("op=All", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(want)))
				dst := make([]byte, 0, len(want)+decodeSlack)
				for b.Loop() {
					dst, err = tc.all(dst[:0])
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
