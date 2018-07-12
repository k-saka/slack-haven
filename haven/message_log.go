package haven

import (
	"sync"
)

// MessageMap contains same message which posted to relaing channels.
// key means posted channel. value means message id.
type messageMap struct {
	originChannelID string
	originID        string
	mmap            map[string]string
}

func newMessageMap(channelID, messageID string) messageMap {
	return messageMap{
		originChannelID: channelID,
		originID:        messageID,
		mmap:            map[string]string{channelID: messageID},
	}
}

// MessageLog is sent message container.
type messageLog struct {
	records []messageMap
	mu      sync.RWMutex
}

// NewMessageLog create message log
func newMessageLog(size int) *messageLog {
	return &messageLog{
		records: make([]messageMap, 0, size),
		mu:      sync.RWMutex{},
	}
}

// Add message log
func (l *messageLog) add(channelID, messageID, originID string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Add new message
	if messageID == originID {
		// If log count is over record cap, pop first record
		if cap(l.records) <= len(l.records) {
			l.records = l.records[1:]
		}
		l.records = append(l.records, newMessageMap(channelID, messageID))
	}

	// Add relayed message
	for _, row := range l.records {
		if row.originID == originID {
			row.mmap[channelID] = messageID
			return
		}
	}
	logger.Info("message log: %#v", l.records)
}

func (l *messageLog) getMessageMap(channelID, messageID string) map[string]string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, record := range l.records {
		if msgID, ok := record.mmap[channelID]; ok {
			if msgID == messageID {
				return record.mmap
			}
		}
	}
	return nil
}
