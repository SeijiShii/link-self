package groupshare

import "sync"

// SubscriptionStore stores topic subscriptions per DID per channel.
type SubscriptionStore interface {
	SetSubscription(did, channel string, topics []string) error
	GetSubscription(did, channel string) ([]string, error)
	GetAllSubscriptions(did string) (map[string][]string, error)
}

// TopicMatches returns true if the record's topic is covered by the
// subscribed topics list. Wildcard "*" matches any topic.
// nil or empty slice means not subscribed (no match).
func TopicMatches(subscribed []string, topic string) bool {
	for _, s := range subscribed {
		if s == "*" || s == topic {
			return true
		}
	}
	return false
}

type subKey struct {
	did     string
	channel string
}

// MemSubscriptionStore is an in-memory implementation of SubscriptionStore.
type MemSubscriptionStore struct {
	mu   sync.RWMutex
	subs map[subKey][]string
}

func NewMemSubscriptionStore() *MemSubscriptionStore {
	return &MemSubscriptionStore{
		subs: make(map[subKey][]string),
	}
}

func (m *MemSubscriptionStore) SetSubscription(did, channel string, topics []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]string, len(topics))
	copy(cp, topics)
	m.subs[subKey{did, channel}] = cp
	return nil
}

func (m *MemSubscriptionStore) GetSubscription(did, channel string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	topics, ok := m.subs[subKey{did, channel}]
	if !ok {
		return nil, nil
	}
	return topics, nil
}

func (m *MemSubscriptionStore) GetAllSubscriptions(did string) (map[string][]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string][]string)
	for k, v := range m.subs {
		if k.did == did {
			result[k.channel] = v
		}
	}
	return result, nil
}
