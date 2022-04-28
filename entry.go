package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// An entry is a file representing a lock or request item

const (
	requestFileType = ".request"
	lockFileType    = ".lock"
)

// ----------------------------------------------------------------------

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

func (e *entry) filetype() string {
	return filepath.Ext(e.path)
}

func (e *entry) isOldest() bool {
	vals := _entries(e.dir()).withFiletype(e.filetype())
	found := vals.match(*e)
	// No matches means we are the oldest, or we check if we are
	return len(*found) == 0 || found.oldest().path == e.path
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

func (e *entry) id() string {
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

func (e *entry) remove() error {
	return os.Remove(e.path)
}

// ----------------------------------------------------------------------

type lockExistsErr error
type tooManyLocksErr error

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

func createRequest(dir, name string) (*entry, error) {
	path, err := createEntryPath(dir, name, requestFileType)
	if err != nil {
		return nil, err
	}

	e := entry{path}
	if err := e.create(""); err != nil {
		return nil, fmt.Errorf("failed to create request %s: %v", path, err)
	}

	return &e, nil
}

func createLock(dir, name string) (*entry, error) {
	path, err := createEntryPath(dir, name, lockFileType)
	if err != nil {
		return nil, err
	}
	e := entry{path}

	n := len(*locks(dir))
	switch {
	case n == 0:
		// we can make the lock
		if err := e.create(""); err != nil {
			return nil, fmt.Errorf("failed to create request %s: %v", path, err)
		}
	case n <= 2:
		return nil, lockExistsErr(fmt.Errorf("%d lock(s) already exist", n))
	default:
		return nil, tooManyLocksErr(fmt.Errorf("%d locks found, expect <= 2", n))
	}

	return &e, nil
}
