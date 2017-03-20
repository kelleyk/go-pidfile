package pidfile

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Make a temporary file, remove it, and return it's path with the hopes that
// no one else create a file with that name.
func tempfilename(t *testing.T) string {
	file, err := ioutil.TempFile("", "pidfile-test")
	if err != nil {
		t.Fatal(err)
	}

	err = file.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = os.Remove(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	return file.Name()
}

func TestGetPath(t *testing.T) {
	pidfilePath := tempfilename(t)
	defer func() {
		_ = os.Remove(pidfilePath)
	}()

	pidfile, err := New(pidfilePath)
	assert.Nil(t, err)

	if a := pidfile.Path(); a != pidfilePath {
		t.Fatalf("was expecting %s but got %s", pidfilePath, a)
	}
}

func TestSimple(t *testing.T) {
	pidfilePath := tempfilename(t)
	defer func() {
		_ = os.Remove(pidfilePath)
	}()

	pidfile, err := New(pidfilePath)
	assert.Nil(t, err)

	err = pidfile.Write(0)
	assert.Nil(t, err)

	p, err := pidfile.Read()
	assert.Nil(t, err)
	assert.Equal(t, Pid(os.Getpid()), p)
}

func TestMakesDirectories(t *testing.T) {
	dir := tempfilename(t)
	defer func() {
		_ = os.RemoveAll(dir)
	}()
	pidfilePath := filepath.Join(dir, "pidfile")
	pidfile, err := New(pidfilePath)

	err = pidfile.Write(0)
	assert.Nil(t, err)

	p, err := pidfile.Read()
	assert.Nil(t, err)
	assert.Equal(t, Pid(os.Getpid()), p)
}
