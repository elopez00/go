// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pods_test

import (
	"crypto/md5"
	"fmt"
	"internal/coverage"
	"internal/coverage/pods"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPodCollection(t *testing.T) {
	//testenv.MustHaveGoBuild(t)

	mkdir := func(d string, perm os.FileMode) string {
		dp := filepath.Join(t.TempDir(), d)
		if err := os.Mkdir(dp, perm); err != nil {
			t.Fatal(err)
		}
		return dp
	}

	mkfile := func(d string, fn string) string {
		fp := filepath.Join(d, fn)
		if err := ioutil.WriteFile(fp, []byte("foo"), 0666); err != nil {
			t.Fatal(err)
		}
		return fp
	}

	mkmeta := func(dir string, tag string) string {
		hash := md5.Sum([]byte(tag))
		fn := fmt.Sprintf("%s.%x", coverage.MetaFilePref, hash)
		return mkfile(dir, fn)
	}

	mkcounter := func(dir string, tag string, nt int) string {
		hash := md5.Sum([]byte(tag))
		dummyPid := int(42)
		fn := fmt.Sprintf(coverage.CounterFileTempl, coverage.CounterFilePref, hash, dummyPid, nt)
		return mkfile(dir, fn)
	}

	trim := func(path string) string {
		b := filepath.Base(path)
		d := filepath.Dir(path)
		db := filepath.Base(d)
		return db + "/" + b
	}

	podToString := func(p pods.Pod) string {
		rv := trim(p.MetaFile) + " [\n"
		for k, df := range p.CounterDataFiles {
			rv += trim(df)
			if p.Origins != nil {
				rv += fmt.Sprintf(" o:%d", p.Origins[k])
			}
			rv += "\n"
		}
		return rv + "]"
	}

	// Create a couple of directories.
	o1 := mkdir("o1", 0777)
	o2 := mkdir("o2", 0777)

	// Add some random files (not coverage related)
	mkfile(o1, "blah.txt")
	mkfile(o1, "something.exe")

	// Add a meta-data file with two counter files to first dir.
	mkmeta(o1, "m1")
	mkcounter(o1, "m1", 1)
	mkcounter(o1, "m1", 2)
	mkcounter(o1, "m1", 2)

	// Add a counter file with no associated meta file.
	mkcounter(o1, "orphan", 9)

	// Add a meta-data file with three counter files to second dir.
	mkmeta(o2, "m2")
	mkcounter(o2, "m2", 1)
	mkcounter(o2, "m2", 2)
	mkcounter(o2, "m2", 3)

	// Add a duplicate of the first meta-file and a corresponding
	// counter file to the second dir. This is intended to capture
	// the scenario where we have two different runs of the same
	// coverage-instrumented binary, but with the output files
	// sent to separate directories.
	mkmeta(o2, "m1")
	mkcounter(o2, "m1", 11)

	// Collect pods.
	podlist, err := pods.CollectPods([]string{o1, o2}, true)
	if err != nil {
		t.Fatal(err)
	}

	// Verify pods
	if len(podlist) != 2 {
		t.Fatalf("expected 2 pods got %d pods", len(podlist))
	}

	for k, p := range podlist {
		t.Logf("%d: mf=%s\n", k, p.MetaFile)
	}

	expected := []string{
		`o1/covmeta.ae7be26cdaa742ca148068d5ac90eaca [
o1/covcounters.ae7be26cdaa742ca148068d5ac90eaca.42.1 o:0
o1/covcounters.ae7be26cdaa742ca148068d5ac90eaca.42.2 o:0
o2/covcounters.ae7be26cdaa742ca148068d5ac90eaca.42.11 o:1
]`,
		`o2/covmeta.aaf2f89992379705dac844c0a2a1d45f [
o2/covcounters.aaf2f89992379705dac844c0a2a1d45f.42.1 o:1
o2/covcounters.aaf2f89992379705dac844c0a2a1d45f.42.2 o:1
o2/covcounters.aaf2f89992379705dac844c0a2a1d45f.42.3 o:1
]`,
	}
	for k, exp := range expected {
		got := podToString(podlist[k])
		if exp != got {
			t.Errorf("pod %d: expected:\n%s\ngot:\n%s", k, exp, got)
		}
	}

	// Check handling of bad/unreadable dir.
	if runtime.GOOS == "linux" {
		dbad := "/dev/null"
		_, err = pods.CollectPods([]string{dbad}, true)
		if err == nil {
			t.Errorf("exected error due to unreadable dir")
		}
	}
}
