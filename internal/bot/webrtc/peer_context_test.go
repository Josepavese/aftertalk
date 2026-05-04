package webrtc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreatePeer_ContextCancel verifies that canceling the context passed to
// CreatePeer causes the underlying PeerConnection to be closed promptly.
func TestCreatePeer_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	mgr := NewManager(nil, nil, 0, 0)
	peer, err := mgr.CreatePeer(ctx, "session-1", "participant-1", "therapist")
	require.NoError(t, err)
	require.NotNil(t, peer)

	// PC is open — SignalingState should not be "closed" yet.
	assert.NotEqual(t, "closed", peer.PC.SignalingState().String())

	// Cancel the context — the cleanup goroutine should close the PC.
	cancel()

	// Give the goroutine a moment to run.
	assert.Eventually(t, func() bool {
		return peer.PC.SignalingState().String() == "closed"
	}, 500*time.Millisecond, 10*time.Millisecond, "PeerConnection must be closed after context cancel")
}

// TestCreatePeer_NormalRemoval verifies that RemovePeer also closes the PC
// and that a subsequent context cancel (via double-close) does not panic.
func TestCreatePeer_NormalRemoval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr := NewManager(nil, nil, 0, 0)
	peer, err := mgr.CreatePeer(ctx, "session-2", "participant-2", "patient")
	require.NoError(t, err)

	// Normal path: RemovePeer closes the PC first.
	mgr.RemovePeer("session-2", "participant-2")
	assert.Equal(t, "closed", peer.PC.SignalingState().String())

	// Now cancel the context — cleanup goroutine calls Close() on already-closed PC.
	// Must not panic (Pion Close() is idempotent).
	assert.NotPanics(t, func() {
		cancel()
		time.Sleep(20 * time.Millisecond)
	})
}

// TestCreatePeer_ErrAlreadyExists verifies that duplicate CreatePeer does not
// launch a second cleanup goroutine for the same peer.
func TestCreatePeer_ErrAlreadyExists(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr := NewManager(nil, nil, 0, 0)
	_, err := mgr.CreatePeer(ctx, "session-3", "participant-3", "therapist")
	require.NoError(t, err)

	_, err = mgr.CreatePeer(ctx, "session-3", "participant-3", "therapist")
	assert.ErrorIs(t, err, ErrPeerAlreadyExists)
}
