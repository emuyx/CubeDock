package core

import (
	"context"
	"fmt"

	"github.com/emuyx/cubedock/internal/friends"
)

func (m *Manager) friendCredentials(index int) (friendCredentials, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	creds, ok := m.friendSessions[index]
	if !ok || creds.uid == 0 || len(creds.key) < 16 {
		return friendCredentials{}, fmt.Errorf("房间未运行或尚未完成网易登录")
	}
	return friendCredentials{uid: creds.uid, key: append([]byte(nil), creds.key...)}, nil
}

// FriendRequests lists pending friend applications for one running room.
func (m *Manager) FriendRequests(ctx context.Context, index int) ([]friends.Request, error) {
	creds, err := m.friendCredentials(index)
	if err != nil {
		return nil, err
	}
	session, err := friends.NewSession(creds.uid, creds.key)
	if err != nil {
		return nil, err
	}
	return session.Pending(ctx)
}

// AcceptFriendRequest accepts one pending application for a running room.
func (m *Manager) AcceptFriendRequest(ctx context.Context, index int, uid uint64) error {
	creds, err := m.friendCredentials(index)
	if err != nil {
		return err
	}
	session, err := friends.NewSession(creds.uid, creds.key)
	if err != nil {
		return err
	}
	if err := session.Accept(ctx, uid); err != nil {
		return err
	}
	m.emit("账号", fmt.Sprintf("房间 %d 已手动同意好友申请（UID %d）", index+1, uid))
	return nil
}
