package devspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

var (
	appLockTimeout = 10 * time.Second
	appLockPoll    = 250 * time.Millisecond
)

// withAppLock serializes mutating CLI operations across processes. It guards
// the read-modify-write span over config.json/state.json/manifest.json, which
// atomicWriteFile alone cannot (each write is atomic; the span is not).
// NOT reentrant: acquire only at an outermost entry point (a command RunE or
// the watch refresh), never inside domain functions.
func withAppLock(fn func() error) error {
	home, err := appHome()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	l := flock.New(filepath.Join(home, ".lock"))
	ctx, cancel := context.WithTimeout(context.Background(), appLockTimeout)
	defer cancel()
	ok, err := l.TryLockContext(ctx, appLockPoll)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("another devspace process holds the lock (%s); retry when it finishes", l.Path())
		}
		return err
	}
	if !ok {
		return fmt.Errorf("another devspace process holds the lock (%s); retry when it finishes", l.Path())
	}
	defer func() {
		_ = l.Unlock()
	}()
	return fn()
}
