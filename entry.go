package lock

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// An entry is a file representing a lock or lock request item

const (
	requestFileType = ".request"
	lockFileType    = ".lock"

	// Default time in seconds to wait between each attempt to acquire the lock
	DefaultPollTime = 30

	// Default maximum time in seconds to wait to acquire the lock before giving up
	DefaultMaxWait = 3600

	// Default name for lock files
	DefaultName = "default_lock"
)

var (
	// Default base directory in which to create locks and lock request files
	DefaultDir = func() string {
		d, _ := os.UserHomeDir()
		return d
	}()

	config = DefaultConfig()
)

// ----------------------------------------------------------------------

type Configuration struct {
	Dir          string
	Name         string
	PollInterval int
	MaxWait      int
}

func DefaultConfig() Configuration {
	return Configuration{
		Dir:          DefaultDir,
		Name:         DefaultName,
		PollInterval: DefaultPollTime,
		MaxWait:      DefaultMaxWait,
	}
}

// Acquire drops a lock request file, and then, when the request is first in queue,
// it will attempt to create the lock file within the time limit configured.
// If successful it will return it to the caller.
func Acquire(cfg *Configuration) (*entry, error) {
	if cfg != nil {
		config = *cfg
	}
	// Create the lock dir if inexistant
	if err := createDir(config.Dir, 0774); err != nil {
		return nil, err
	}

	req, err := createRequest()
	if err != nil {
		return nil, err
	}

	isTimeOut := timedOut(config.MaxWait)

	// Loop until we are first in queue (or we timeout)
	for !req.IsOldest() {
		if isTimeOut() {
			msg := fmt.Sprintf("Timed out (%ds) waiting to acquire lock", config.MaxWait)
			if err := req.Remove(); err != nil {
				msg = fmt.Sprintf(
					" (also failed to remove request %s: %v - please remove manually)",
					req.Path(),
					err,
				)
			}
			return nil, fmt.Errorf(msg)
		}

		time.Sleep(time.Duration(config.PollInterval) * time.Second)
	}

	var lck *entry

	// first in queue, try and get lock
	for !isTimeOut() {
		lck, err = create()
		switch err.(type) {
		case nil:
			// We have the lock:
			// 1. print out the lock token for the client to capture
			// 2. delete the request
			return lck, req.Remove()
		case ExistsErr:
			// wait for the existing lock to be removed
		default:
			if removeErr := req.Remove(); removeErr != nil {
				err = fmt.Errorf(
					"Error creating lock %v, and also failed to remove request %s: %v",
					err,
					req.Path(),
					removeErr,
				)
			}
			return nil, err
		}
	}

	return lck, nil
}

func Delete() error {
	return nil
}

func WithID(id, lockdir string) (*entry, error) {

	return nil, nil
}

func timedOut(max int) func() bool {
	started := time.Now().Unix()
	return func() bool {
		val := (time.Now().Unix() - started) > int64(max)
		return val
	}
}

type entries []entry

func (e *entries) filter(acceptFn func(entry) bool) *entries {
	var ok entries
	for _, ee := range *e {
		if acceptFn(ee) {
			ok = append(ok, ee)
		}
	}
	return &ok
}

func (e *entries) extend(other *entries) *entries {
	for _, item := range *other {
		*e = append(*e, item)
	}
	return e
}

func (e *entries) match(ee entry) *entries {
	var es entries
	for _, item := range *e {
		if item.path == ee.path {
			// ignore the one passed in
			continue
		}
		if item.hasNode(ee.node()) || item.hasName(ee.name()) {
			es = append(es, item)
		}
	}
	return &es
}

func (e *entries) withFiletype(ft string) *entries {
	return e.filter(func(ee entry) bool {
		return ee.filetype() == ft
	})
}

func (e *entries) withName(name string) *entries {
	return e.filter(func(ee entry) bool {
		return ee.name() == name
	})
}

func (e *entries) withNode(nodename string) *entries {
	return e.filter(func(ee entry) bool {
		return ee.node() == nodename
	})
}

func (e *entries) oldest() *entry {
	if e == nil || len(*e) == 0 {
		return nil
	}

	sort.Slice(*e, func(i, j int) bool {
		return (*e)[i].created() < (*e)[j].created()
	})

	return &(*e)[0]
}

// ----------------------------------------------------------------------

type entry struct {
	path string
}

func (e *entry) Remove() error {
	return os.Remove(e.path)
}

func (e *entry) IsOldest() bool {
	vals := _entries(e.dir()).withFiletype(e.filetype())
	found := vals.match(*e)
	// No matches means we are the oldest, or we check if we are
	return len(*found) == 0 || found.oldest().path == e.path
}

func (e *entry) Path() string {
	return e.path
}

func (e *entry) filetype() string {
	return filepath.Ext(e.path)
}

func (e *entry) fields() []string {
	name := strings.Replace(e.base(), fmt.Sprintf(".%s", e.filetype()), "", -1)
	return strings.Split(name, "__")
}

func (e *entry) name() string {
	return e.fields()[0]
}

func (e *entry) node() string {
	return e.fields()[1]
}

func (e *entry) ID() string {
	return e.fields()[2]
}

func (e *entry) created() int {
	when := e.fields()[3]
	value, _ := strconv.Atoi(when)
	return value
}

func (e *entry) hasName(name string) bool {
	return e.name() == name
}

func (e *entry) hasNode(name string) bool {
	return e.node() == name
}

func (e *entry) base() string {
	return filepath.Base(e.path)
}

func (e *entry) dir() string {
	return filepath.Dir(e.path)
}

// create will write to disk the file
func (e *entry) create(contents string) error {
	return os.WriteFile(e.path, []byte(contents), 0774)
}

// ----------------------------------------------------------------------

type ExistsErr error
type TooManyLocksErr error

func createEntryPath(dir, name, filetype string) (string, error) {
	uuid, err := newUUID()
	if err != nil {
		return "", err
	}

	uuid = strings.ReplaceAll(uuid, "-", "")

	name = fmt.Sprintf(
		"%s__%s__%s__%d%s",
		strings.Replace(name, "/", "_", -1),
		currentNode(),
		uuid,
		currentEpoch(),
		filetype,
	)
	return filepath.Join(dir, name), nil
}

func requests(dir string) *entries {
	return _entries(dir).withFiletype(requestFileType)
}

func locks(dir string) *entries {
	return _entries(dir).withFiletype(lockFileType)
}

func _entries(dir string) *entries {
	matches, _ := filepath.Glob(fmt.Sprintf("%s/*", dir))
	var items entries
	for _, item := range matches {
		items = append(items, entry{item})
	}
	return &items
}

func createRequest() (*entry, error) {
	path, err := createEntryPath(config.Dir, config.Name, requestFileType)
	if err != nil {
		return nil, err
	}

	e := entry{path}
	if err := e.create(""); err != nil {
		return nil, fmt.Errorf("failed to create request %s: %v", path, err)
	}

	return &e, nil
}

// createDir creates the given directory with the provided permission
func createDir(dir string, perm os.FileMode) error {
	if err := os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("unable to create lock dir %s: %v", dir, err)
	}

	return nil
}

// create will create the lock file in the given directory with the given name
// unless one or more locks already exist.
func create() (*entry, error) {
	path, err := createEntryPath(config.Dir, config.Name, lockFileType)
	if err != nil {
		return nil, err
	}
	e := entry{path}

	n := len(*locks(config.Dir))
	switch {
	case n == 0:
		// we can make the lock
		if err := e.create(""); err != nil {
			return nil, fmt.Errorf("failed to create request %s: %v", path, err)
		}
	case n <= 2:
		return nil, ExistsErr(fmt.Errorf("%d lock(s) already exist", n))
	default:
		return nil, TooManyLocksErr(fmt.Errorf("%d locks found, expect <= 2", n))
	}

	return &e, nil
}
