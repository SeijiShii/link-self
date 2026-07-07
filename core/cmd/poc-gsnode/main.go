// poc-gsnode is a throwaway harness for the Go↔TS groupshare interop test
// (ts/linkself/test/groupshare.interop.test.ts). It wires the internal
// groupshare layer onto a node the same way pkg/linkself does, with a fixed
// two-member group of {self, -peer}. Control protocol over plain messages:
//
//	"go-put"     → put a record {visits, area/1, go-1, "from-go"} and reply "put-done"
//	"check:<id>" → reply "found:<body>" or "missing:<id>"
//
// Once groupshare is wired into the public MyDB sync scope (Phase C), this
// harness can be replaced by one built on pkg/linkself.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/SeijiShii/link-self/core/internal/did"
	"github.com/SeijiShii/link-self/core/internal/envelope"
	"github.com/SeijiShii/link-self/core/internal/groupshare"
	"github.com/SeijiShii/link-self/core/internal/node"
)

type fixedResolver struct{ members []string }

func (f *fixedResolver) MemberDIDsForGroup(_ context.Context, _ string) ([]string, error) {
	return f.members, nil
}

func main() {
	peerDID := flag.String("peer", "", "remote (js) peer DID, member of the shared group")
	flag.Parse()
	if *peerDID == "" {
		fmt.Fprintln(os.Stderr, "usage: poc-gsnode -peer <did>")
		os.Exit(2)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, err := node.New(ctx, node.Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0/ws"},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "start node:", err)
		os.Exit(1)
	}
	defer n.Close()

	gsLayer := groupshare.NewGroupShareLayer(
		groupshare.NewMemSharedStorage(),
		&fixedResolver{members: []string{*peerDID}},
		func(ctx context.Context, memberDIDs []string, payload []byte) error {
			wrapped, err := envelope.Wrap(envelope.TypeGroupShare, payload)
			if err != nil {
				return err
			}
			return n.SendToGroup(ctx, memberDIDs, wrapped)
		},
		n.Identity.DID,
	)
	gsLayer.RemoteSubs = groupshare.NewMemSubscriptionStore()
	gsLayer.SendSubAnnounce = func(ctx context.Context, memberDIDs []string, payload []byte) error {
		wrapped, err := envelope.Wrap(envelope.TypeSubAnnounce, payload)
		if err != nil {
			return err
		}
		return n.SendToGroup(ctx, memberDIDs, wrapped)
	}
	if err := gsLayer.RegisterChannel(&groupshare.Channel{Name: "visits", GroupID: "net-1"}); err != nil {
		fmt.Fprintln(os.Stderr, "register channel:", err)
		os.Exit(1)
	}

	n.SetOnGroupShare(func(peerDID string, payload []byte) {
		if err := gsLayer.HandleIncoming(context.Background(), payload); err != nil {
			fmt.Fprintln(os.Stderr, "groupshare incoming:", err)
		}
	})
	n.SetOnSubAnnounce(func(peerDID string, payload []byte) {
		_ = gsLayer.HandleSubAnnouncement(peerDID, payload)
	})

	reply := func(toDID string, msg string) {
		pid, err := did.DIDToPeerID(toDID)
		if err != nil {
			return
		}
		if err := n.SendToPeerID(ctx, pid, []byte(msg)); err != nil {
			fmt.Fprintln(os.Stderr, "reply:", err)
		}
	}

	n.SetOnMessage(func(fromDID string, payload []byte) {
		msg := string(payload)
		fmt.Fprintf(os.Stderr, "[go] message from %s: %q\n", fromDID, msg)
		switch {
		case msg == "go-put":
			if err := gsLayer.Put(ctx, "visits", "area/1", "go-1", []byte("from-go")); err != nil {
				reply(fromDID, "put-error:"+err.Error())
				return
			}
			reply(fromDID, "put-done")
		case strings.HasPrefix(msg, "check:"):
			id := strings.TrimPrefix(msg, "check:")
			rec, err := gsLayer.Get(ctx, "visits", id)
			if err != nil || rec == nil {
				reply(fromDID, "missing:"+id)
				return
			}
			reply(fromDID, "found:"+string(rec.Body))
		}
	})

	if err := n.Start(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "start:", err)
		os.Exit(1)
	}

	wsAddr := ""
	for _, a := range n.Host.Addrs() {
		if strings.Contains(a.String(), "/ws") {
			wsAddr = a.String() + "/p2p/" + n.Host.ID().String()
			break
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]string{
		"did":    n.Identity.DID,
		"wsAddr": wsAddr,
	})

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
}
