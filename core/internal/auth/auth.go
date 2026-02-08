// Package auth implements challenge-response authentication over a libp2p stream.
// The initiator sends a random challenge; the responder signs it with their private key;
// the initiator verifies the signature with the expected DID (public key).
package auth

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/SeijiShii/link-self/core/internal/did"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	// ProtocolID is the protocol used for LinkSelf DID authentication.
	ProtocolID = "/linkself/auth/1.0.0"
	// ChallengeSize is the byte length of the random challenge.
	ChallengeSize = 32
)

var (
	// ErrAuthFailed is returned when challenge-response verification fails.
	ErrAuthFailed = errors.New("authentication failed")
	// ErrWrongPeer is returned when the remote peer's ID does not match the expected DID.
	ErrWrongPeer = errors.New("peer ID does not match expected DID")
)

// ChallengeResponse runs the responder side: read challenge from stream, sign with privKey, write signature.
func ChallengeResponse(stream network.Stream, privKey crypto.PrivKey) error {
	defer stream.Close()
	challenge := make([]byte, ChallengeSize)
	if _, err := io.ReadFull(stream, challenge); err != nil {
		return fmt.Errorf("read challenge: %w", err)
	}
	sig, err := privKey.Sign(challenge)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}
	// Write length-prefixed signature
	if err := binary.Write(stream, binary.BigEndian, uint32(len(sig))); err != nil {
		return err
	}
	if _, err := stream.Write(sig); err != nil {
		return err
	}
	return stream.CloseWrite()
}

// VerifyChallenge runs the initiator side: write challenge, read signature, verify with expected DID.
// If verification succeeds, the stream is left open and the caller can use it for further protocol.
func VerifyChallenge(ctx context.Context, stream network.Stream, expectedDID string) error {
	pubKey, err := did.ParseToPubKey(expectedDID)
	if err != nil {
		return err
	}
	challenge := make([]byte, ChallengeSize)
	if _, err := rand.Read(challenge); err != nil {
		return err
	}
	if _, err := stream.Write(challenge); err != nil {
		stream.Reset()
		return err
	}
	if err := stream.CloseWrite(); err != nil {
		stream.Reset()
		return err
	}
	var sigLen uint32
	if err := binary.Read(stream, binary.BigEndian, &sigLen); err != nil {
		stream.Reset()
		return err
	}
	if sigLen > 1<<20 {
		stream.Reset()
		return ErrAuthFailed
	}
	sig := make([]byte, sigLen)
	if _, err := io.ReadFull(stream, sig); err != nil {
		stream.Reset()
		return err
	}
	ok, err := pubKey.Verify(challenge, sig)
	if err != nil || !ok {
		stream.Reset()
		return ErrAuthFailed
	}
	// Optionally verify that the stream's remote peer ID matches the DID-derived peer ID
	remoteID := stream.Conn().RemotePeer()
	expectedID, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		return err
	}
	if remoteID != expectedID {
		stream.Reset()
		return ErrWrongPeer
	}
	return nil
}

// RunInitiator opens a stream to the given peer with ProtocolID, then runs VerifyChallenge.
// Returns the stream on success (caller must close it when done).
func RunInitiator(ctx context.Context, h host.Host, pid peer.ID, expectedDID string) (network.Stream, error) {
	stream, err := h.NewStream(ctx, pid, ProtocolID)
	if err != nil {
		return nil, err
	}
	if err := VerifyChallenge(ctx, stream, expectedDID); err != nil {
		return nil, err
	}
	return stream, nil
}
