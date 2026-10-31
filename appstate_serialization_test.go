// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/store"
)

type gatedAppState struct {
	store.AppStateStore
	entered, release, changed chan struct{}
}

func (s *gatedAppState) GetAppStateVersion(context.Context, string) (uint64, [128]byte, error) {
	close(s.entered)
	<-s.release
	return 0, [128]byte{}, errors.New("end send sequence")
}
func (s *gatedAppState) DeleteAppStateVersion(context.Context, string) error {
	close(s.changed)
	return errors.New("end fetch sequence")
}
func TestAppStateSendSerializesFetch(t *testing.T) {
	s := &gatedAppState{entered: make(chan struct{}), release: make(chan struct{}), changed: make(chan struct{})}
	c := NewClient(&store.Device{AppState: s}, nil)
	send := make(chan error, 1)
	fetch := make(chan error, 1)
	go func() {
		send <- c.SendAppState(context.Background(), appstate.PatchInfo{Type: appstate.WAPatchRegular})
	}()
	<-s.entered
	go func() { fetch <- c.FetchAppState(context.Background(), appstate.WAPatchRegular, true, false) }()
	select {
	case <-s.changed:
		close(s.release)
		<-send
		<-fetch
		t.Fatal("fetch changed stored state during send version read")
	case <-time.After(100 * time.Millisecond):
	}
	close(s.release)
	<-send
	select {
	case <-fetch:
	case <-time.After(time.Second):
		t.Fatal("fetch remained blocked after send completed")
	}
}
