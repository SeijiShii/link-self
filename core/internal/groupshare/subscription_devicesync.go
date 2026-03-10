package groupshare

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SeijiShii/link-self/core/internal/devicesync"
)

const subsTable = "_groupshare_subs"

// subsRecord is the JSON body stored in DeviceSync for each subscription.
type subsRecord struct {
	Topics []string `json:"topics"`
}

// DeviceSyncSubscriptionStore implements SubscriptionStore backed by
// a DeviceSync ReplicationEngine. Subscriptions are stored in the
// "_groupshare_subs" table and automatically replicated to same-DID devices.
type DeviceSyncSubscriptionStore struct {
	engine *devicesync.ReplicationEngine
}

func NewDeviceSyncSubscriptionStore(engine *devicesync.ReplicationEngine) *DeviceSyncSubscriptionStore {
	return &DeviceSyncSubscriptionStore{engine: engine}
}

// recordID encodes did + channel into a single DeviceSync record ID.
func subRecordID(did, channel string) string {
	return fmt.Sprintf("%s::%s", did, channel)
}

func (s *DeviceSyncSubscriptionStore) SetSubscription(did, channel string, topics []string) error {
	body, err := json.Marshal(subsRecord{Topics: topics})
	if err != nil {
		return err
	}
	return s.engine.Put(context.Background(), subsTable, subRecordID(did, channel), body)
}

func (s *DeviceSyncSubscriptionStore) GetSubscription(did, channel string) ([]string, error) {
	rec, err := s.engine.Get(context.Background(), subsTable, subRecordID(did, channel))
	if err != nil || rec == nil {
		return nil, err
	}
	var sr subsRecord
	if err := json.Unmarshal(rec.Body, &sr); err != nil {
		return nil, err
	}
	return sr.Topics, nil
}

func (s *DeviceSyncSubscriptionStore) GetAllSubscriptions(did string) (map[string][]string, error) {
	recs, err := s.engine.List(context.Background(), subsTable)
	if err != nil {
		return nil, err
	}
	prefix := did + "::"
	result := make(map[string][]string)
	for _, rec := range recs {
		if strings.HasPrefix(rec.ID, prefix) {
			channel := strings.TrimPrefix(rec.ID, prefix)
			var sr subsRecord
			if err := json.Unmarshal(rec.Body, &sr); err != nil {
				continue
			}
			result[channel] = sr.Topics
		}
	}
	return result, nil
}
