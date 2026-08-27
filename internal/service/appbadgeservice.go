package service

import (
	"strconv"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/services/dock"
)

// AppBadgeService keeps the launcher/taskbar badge in sync with the unread
// state shown by the application. The platform implementation is supplied by
// Wails3 (macOS Dock and Windows taskbar; Linux is a safe no-op).
type AppBadgeService struct {
	dock *dock.DockService
	mu   sync.Mutex
}

func NewAppBadgeService(dockService *dock.DockService) *AppBadgeService {
	return &AppBadgeService{dock: dockService}
}

func (s *AppBadgeService) SetCount(count int) error {
	if s == nil || s.dock == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if count <= 0 {
		return s.dock.RemoveBadge()
	}
	return s.dock.SetBadge(strconv.Itoa(count))
}
