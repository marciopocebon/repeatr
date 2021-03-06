package fspatch

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func prepareFiles(t *testing.T) (string, string, string, string) {
	dir, err := ioutil.TempDir("", "repeatr-fspatch-test")
	if err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(dir, "exist")
	if err := ioutil.WriteFile(file, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	invalid := filepath.Join(dir, "doesnt-exist")

	symlink := filepath.Join(dir, "symlink")
	if err := os.Symlink(file, symlink); err != nil {
		t.Fatal(err)
	}

	return file, invalid, symlink, dir
}

func TestLUtimesNano(t *testing.T) {
	file, invalid, symlink, dir := prepareFiles(t)
	defer os.RemoveAll(dir)

	before, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	if err := LUtimesNano(symlink, time.Unix(0, 0), time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}

	symlinkInfo, err := os.Lstat(symlink)
	if err != nil {
		t.Fatal(err)
	}
	if before.ModTime().Unix() == symlinkInfo.ModTime().Unix() {
		t.Fatal("The modification time of the symlink should be different")
	}

	fileInfo, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if before.ModTime().Unix() != fileInfo.ModTime().Unix() {
		t.Fatal("The modification time of the file should be same")
	}

	if err := LUtimesNano(invalid, time.Unix(0, 0), time.Unix(0, 0)); err == nil {
		t.Fatal("Doesn't return an error on a non-existing file")
	}
}
