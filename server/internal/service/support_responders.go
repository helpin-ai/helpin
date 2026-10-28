package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// Support responders are the teammates who actually answer customers. Only they
// count when Helpin tells customers that someone is around, so teammates who
// only read conversations or get tagged into them never promise a live reply.
const (
	supportResponderWindow       = 14 * 24 * time.Hour
	supportResponderMinReplyDays = 3
	supportNewMemberWindow       = 7 * 24 * time.Hour
	supportResponderCacheTTL     = 10 * time.Minute
	supportResponderCacheMax     = 1024
)

// supportResponders is the set of teammates who answer customers in a workspace.
type supportResponders struct {
	userIDs map[string]struct{}
	// hasReplyHistory is false when no teammate replied to a customer within
	// supportResponderWindow. Such a workspace has nothing to learn from yet, so
	// everyone with support access counts until someone replies.
	hasReplyHistory bool
}

// includes reports whether userID counts as someone who answers customers.
func (r supportResponders) includes(userID string) bool {
	if !r.hasReplyHistory {
		return true
	}
	_, ok := r.userIDs[userID]
	return ok
}

// newSupportResponders picks the responders from recent teammate replies. A
// teammate counts after replying on supportResponderMinReplyDays separate days
// (in loc) within supportResponderWindow, or after one reply if they joined
// the workspace within supportNewMemberWindow.
func newSupportResponders(replies []repository.SupportTeammateReply, loc *time.Location, now time.Time) supportResponders {
	if loc == nil {
		loc = time.UTC
	}
	windowStart := now.Add(-supportResponderWindow)
	newMemberSince := now.Add(-supportNewMemberWindow)
	replyDays := make(map[string]map[string]struct{})
	responders := supportResponders{userIDs: make(map[string]struct{})}
	for _, reply := range replies {
		userID := strings.TrimSpace(reply.UserID)
		if userID == "" || reply.RepliedAt.Before(windowStart) {
			continue
		}
		responders.hasReplyHistory = true
		if !reply.JoinedAt().Before(newMemberSince) {
			responders.userIDs[userID] = struct{}{}
			continue
		}
		if replyDays[userID] == nil {
			replyDays[userID] = make(map[string]struct{})
		}
		replyDays[userID][reply.RepliedAt.In(loc).Format(time.DateOnly)] = struct{}{}
		if len(replyDays[userID]) >= supportResponderMinReplyDays {
			responders.userIDs[userID] = struct{}{}
		}
	}
	return responders
}

// supportResponderIndex holds a workspace's responders and, for each inbox, the
// teammates who answer customers in that inbox. The Shared Inbox is keyed "".
type supportResponderIndex struct {
	workspace supportResponders
	byMailbox map[string]supportResponders
}

// newSupportResponderIndex counts responders across the workspace and per inbox.
// A reply counts toward the inbox the conversation is in now, unless the
// conversation moved after it: it was then written in an earlier inbox, and
// counting it would make a Shared Inbox teammate look like the new inbox's team.
func newSupportResponderIndex(replies []repository.SupportTeammateReply, loc *time.Location, now time.Time) supportResponderIndex {
	repliesByMailbox := make(map[string][]repository.SupportTeammateReply)
	for _, reply := range replies {
		if reply.MovedAfter {
			continue
		}
		mailboxID := strings.TrimSpace(derefString(reply.MailboxID))
		repliesByMailbox[mailboxID] = append(repliesByMailbox[mailboxID], reply)
	}
	index := supportResponderIndex{
		workspace: newSupportResponders(replies, loc, now),
		byMailbox: make(map[string]supportResponders, len(repliesByMailbox)),
	}
	for mailboxID, mailboxReplies := range repliesByMailbox {
		index.byMailbox[mailboxID] = newSupportResponders(mailboxReplies, loc, now)
	}
	return index
}

// inbox returns the teammates who answer customers in an inbox, "" for the
// Shared Inbox. An inbox without recent replies has no reply history.
func (i supportResponderIndex) inbox(mailboxID string) supportResponders {
	return i.byMailbox[mailboxID]
}

// SupportResponderDirectory finds and caches the teammates who answer customers in each workspace.
type SupportResponderDirectory struct {
	messageRepo *repository.SupportMessageRepository

	mu    sync.Mutex
	cache map[string]cachedSupportResponders
}

type cachedSupportResponders struct {
	timezone string
	loadedAt time.Time
	index    supportResponderIndex
}

// NewSupportResponderDirectory creates a responder directory backed by support messages.
func NewSupportResponderDirectory(messageRepo *repository.SupportMessageRepository) *SupportResponderDirectory {
	return &SupportResponderDirectory{messageRepo: messageRepo, cache: make(map[string]cachedSupportResponders)}
}

// responders returns the workspace's responders, counting reply days in the
// business-hours timezone. A nil directory knows no reply history, so every
// teammate with support access counts.
func (d *SupportResponderDirectory) responders(ctx context.Context, workspaceID, timezone string, now time.Time) (supportResponders, error) {
	index, err := d.index(ctx, workspaceID, timezone, now)
	return index.workspace, err
}

// index returns the workspace's responders together with each inbox's.
func (d *SupportResponderDirectory) index(ctx context.Context, workspaceID, timezone string, now time.Time) (supportResponderIndex, error) {
	if d == nil || d.messageRepo == nil || strings.TrimSpace(workspaceID) == "" {
		return supportResponderIndex{}, nil
	}
	d.mu.Lock()
	cached, ok := d.cache[workspaceID]
	d.mu.Unlock()
	if ok && cached.timezone == timezone && now.Sub(cached.loadedAt) < supportResponderCacheTTL && !now.Before(cached.loadedAt) {
		return cached.index, nil
	}

	replies, err := d.messageRepo.ListTeammateReplies(ctx, workspaceID, now.Add(-supportResponderWindow))
	if err != nil {
		return supportResponderIndex{}, fmt.Errorf("load support responders: %w", err)
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil || strings.TrimSpace(timezone) == "" {
		loc = time.UTC
	}
	index := newSupportResponderIndex(replies, loc, now)

	d.mu.Lock()
	if len(d.cache) >= supportResponderCacheMax {
		for id, entry := range d.cache {
			if now.Sub(entry.loadedAt) >= supportResponderCacheTTL {
				delete(d.cache, id)
			}
		}
	}
	d.cache[workspaceID] = cachedSupportResponders{timezone: timezone, loadedAt: now, index: index}
	d.mu.Unlock()
	return index, nil
}

// supportCustomerTeammates is what customers are told about each support teammate.
type supportCustomerTeammates struct {
	teammates  []supportTeammatePresence
	responders supportResponders
	now        time.Time
}

// resolveSupportCustomerTeammates resolves teammate presence and decides which
// teammates customers can expect a reply from right now.
func resolveSupportCustomerTeammates(
	ctx context.Context,
	workspaceRepo *repository.WorkspaceRepository,
	presence websocket.PresenceProvider,
	statusOverrideRepo *repository.SupportTeammateStatusOverrideRepository,
	responderDirectory *SupportResponderDirectory,
	workspaceID, timezone string,
	now time.Time,
) (supportCustomerTeammates, error) {
	teammates, err := resolveSupportTeammatePresence(ctx, workspaceRepo, presence, statusOverrideRepo, workspaceID, now)
	if err != nil {
		return supportCustomerTeammates{}, err
	}
	return newSupportCustomerTeammates(ctx, responderDirectory, teammates, workspaceID, timezone, now)
}

// newSupportCustomerTeammates adds the workspace's responders to resolved
// presence. Responders are only looked up when someone is around.
func newSupportCustomerTeammates(
	ctx context.Context,
	responderDirectory *SupportResponderDirectory,
	teammates []supportTeammatePresence,
	workspaceID, timezone string,
	now time.Time,
) (supportCustomerTeammates, error) {
	customers := supportCustomerTeammates{teammates: teammates, now: now}
	for _, teammate := range teammates {
		if !customers.around(teammate) {
			continue
		}
		responders, err := responderDirectory.responders(ctx, workspaceID, timezone, now)
		if err != nil {
			return supportCustomerTeammates{}, err
		}
		customers.responders = responders
		break
	}
	return customers, nil
}

// around reports whether a teammate has Helpin open, used it recently, and has
// not set themselves Away or Offline.
func (c supportCustomerTeammates) around(teammate supportTeammatePresence) bool {
	if manual := teammate.status.ManualStatus; manual != nil && *manual != model.SupportTeammateStatusOnline {
		return false
	}
	return teammate.connected && teammate.activeWithin(supportTeammateActiveWindow, c.now)
}

// available reports whether customers can expect a reply from this teammate
// right now: a support responder who is around.
func (c supportCustomerTeammates) available(teammate supportTeammatePresence) bool {
	return c.around(teammate) && c.responders.includes(teammate.status.UserID)
}

// anyAvailable reports whether any teammate is available to customers.
func (c supportCustomerTeammates) anyAvailable() bool {
	for _, teammate := range c.teammates {
		if c.available(teammate) {
			return true
		}
	}
	return false
}

// anyAround reports whether any teammate is around, responder or not.
func (c supportCustomerTeammates) anyAround() bool {
	for _, teammate := range c.teammates {
		if c.around(teammate) {
			return true
		}
	}
	return false
}

// anyAvailableAmong reports whether one of an inbox's people is around. The
// inbox decides who its people are, so workspace-wide reply counts do not.
func (c supportCustomerTeammates) anyAvailableAmong(userIDs map[string]struct{}) bool {
	for _, teammate := range c.teammates {
		if _, ok := userIDs[teammate.status.UserID]; ok && c.around(teammate) {
			return true
		}
	}
	return false
}

// statusByUserID maps each teammate to the status customers see: online only
// when available, away while they still have Helpin open or just left, and
// offline otherwise.
func (c supportCustomerTeammates) statusByUserID() map[string]string {
	statuses := make(map[string]string, len(c.teammates))
	for _, teammate := range c.teammates {
		status := teammate.status.Status
		switch {
		case c.available(teammate):
			status = model.SupportTeammateStatusOnline
		case status == model.SupportTeammateStatusOnline:
			status = model.SupportTeammateStatusAway
		}
		statuses[teammate.status.UserID] = status
	}
	return statuses
}

// responderUserIDs returns the teammates customers should see as the support
// team, or nil when the workspace has no reply history to tell them apart.
func (c supportCustomerTeammates) responderUserIDs() map[string]struct{} {
	if !c.responders.hasReplyHistory {
		return nil
	}
	return c.responders.userIDs
}
