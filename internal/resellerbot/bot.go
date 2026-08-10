package resellerbot

import (
	"context"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

// chatStateStore tracks one conversation state per chat within a single bot
// instance. Separate from internal/web/service/tgbot's userStateMgr: each
// owner's bot is its own instance, so chat ids never collide across owners.
type chatStateStore struct {
	mu     sync.Mutex
	states map[int64]string
}

func newChatStateStore() *chatStateStore {
	return &chatStateStore{states: make(map[int64]string)}
}

func (s *chatStateStore) set(chatID int64, state string) {
	s.mu.Lock()
	s.states[chatID] = state
	s.mu.Unlock()
}

func (s *chatStateStore) get(chatID int64) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.states[chatID]
	return v, ok
}

func (s *chatStateStore) clear(chatID int64) {
	s.mu.Lock()
	delete(s.states, chatID)
	s.mu.Unlock()
}

type botInstance struct {
	ownerId int
	token   string
	bot     *telego.Bot
	states  *chatStateStore
	cancel  context.CancelFunc
	handler *th.BotHandler
	wg      sync.WaitGroup
}

func startBotInstance(ownerId int, token string) (*botInstance, error) {
	bot, err := telego.NewBot(token)
	if err != nil {
		return nil, err
	}

	inst := &botInstance{
		ownerId: ownerId,
		token:   token,
		bot:     bot,
		states:  newChatStateStore(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	inst.cancel = cancel

	updates, err := bot.UpdatesViaLongPolling(ctx, &telego.GetUpdatesParams{Timeout: 20})
	if err != nil {
		cancel()
		return nil, err
	}

	h, err := th.NewBotHandler(bot, updates)
	if err != nil {
		cancel()
		return nil, err
	}
	registerHandlers(h, inst)
	inst.handler = h

	inst.wg.Add(1)
	go func() {
		defer inst.wg.Done()
		_ = h.Start()
	}()

	go inst.cacheUsername()

	logger.Infof("resellerbot: started bot for owner %d", ownerId)
	return inst, nil
}

// cacheUsername best-effort fetches and stores the bot's own @username so
// the panel can show it without a live getMe round trip on every page load.
func (inst *botInstance) cacheUsername() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	me, err := inst.bot.GetMe(ctx)
	if err != nil || me == nil {
		return
	}
	updateBotUsername(inst.ownerId, me.Username)
}

func (inst *botInstance) stop() {
	if inst.cancel != nil {
		inst.cancel()
	}
	if inst.handler != nil {
		_ = inst.handler.Stop()
	}
	inst.wg.Wait()
	logger.Infof("resellerbot: stopped bot for owner %d", inst.ownerId)
}
