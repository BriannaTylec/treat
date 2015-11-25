// Copyright 2015 TREAT Authors. All rights reserved.
//
// This file is part of TREAT.
//
// TREAT is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// TREAT is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with TREAT.  If not, see <http://www.gnu.org/licenses/>.

package treat

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var BASE_COMP = map[byte]byte{
	[]byte("A")[0]: []byte("T")[0],
	[]byte("C")[0]: []byte("G")[0],
	[]byte("G")[0]: []byte("C")[0],
	[]byte("T")[0]: []byte("A")[0],
	[]byte("N")[0]: []byte("N")[0],
}

// fastx_collapser will output fasta headers in the format [id]-[count]
//  for example:
//      > 132-2082
//
// This regex will parse any fasta header (including those from
// fastx_collapser) where the collapsed read count is found at the end of the
// string separated by either a '-' or an '_'. For example, all these
// are acceptable and the collapsed count would = 2082:
//     > SAMPLE1_GENE_123432_2082
//     > 132-2082
//     > GENE-88772-2082
var fastxPattern = regexp.MustCompile(`.+[_\-](\d+)$`)

type Fragment struct {
	Name      string
	ReadCount uint32
	Norm      float64
	Bases     string
	EditBase  rune
	EditSite  []BaseCountType
}

// From: http://stackoverflow.com/a/10030772
func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func parseMergeCount(recId string) uint32 {
	// First try and parse fastx-collapser header lines
	matches := fastxPattern.FindStringSubmatch(recId)
	if len(matches) == 2 {
		count, err := strconv.Atoi(matches[1])
		if err != nil {
			return uint32(1)
		}

		return uint32(count)
	}

	return uint32(1)
}

func NewFragment(name, seq string, orientation OrientationType, base rune) *Fragment {
	base = unicode.ToUpper(base)

	// Ensure all sequences are in forward 5' -> 3' orientation
	if orientation == REVERSE {
		seq = reverse(seq)
	}

	seq = strings.ToUpper(seq)
	base3 := strings.Replace(seq, string(base), "", -1)
	n := len(base3) + 1
	editSite := make([]BaseCountType, n)

	baseCount := BaseCountType(0)
	index := 0
	procBases := func(r rune) rune {
		if r != base {
			editSite[index] = baseCount
			baseCount = 0
			index++
			return r
		}
		baseCount++
		return -1
	}
	bases := strings.Map(procBases, seq)
	editSite[index] = baseCount
	reads := parseMergeCount(name)

	return &Fragment{Name: name, ReadCount: reads, Bases: bases, EditBase: base, EditSite: editSite}
}

func (f *Fragment) ToFasta() string {
	var buf bytes.Buffer
	buf.WriteString(">")
	buf.WriteString(f.Name)
	buf.WriteString("\n")
	buf.WriteString(f.String())
	return buf.String()
}

func (f *Fragment) String() string {
	var buf bytes.Buffer

	for i, b := range f.EditSite {
		buf.WriteString(strings.Repeat(string(f.EditBase), int(b)))
		if i < len(f.Bases) {
			buf.WriteString(string(f.Bases[i]))
		}
	}

	return buf.String()
}

func (f *Fragment) UnmarshalBytes(data []byte) error {
	buf := bytes.NewReader(data)
	dec := gob.NewDecoder(buf)
	err := dec.Decode(f)
	if err != nil {
		return err
	}

	return nil
}

func (f *Fragment) MarshalBytes() ([]byte, error) {
	data := new(bytes.Buffer)
	enc := gob.NewEncoder(data)
	err := enc.Encode(f)
	if err != nil {
		return nil, err
	}

	return data.Bytes(), nil
}
