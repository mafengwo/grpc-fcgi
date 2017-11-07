// based on
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errgroup

import (
	"sync"
)

// ErrGroup is a group of functions that will exit on first
// error
type ErrGroup struct {
	sync.Mutex
	numFuncs int
	errChan  chan error
}

// New creeats a new errgroup
func New() *ErrGroup {
	return &ErrGroup{
		errChan: make(chan error),
	}
}

// Wait returns on first error or when all functions finish.
// On error, it does not wait for the other functions, so
// this should only be used if the program will exit.
func (g *ErrGroup) Wait() error {
	g.Lock()
	defer g.Unlock()

	i := 0
	for err := range g.errChan {
		if err != nil {
			return err
		}
		i++
		if i >= g.numFuncs {
			break
		}
	}
	return nil
}

// Go calls the given function in a new goroutine.
// must not be called after wait is called
func (g *ErrGroup) Go(f func() error) {
	g.Lock()
	defer g.Unlock()
	g.numFuncs++
	go func() {
		g.errChan <- f()
	}()
}
