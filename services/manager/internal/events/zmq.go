package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/go-zeromq/zmq4"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/pkg/monerod"
)

// zmqBlockMessage is the JSON structure published by monerod on the
// "json-minimal-chain_main" ZMQ topic.
type zmqBlockMessage struct {
	FirstHeight uint64 `json:"first_height"`
	FirstPrevID string `json:"first_prev_id"`
	NBlocks     int    `json:"n_blocks"`
}

// BlockListener subscribes to monerod ZMQ block notifications and fires
// registered handler callbacks on each new block.
type BlockListener struct {
	zmqURL   string
	monerod  *monerod.Client // used for polling fallback
	logger   *slog.Logger

	mu       sync.Mutex
	handlers []func(height uint64)
}

// NewBlockListener creates a new block event listener.
// The monerod client is used as a polling fallback when ZMQ is unavailable.
// Pass nil for monerodClient if polling fallback is not needed.
func NewBlockListener(zmqURL string, monerodClient *monerod.Client, logger *slog.Logger) *BlockListener {
	return &BlockListener{
		zmqURL:  zmqURL,
		monerod: monerodClient,
		logger:  logger.With(slog.String("component", "block-listener")),
	}
}

// OnBlock registers a callback for new block events. The callback receives
// the new block height. Must be called before Run.
func (bl *BlockListener) OnBlock(fn func(height uint64)) {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.handlers = append(bl.handlers, fn)
}

// fireHandlers calls all registered handlers with the given height.
func (bl *BlockListener) fireHandlers(height uint64) {
	bl.mu.Lock()
	handlers := make([]func(uint64), len(bl.handlers))
	copy(handlers, bl.handlers)
	bl.mu.Unlock()

	for _, fn := range handlers {
		fn(height)
	}
}

// Run connects to monerod ZMQ and listens for block events. It blocks until
// ctx is cancelled. If the ZMQ connection fails after retries, it falls back
// to polling monerod via RPC.
func (bl *BlockListener) Run(ctx context.Context) error {
	bl.logger.Info("starting block listener", slog.String("zmq_url", bl.zmqURL))

	const maxRetries = 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := bl.runZMQ(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		bl.logger.Warn("ZMQ connection failed",
			slog.Int("attempt", attempt),
			slog.Int("max_retries", maxRetries),
			slog.String("error", err.Error()),
		)
		// Brief pause before retry.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	bl.logger.Warn("ZMQ unavailable after retries, falling back to polling mode")
	return bl.runPolling(ctx)
}

// runZMQ connects to the ZMQ PUB socket and listens for block messages.
func (bl *BlockListener) runZMQ(ctx context.Context) error {
	sub := zmq4.NewSub(ctx)
	defer sub.Close()

	if err := sub.Dial(bl.zmqURL); err != nil {
		return fmt.Errorf("dialing ZMQ at %s: %w", bl.zmqURL, err)
	}

	if err := sub.SetOption(zmq4.OptionSubscribe, "json-minimal-chain_main"); err != nil {
		return fmt.Errorf("subscribing to ZMQ topic: %w", err)
	}

	bl.logger.Info("connected to monerod ZMQ", slog.String("topic", "json-minimal-chain_main"))

	for {
		msg, err := sub.Recv()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("receiving ZMQ message: %w", err)
		}

		// ZMQ PUB messages have multiple frames: [topic, payload].
		// The go-zeromq library concatenates them into msg.Frames.
		if len(msg.Frames) < 2 {
			bl.logger.Warn("received ZMQ message with unexpected frame count",
				slog.Int("frames", len(msg.Frames)),
			)
			continue
		}

		payload := msg.Frames[1]

		var blockMsg zmqBlockMessage
		if err := json.Unmarshal(payload, &blockMsg); err != nil {
			bl.logger.Warn("failed to parse ZMQ block message",
				slog.String("error", err.Error()),
				slog.String("payload", string(payload)),
			)
			continue
		}

		// Fire handlers for each block in the message. Typically NBlocks is 1,
		// but during reorgs it can be higher.
		for i := 0; i < blockMsg.NBlocks; i++ {
			height := blockMsg.FirstHeight + uint64(i)
			bl.logger.Info("new block from ZMQ", slog.Uint64("height", height))
			bl.fireHandlers(height)
		}
	}
}

// runPolling polls monerod get_last_block_header every 5 seconds as a fallback
// when ZMQ is unavailable.
func (bl *BlockListener) runPolling(ctx context.Context) error {
	if bl.monerod == nil {
		return fmt.Errorf("polling fallback unavailable: no monerod client configured")
	}

	bl.logger.Info("starting polling mode", slog.Duration("interval", 5*time.Second))

	var lastHeight uint64

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Fetch initial height.
	header, err := bl.monerod.GetLastBlockHeader(ctx)
	if err != nil {
		bl.logger.Error("failed to get initial block header for polling",
			slog.String("error", err.Error()),
		)
	} else {
		lastHeight = header.Height
		bl.logger.Info("polling initial height", slog.Uint64("height", lastHeight))
	}

	for {
		select {
		case <-ctx.Done():
			bl.logger.Info("polling mode stopping")
			return ctx.Err()
		case <-ticker.C:
			header, err := bl.monerod.GetLastBlockHeader(ctx)
			if err != nil {
				bl.logger.Error("polling: failed to get block header",
					slog.String("error", err.Error()),
				)
				continue
			}

			if header.Height > lastHeight {
				// Fire handlers for every height we may have missed.
				for h := lastHeight + 1; h <= header.Height; h++ {
					bl.logger.Info("new block from polling", slog.Uint64("height", h))
					bl.fireHandlers(h)
				}
				lastHeight = header.Height
			}
		}
	}
}
