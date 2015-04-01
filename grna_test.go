// Copyright 2015 TREAT Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package treat

import (
    "testing"
)

func TestGrna(t *testing.T) {
    grna, err := GrnaFromFile("examples/rps12-guide-rna.csv")
    if err != nil {
        t.Errorf("%s", err)
    }

    if len(grna) != 14 {
        t.Errorf("Wrong grna size. %d != %d", len(grna), 14)
    }

    for _, g := range(grna) {
        if g.Start == 0 {
            t.Errorf("Wrong grna start")
        }
        if g.End == 0 {
            t.Errorf("Wrong grna end")
        }
    }
}
