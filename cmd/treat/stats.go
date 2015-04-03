// Copyright 2015 TREAT Authors. All rights reserved.
// Use of this source code is governed by a BSD style
// license that can be found in the LICENSE file.

package main

import (
    "log"
    "fmt"
    "strings"
    "github.com/ubccr/treat"
)

func percent(x, y int) (float64) {
    return (float64(x)/float64(y))*float64(100)
}

func Stats(dbpath, gene string) {
    s, err := NewStorage(dbpath)
    if err != nil {
        log.Fatalf("%s", err)
    }

    geneTemplates, err = s.TemplateMap()
    if err != nil {
        log.Fatal(err)
    }

    for g, tmpl := range(geneTemplates) {
        if len(gene) > 0 && g != gene {
            continue
        }

        grandTotal := 0
        nonMutant := 0
        countMap := make(map[string]int)
        totalMap := make(map[string]int)
        err = s.Search(&SearchFields{Gene: g, All: true, EditStop: -1, JuncLen: -1, JuncEnd: -1}, func (key *treat.AlignmentKey, a *treat.Alignment) {
            if a.HasMutation == uint64(0) {
                countMap[key.Sample]++
                nonMutant++
            }

            totalMap[key.Sample]++
            grandTotal++
        })

        if err != nil {
            log.Fatalf("%s", err)
        }

        fmt.Println(strings.Repeat("=", 80))
        fmt.Println(g)
        fmt.Println(strings.Repeat("=", 80))
        fmt.Printf("%20s%11d\n", "Total Alignments:", grandTotal)
        fmt.Printf("%20s%11d\n", "Non-Mutant:", nonMutant)
        fmt.Printf("%20s%11d\n", "Mutant:", grandTotal-nonMutant)
        fmt.Printf("%20s%11s\n", "Edit Base:", string(tmpl.EditBase))
        fmt.Printf("%20s%11d\n", "Alt Templates:", len(tmpl.AltRegion))
        fmt.Printf("%20s%11d\n", "Guide RNAs:", len(tmpl.Grna))
        fmt.Println(strings.Repeat("-", 80))
        fmt.Printf("%-25s%11s%15s%8s%11s%8s\n", "Sample", "Total", "Non-Mutant", "%", "Mutant", "%")
        fmt.Println(strings.Repeat("-", 80))
        for sample, total := range(totalMap) {
            fmt.Printf("%-25s%11d%15d%8.2f%11d%8.2f\n", sample, total, countMap[sample], percent(countMap[sample], total), total-countMap[sample], percent(total-countMap[sample], total))
        }

        fmt.Println()
    }
}
