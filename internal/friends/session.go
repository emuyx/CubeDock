package friends

import (
	"context"
	"fmt"
	"strconv"
)

// Request is one pending friend application shown in the console.
type Request struct {
	// UID is encoded as a string because NetEase IDs can exceed JavaScript's
	// safe integer range.
	UID       uint64 `json:"uid,string"`
	Name      string `json:"name"`
	ApplyTime string `json:"apply_time,omitempty"`
}

// Session reads and accepts pending friend requests on demand.
type Session struct {
	client *gatewayClient
}

// NewSession builds a request session from the room owner's UID and room
// encryption key.
func NewSession(uid uint32, encryptionKey []byte) (*Session, error) {
	if uid == 0 {
		return nil, fmt.Errorf("房间账号 UID 无效")
	}
	if len(encryptionKey) != 32 {
		return nil, fmt.Errorf("房间加密密钥长度无效")
	}
	return &Session{client: newGatewayClient(uid, encryptionKey)}, nil
}

// Pending returns the current friend applications.
func (s *Session) Pending(ctx context.Context) ([]Request, error) {
	if err := s.client.ensureGateway(ctx, nil); err != nil {
		return nil, err
	}
	applications, err := s.client.pending(ctx)
	if err != nil {
		// The gateway may have rotated; force discovery on the next call.
		s.client.gateway = ""
		return nil, err
	}
	requests := make([]Request, 0, len(applications))
	for _, application := range applications {
		id, ok := accountID(application.UID)
		if !ok || strconv.FormatUint(id, 10) == s.client.uid {
			continue
		}
		requests = append(requests, Request{
			UID:       id,
			Name:      displayName(application.Nickname),
			ApplyTime: string(application.ApplyTime),
		})
	}
	return requests, nil
}

// Accept accepts one pending friend application.
func (s *Session) Accept(ctx context.Context, uid uint64) error {
	if uid == 0 {
		return fmt.Errorf("请指定好友申请 UID")
	}
	if err := s.client.ensureGateway(ctx, nil); err != nil {
		return err
	}
	if err := s.client.accept(ctx, uid); err != nil {
		s.client.gateway = ""
		return err
	}
	return nil
}
