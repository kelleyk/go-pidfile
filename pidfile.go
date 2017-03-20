// Package pidfile manages pid files.
package pidfile

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/facebookgo/atomicfile"
	"github.com/pkg/errors"
)

type Pid int32 // __S32_TYPE

type Pidfile interface {
	Path() string
	Write(Pid) error
	Read() (Pid, error)
}

type pidfile struct {
	path string
}

var _ Pidfile = (*pidfile)(nil)

// New returns a Pidfile that can be used to inspect and manage the file at the given path.
func New(path string) (Pidfile, error) {
	return &pidfile{
		path: path,
	}, nil
}

func (p *pidfile) Path() string {
	return p.path
}

// Write the pidfile.  If pid is 0, the pid of the current process is used instead.
func (p *pidfile) Write(pid Pid) error {
	if pid == 0 {
		pid = Pid(os.Getpid())
	}

	if err := os.MkdirAll(filepath.Dir(p.path), os.FileMode(0755)); err != nil {
		return errors.Wrapf(err, "failed to create parent directories of pidfile: %v", p.path)
	}

	f, err := atomicfile.New(p.path, os.FileMode(0644))
	if err != nil {
		return errors.Wrapf(err, "error opening pidfile: %v", p.path)
	}

	// If we don't make it to the graceful Close below, throw out anything we managed to get on disk.
	defer func() {
		_ = f.Abort()
	}()

	if _, err := fmt.Fprintf(f, "%d", os.Getpid()); err != nil {
		return errors.Wrapf(err, "failed to write pid to pidfile: %v", p.path)
	}

	if err := f.Close(); err != nil {
		return errors.Wrapf(err, "failed to close pidfile: %v", p.path)
	}
	return nil
}

// Read the pidfile.
func (p *pidfile) Read() (Pid, error) {
	d, err := ioutil.ReadFile(p.path)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read pidfile: %v", p.path)
	}

	pid, err := strconv.Atoi(string(bytes.TrimSpace(d)))
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse pid from pidfile: %v", p.path)
	}

	return Pid(pid), nil
}
