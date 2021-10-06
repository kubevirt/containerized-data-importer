package ovirtclient

import (
	"sync"
)

// diskWithData adds the ability to store the data directly in the disk for mocking purposes.
type diskWithData struct {
	disk
	lock   *sync.Mutex
	locked bool
	data   []byte
}

func (d *diskWithData) Lock() error {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.locked {
		return newError(EDiskLocked, "disk %s locked", d.id)
	}
	d.locked = true
	return nil
}

func (d *diskWithData) Unlock() {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.locked = false
}
