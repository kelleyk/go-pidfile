package pidfile

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// TODO: test what happens when:
//  - we don't have permission to write the pidfile;
//  - we don't have permission to remove the pidfile;
//  - we don't have permission to read the pidfile;
//  - the pidfile contains a negative number;
//  - the pidfile contains something that is not numeric;
//  - when we don't have access to procfs to check the ctime of the process in question;
// - ...

type PidfileLockTestSuite struct {
	suite.Suite

	base        string
	pidfilePath string
	pl          *pidfileLock
}

func TestPidfileLockTestSuite(t *testing.T) {
	suite.Run(t, new(PidfileLockTestSuite))
}

func (suite *PidfileLockTestSuite) SetupTest() {
	t := suite.T()
	var err error

	suite.base, err = ioutil.TempDir("", "pidfile-test")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %v", err)
	}

	suite.pidfilePath = filepath.Join(suite.base, "test.pid")

	pl, err := NewLock(suite.pidfilePath)
	if err != nil {
		t.Fatalf("failed to create PidfileLock: %v", err)
	}
	suite.pl = pl.(*pidfileLock)
}

func (suite *PidfileLockTestSuite) TearDownTest() {
	t := suite.T()

	if err := os.RemoveAll(suite.base); err != nil {
		t.Fatalf("failed to remove temporary directory: %v", err)
	}
}

func (suite *PidfileLockTestSuite) makePidfile(valid bool) {
	t := suite.T()

	if err := ioutil.WriteFile(suite.pidfilePath, []byte(fmt.Sprintf("%d", os.Getpid())), os.FileMode(0644)); err != nil {
		t.Fatalf("failed to write pidfile: %v", err)
	}

	if !valid {
		ts := time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC) // Any date before this process started will be fine.
		if err := os.Chtimes(suite.pidfilePath, ts, ts); err != nil {
			t.Fatalf("failed to set pidfile mtime: %v", err)
		}
	}
}

func (suite *PidfileLockTestSuite) assertPidfile(exists bool) {
	t := suite.T()

	_, err := os.Stat(suite.pidfilePath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	if exists {
		assert.Nil(t, err, "pidfile does not exist when it should")
	} else {
		assert.True(t, os.IsNotExist(err), "pidfile exists when it should not")
	}
}

// If there's no pidfile, Holder should report that nobody holds the lock.
func (suite *PidfileLockTestSuite) TestHolder_NotExist() {
	t := suite.T()

	pid, err := suite.pl.Holder()
	assert.Equal(t, Pid(0), pid)
	assert.Nil(t, err)
}

// If the pidfile exists but its mtime is earlier than the creation time of the process whose pid it contains, it is
// valid and should be ignored.
func (suite *PidfileLockTestSuite) TestHolder_Invalid() {
	t := suite.T()

	suite.makePidfile(false)

	pid, err := suite.pl.Holder()
	assert.Equal(t, Pid(0), pid)
	assert.Nil(t, err)
}

// If the lock is currently held, Holder should succeed.
func (suite *PidfileLockTestSuite) TestHolder_Exist() {
	t := suite.T()

	suite.makePidfile(true)

	pid, err := suite.pl.Holder()
	assert.Equal(t, Pid(os.Getpid()), pid)
	assert.Nil(t, err)
}

// If the pidfile does not exist, we should be able to take the lock.
func (suite *PidfileLockTestSuite) TestLock_NotExist() {
	t := suite.T()

	err := suite.pl.Lock(0)
	assert.Nil(t, err)

	suite.assertPidfile(true)
}

// If the pidfile exists but the lock is not valid, we should be able to take the lock as normal.
func (suite *PidfileLockTestSuite) TestLock_Invalid() {
	t := suite.T()

	suite.makePidfile(false)

	err := suite.pl.Lock(0)
	assert.Nil(t, err)
}

// If the pidfile exists and the lock is valid, we should get an error when we try to take the lock.
func (suite *PidfileLockTestSuite) TestLock_Exist() {
	t := suite.T()

	suite.makePidfile(true)

	err := suite.pl.Lock(0)
	assert.Equal(t, os.ErrExist, err)
}

// If the pidfile does not exist, Unlock should fail.
func (suite *PidfileLockTestSuite) TestUnlock_NotExist() {
	t := suite.T()

	err := suite.pl.Unlock(0)
	assert.Equal(t, os.ErrNotExist, err)
}

// If the lock is invalid, it is already unlocked, so Unlock should fail.
func (suite *PidfileLockTestSuite) TestUnlock_Invalid() {
	t := suite.T()

	suite.makePidfile(false)

	err := suite.pl.Unlock(0)
	assert.Equal(t, os.ErrNotExist, err)

	suite.assertPidfile(true)
}

// If the pid passed to Unlock doesn't match what is in the pidfile, the lock should not be released.
func (suite *PidfileLockTestSuite) TestUnlock_NotOwner() {
	t := suite.T()

	// XXX: We assume that pid 1 has been around for a long time.
	if err := ioutil.WriteFile(suite.pidfilePath, []byte("1"), os.FileMode(0644)); err != nil {
		t.Fatalf("failed to write pidfile: %v", err)
	}

	err := suite.pl.Unlock(0)
	assert.NotNil(t, err)

	suite.assertPidfile(true)
}

// If the pid we pass to Unlock owns the lock, it should be successfully released.
func (suite *PidfileLockTestSuite) TestUnlock_Owner() {
	t := suite.T()

	suite.makePidfile(true)

	err := suite.pl.Unlock(0)
	assert.Nil(t, err)

	suite.assertPidfile(false)
}
