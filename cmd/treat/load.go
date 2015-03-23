package main

import (
    "os"
    "log"
    "strings"
    "fmt"
    "regexp"
    "strconv"
    "path/filepath"
    "encoding/binary"
    "github.com/aebruno/gofasta"
    "github.com/ubccr/treat"
    "github.com/boltdb/bolt"
)

type LoadOptions struct {
    Gene          string
    EditBase      string
    TemplatePath  string
    FragmentPath  []string
    Primer5       int
    Primer3       int
    Replicate     int
    Norm          float64
    GrnaPath      string
}

var readsPattern = regexp.MustCompile(`\s*merge_count=(\d+)\s*`)

func MergeCount(rec *gofasta.SeqRecord) (uint64) {
    mergeCount := 1
    matches := readsPattern.FindStringSubmatch(rec.Id)
    if len(matches) == 2 {
        count, err := strconv.Atoi(matches[1])
        if err == nil {
            mergeCount = count
        }
    }

    return uint64(mergeCount)
}

func TotalReads(path string) (uint64) {
    total := uint64(0)

    f, err := os.Open(path)
    if err != nil {
        log.Fatal(err)
    }
    defer f.Close()

    for rec := range gofasta.SimpleParser(f) {
        total += MergeCount(rec)
    }

    return total
}

func Load(dbpath string, options *LoadOptions) {
    if len(options.Gene) == 0 {
        log.Fatalln("ERROR Gene name is required")
    }
    if len(options.TemplatePath) == 0 {
        log.Fatalln("ERROR Please provide path to templates file")
    }
    if options.FragmentPath == nil || len(options.FragmentPath) == 0 {
        log.Fatalln("ERROR Please provide 1 or more fragment files to load")
    }
    if len(options.EditBase) != 1 {
        log.Fatalln("ERROR Please provide the edit base")
    }

    options.Gene = strings.Replace(options.Gene, " ", "_", -1)

    grna := make([]*treat.Grna, 0)
    if len(options.GrnaPath) != 0 {
        g, err := treat.GrnaFromFile(options.GrnaPath)
        if err != nil {
            log.Fatalln("ERROR parsing grna file: %s", err)
        }
        grna = g
    }

    tmpl, err := treat.NewTemplateFromFasta(options.TemplatePath, treat.FORWARD, rune(options.EditBase[0]))
    if err != nil {
        log.Fatalln(err)
    }

    tmpl.Grna = grna
    tmpl.Primer3 = options.Primer3
    tmpl.Primer5 = options.Primer5

    // Normalize read counts
    if options.Norm == 0 {
        total := uint64(0)
        for _,path := range(options.FragmentPath) {
            total += TotalReads(path)
        }
        options.Norm = float64(total) / float64(len(options.FragmentPath))
        log.Printf("Total reads across all samples: %d", total)
        log.Printf("Normalizing to average read count:: %.4f", options.Norm)
    }

    db, err := bolt.Open(dbpath, 0644, nil)
    if err != nil {
        log.Fatal(err)
    }

    err = db.Update(func(tx *bolt.Tx) error {
        _, err := tx.CreateBucketIfNotExists([]byte(BUCKET_ALIGNMENTS))
        if err != nil {
            return err
        }

        _, err = tx.CreateBucketIfNotExists([]byte(BUCKET_FRAGMENTS))
        if err != nil {
            return err
        }

        b, err := tx.CreateBucketIfNotExists([]byte(BUCKET_TEMPLATES))
        if err != nil {
            return err
        }

        data, err := tmpl.MarshalBytes()
        if err != nil {
            return err
        }

        err = b.Put([]byte(options.Gene), data)
        return err
    })

    if err != nil {
        log.Fatal(err)
    }

    for _,path := range(options.FragmentPath) {
        log.Printf("Computing total read count for file: %s", path)
        total := TotalReads(path)
        scale := options.Norm / float64(total)
        log.Printf("Total reads for file: %d", total)
        log.Printf("Normalized scaling factor: %.4f", scale)

        f, err := os.Open(path)
        if err != nil {
            log.Fatal(err)
        }
        defer f.Close()

        fname := filepath.Base(path)
        sample := fname[:len(fname)-len(filepath.Ext(path))]
        sample = strings.Replace(sample, " ", "_", -1)

        akey := &treat.AlignmentKey{options.Gene, sample}
        key, err := akey.MarshalBinary()
        if err != nil {
            log.Fatal(err)
        }

        err = db.Update(func(tx *bolt.Tx) error {
            b := tx.Bucket([]byte(BUCKET_ALIGNMENTS))
            _, err := b.CreateBucket(key)
            if err != nil {
                return fmt.Errorf("Data already exists for gene %s and sample %s. Please delete database and reload (error: %s)", akey.Gene, akey.Sample, err)
            }

            b = tx.Bucket([]byte(BUCKET_FRAGMENTS))
            _, err = b.CreateBucket(key)
            if err != nil {
                return fmt.Errorf("Data already exists for gene %s and sample %s. Please delete database and reload (error: %s)", akey.Gene, akey.Sample, err)
            }

            return nil
        })

        if err != nil {
            log.Fatalf("ERROR %s", err)
        }

        var tx *bolt.Tx
        var alnBucket *bolt.Bucket
        var fragBucket *bolt.Bucket
        count := 0

        log.Printf("Processing fragments for sample name : %s", sample)
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
                alnBucket = tx.Bucket([]byte(BUCKET_ALIGNMENTS)).Bucket(key)
                fragBucket = tx.Bucket([]byte(BUCKET_FRAGMENTS)).Bucket(key)
            }

            mergeCount := MergeCount(rec)
            norm := scale * float64(mergeCount)

            frag := treat.NewFragment(rec.Id, rec.Seq, treat.FORWARD, mergeCount, norm, rune(options.EditBase[0]))
            aln := treat.NewAlignment(frag, tmpl)

            id, _ := alnBucket.NextSequence()
            kbytes := make([]byte, 8)
            binary.BigEndian.PutUint64(kbytes, id)

            data, err := aln.MarshalBinary()
            if err != nil {
                log.Fatal(err)
            }

            err = alnBucket.Put(kbytes, data)
            if err != nil {
                log.Fatal(err)
            }

            data, err = frag.MarshalBytes()
            if err != nil {
                log.Fatal(err)
            }

            err = fragBucket.Put(kbytes, data)
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

        log.Printf("Loaded %d fragment sequences for sample %s", count, sample)
    }
}
