package locks

import (
	"context"
	"log"
	"sync"
)

type mutexKV struct {
	lock  sync.Mutex
	store map[string]*sync.Mutex
}

func (m *mutexKV) Lock(ctx context.Context, key string) error {
	log.Printf("[DEBUG] Locking %q", key)
	l := m.get(key)
	for locked := false; !locked; {
		if err := ctx.Err(); err != nil {
			return err
		}
		locked = l.TryLock()
	}
	log.Printf("[DEBUG] Locked %q", key)
	return nil
}

func (m *mutexKV) Unlock(key string) {
	log.Printf("[DEBUG] Unlocking %q", key)
	m.get(key).Unlock()
	log.Printf("[DEBUG] Unlocked %q", key)
}

func (m *mutexKV) get(key string) *sync.Mutex {
	m.lock.Lock()
	defer m.lock.Unlock()
	mutex, ok := m.store[key]
	if !ok {
		mutex = &sync.Mutex{}
		m.store[key] = mutex
	}
	return mutex
}

func NewMutexKV() *mutexKV {
	return &mutexKV{
		store: make(map[string]*sync.Mutex),
	}
}

var monoMutexKV = NewMutexKV()

func Lock(ctx context.Context, key string) error {
	return monoMutexKV.Lock(ctx, key)
}

func Unlock(key string) {
	monoMutexKV.Unlock(key)
}
