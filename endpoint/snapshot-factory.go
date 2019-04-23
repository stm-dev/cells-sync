package endpoint

import (
	"sync"

	"github.com/pydio/cells/common/sync/model"
)

type SnapshotFactory struct {
	sync.Mutex
	snaps map[string]model.Snapshoter
}

func NewSnapshotFactory() model.SnapshotFactory {
	return &SnapshotFactory{
		snaps: make(map[string]model.Snapshoter),
	}
}

func (f *SnapshotFactory) Load(name string) (model.Snapshoter, error) {
	f.Lock()
	defer f.Unlock()
	if s, ok := f.snaps[name]; ok {
		return s, nil
	}
	s, e := NewSnapshot(name)
	if e != nil {
		return nil, e
	}
	f.snaps[name] = s
	return s, nil
}