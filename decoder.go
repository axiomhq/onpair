package onpair

import (
	"fmt"
	"math"
)

// Decoder retains decoded offsets for repeated reads without retaining decoded strings.
// It borrows the archive: do not mutate the archive or its backing slices while using it.
// Concurrent reads are safe when each call owns its destination buffer.
type Decoder struct {
	archive *Archive
	ends    []int
}

// Decoder validates the archive and indexes decoded lengths without changing its wire format.
func (a *Archive) Decoder() (*Decoder, error) {
	if a == nil {
		return nil, fmt.Errorf("cannot prepare a nil archive")
	}
	if err := validateArchiveStructure(a); err != nil {
		return nil, fmt.Errorf("prepare decoder: %w", err)
	}
	if a.StringBoundaries[len(a.StringBoundaries)-1] != len(a.CompressedData) {
		return nil, fmt.Errorf("prepare decoder: string boundaries do not cover the code stream")
	}
	d := &Decoder{archive: a, ends: make([]int, a.Rows()+1)}
	for row := range a.Rows() {
		n, err := a.DecodedLen(row)
		if err != nil {
			return nil, fmt.Errorf("prepare decoder: %w", err)
		}
		if n > math.MaxInt-decodeSlack-d.ends[row] {
			return nil, fmt.Errorf("prepare decoder: decoded length overflows at row %d", row)
		}
		d.ends[row+1] = d.ends[row] + n
	}
	return d, nil
}

// DecodedLen returns a row's decoded byte length without scanning its tokens.
func (d *Decoder) DecodedLen(index int) (int, error) {
	if index < 0 || index >= len(d.ends)-1 {
		return 0, fmt.Errorf("index out of bounds: %d", index)
	}
	return d.ends[index+1] - d.ends[index], nil
}

// AppendRow appends one decoded row, reusing dst's capacity when possible.
// Destination storage must not alias the archive; bytes past the result are scratch space.
func (d *Decoder) AppendRow(dst []byte, index int) ([]byte, error) {
	n, err := d.DecodedLen(index)
	if err != nil {
		return dst, err
	}
	if n > math.MaxInt-decodeSlack-len(dst) {
		return dst, fmt.Errorf("appended length overflows")
	}
	a := d.archive
	codes := a.CompressedData[a.StringBoundaries[index]:a.StringBoundaries[index+1]]
	return a.appendTokens(dst, codes, n), nil
}

// AppendAll appends all decoded rows, reusing dst's capacity when possible.
// Destination storage must not alias the archive; bytes past the result are scratch space.
func (d *Decoder) AppendAll(dst []byte) ([]byte, error) {
	n := d.ends[len(d.ends)-1]
	if n > math.MaxInt-decodeSlack-len(dst) {
		return dst, fmt.Errorf("appended length overflows")
	}
	return d.archive.appendTokens(dst, d.archive.CompressedData, n), nil
}
