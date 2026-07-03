package devspace

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func TestWithAppLockSerializesWriters(t *testing.T) {
	t.Setenv(envHome, t.TempDir())
	var active int32
	var maxActive int32
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- withAppLock(func() error {
				current := atomic.AddInt32(&active, 1)
				for {
					max := atomic.LoadInt32(&maxActive)
					if current <= max || atomic.CompareAndSwapInt32(&maxActive, max, current) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				atomic.AddInt32(&active, -1)
				return nil
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if maxActive != 1 {
		t.Fatalf("max concurrent lock holders = %d, want 1", maxActive)
	}
}

func TestWithAppLockTimesOut(t *testing.T) {
	home := t.TempDir()
	t.Setenv(envHome, home)
	previousTimeout := appLockTimeout
	previousPoll := appLockPoll
	appLockTimeout = 30 * time.Millisecond
	appLockPoll = 5 * time.Millisecond
	t.Cleanup(func() {
		appLockTimeout = previousTimeout
		appLockPoll = previousPoll
	})
	held := flock.New(home + "/.lock")
	if err := held.Lock(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = held.Unlock()
	}()
	err := withAppLock(func() error {
		t.Fatal("lock body should not run while another handle holds the lock")
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "another devspace process holds the lock") {
		t.Fatalf("expected lock contention error, got %v", err)
	}
}
