package pidfile

import (
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/process"
)

func isWrappedNotExist(err error) bool {
	for err != nil {
		if os.IsNotExist(err) {
			return true
		}
		err = errors.Cause(err)
	}
	return false
}

// A PidfileLock is ... TODO: writeme ...
// It is considered valid only while the original process runs.
type PidfileLock interface {
	Pidfile

	Holder() (Pid, error)
	Lock(Pid) error
	Unlock(Pid) error
}

type pidfileLock struct {
	*pidfile
}

var _ PidfileLock = (*pidfileLock)(nil)

func NewLock(path string) (PidfileLock, error) {
	p, err := New(path)
	if err != nil {
		return nil, err
	}

	return &pidfileLock{
		pidfile: p.(*pidfile),
	}, nil
}

// // If err != nil, returns zero-values for pid and ts.
// func (p *pidfileLock) read() (Pid, time.Time, error) {
// 	return Pid(0), time.Time{}, errors.New("not implemented")
// }

// Returns true iff a lock created by the given pid at the given time is still valid; that is, if the same process was
// running when the lock was created.  If the process does not exist, (false, nil) is returned.
func (p *pidfileLock) lockValid(pid Pid, mtime time.Time) (bool, error) {
	info, err := process.NewProcess(int32(pid))
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return false, errors.Wrap(err, "failed to get process information")
	}

	// XXX: The docs for this function say that it returns seconds, but it clearly returns milliseconds.
	procCreateUnixMs, err := info.CreateTime()
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return false, errors.Wrap(err, "failed to get process creation time")
	}

	procCreateTime := time.Unix(procCreateUnixMs/1000, 0)
	return procCreateTime.Before(mtime), nil
}

// Holder returns the pid of the process that holds the lock, or 0 if none exists.  The lock is only considered held if
// the pidfile exists, the process whose pid matches its contents is running, and that process started before the mtime
// of the pidfile.
func (p *pidfileLock) Holder() (Pid, error) {
	lockPid, lockMtime, err := p.pidfile.Read()
	if err != nil {
		if isWrappedNotExist(err) {
			return Pid(0), nil
		}
		return Pid(0), errors.Wrap(err, "failed to read pidfile")
	}

	ok, err := p.lockValid(lockPid, lockMtime)
	if err != nil {
		return Pid(0), errors.Wrap(err, "failed to validate lock")
	}

	if !ok {
		return Pid(0), nil
	}
	return lockPid, nil
}

// Lock atomically creates the pidfile and writes the given pid to it.  If any process currently holds the lock, Lock
// will return an error.  If pid is 0, the pid of the current process is used.
func (p *pidfileLock) Lock(pid Pid) error {
	// XXX: What about the case where the file does exist but the lock is not valid?
	// TODO: Is it worth special handling of an operation that fails because the lock is already held?

	if pid == 0 {
		pid = Pid(os.Getpid())
	}

	lockPid, err := p.Holder()
	if err != nil {
		return errors.Wrap(err, "failed to examine existing lock")
	}
	if lockPid != Pid(0) {
		return os.ErrExist
	}

	if err := p.Write(pid); err != nil {
		return errors.Wrap(err, "failed to write pidfile")
	}

	return nil
}

// Unlock releases the lock.  If the lock is not held by a process with the given pid, Unlock will return an error.  If
// pid is 0, the pid of the current process is used.
func (p *pidfileLock) Unlock(pid Pid) error {
	if pid == 0 {
		pid = Pid(os.Getpid())
	}

	lockPid, lockMtime, err := p.Read()
	if err != nil {
		if isWrappedNotExist(err) {
			return os.ErrNotExist
		}
		return errors.Wrap(err, "failed to read pid")
	}

	ok, err := p.lockValid(lockPid, lockMtime)
	if err != nil {
		return errors.Wrap(err, "failed to validate lock")
	}

	if !ok {
		return os.ErrNotExist
	}

	if lockPid != pid {
		return fmt.Errorf("pidfile is held by %d; lock cannot be released by %d", lockPid, pid)
	}

	if err := os.Remove(p.path); err != nil {
		return errors.Wrap(err, "failed to remove pidfile")
	}

	return nil
}
