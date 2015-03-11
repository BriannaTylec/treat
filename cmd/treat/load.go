package main

import (
    "fmt"
    "os"
    "log"
    "path/filepath"
    "github.com/aebruno/gofasta"
    "github.com/ubccr/treat"
    "github.com/boltdb/bolt"
)

func Load(dbpath, gene, templates string, fragments []string, replicate int, norm float64) {
    if len(gene) == 0 {
        log.Fatalln("ERROR Gene name is required")
    }
    if len(templates) == 0 {
        log.Fatalln("ERROR Please provide path to templates file")
    }
    if fragments == nil || len(fragments) == 0 {
        log.Fatalln("ERROR Please provide 1 or more fragment files to load")
    }

    tmpl, err := treat.NewTemplateFromFasta(templates, treat.FORWARD, 't')
    if err != nil {
        log.Fatalln(err)
    }

    db, err := bolt.Open(dbpath, 0644, nil)
    if err != nil {
        log.Fatal(err)
    }

    err = db.Update(func(tx *bolt.Tx) error {
        _, err := tx.CreateBucketIfNotExists([]byte("alignments"))
        if err != nil {
            return err
        }
        return nil
    })

    if err != nil {
        log.Fatal(err)
    }

    for _,path := range(fragments) {
        f, err := os.Open(path)
        if err != nil {
            log.Fatal(err)
        }

        fname := filepath.Base(path)
        sample := fname[:len(fname)-len(filepath.Ext(path))]

        var tx *bolt.Tx
        count := 0

        for rec := range gofasta.SimpleParser(f) {

            if count % 100 == 0 {
                if count > 0 {
                    if err := tx.Commit(); err != nil {
                        log.Fatal(err)
                    }
                }
                tx, err = db.Begin(true)
                if err != nil {
                    log.Fatal(err)
                }
            }

            frag := treat.NewFragment(rec.Id, rec.Seq, treat.FORWARD, 100, 't')
            aln := treat.NewAlignment(frag, tmpl, 0, 0)


            bucket := tx.Bucket([]byte("alignments"))
            id, _ := bucket.NextSequence()

            key := fmt.Sprintf("%s;%s;%d;%d", gene, sample, replicate, id)

            data, err := aln.Bytes()
            if err != nil {
                log.Fatal(err)
            }

            err = bucket.Put([]byte(key), data)
            if err != nil {
                log.Fatal(err)
            }

            count++
        }

        if count % 100 != 0 {
            if err := tx.Commit(); err != nil {
                log.Fatal(err)
            }
        }
    }
}
