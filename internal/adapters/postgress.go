package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"realtimer/internal/config"
	"realtimer/internal/pubsub"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lib/pq"
)

var postgressdb *pgx.Conn

const channelName = "your_channel_name"

type Listener struct {
	db          *pgx.Conn
	channelName string
}

func newListener(db *pgx.Conn) *Listener {
	return &Listener{
		db:          db,
		channelName: channelName,
	}
}

func newPostgress(cfg config.DBConfig, pubsub *pubsub.SubscriptionManager) (*pgx.Conn, error) {
	ctx := context.Background()

	host := fmt.Sprintf("%s:%s", cfg.Database.Host, cfg.Database.Host)

	dsn := url.URL{
		Scheme: "postgres",
		Host:   host,
		User: url.UserPassword(
			cfg.Database.Username,
			cfg.Database.Password,
		),
		Path: cfg.Database.Name,
	}

	q := dsn.Query()
	q.Add("sslmode", "disable")
	dsn.RawQuery = q.Encode()

	var err error
	postgressdb, err = pgx.Connect(ctx, dsn.String())
	if err != nil {
		return nil, err
	}

	listener := newListener(postgressdb)
	go listener.Start(ctx, pubsub)

	return postgressdb, nil
}

func (l *Listener) Start(ctx context.Context, pubsub *pubsub.SubscriptionManager) error {

	listener := pq.NewListener(os.Getenv("DATABASE_URL"), 10*time.Second, time.Minute, nil)
	err := listener.Listen(channelName)
	if err != nil {
		return fmt.Errorf("failed to listen to channel: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			notification := listener.NotificationChannel()
			for n := range notification {
				l.handleNotifications(n, pubsub)
			}
		}
	}
}

func (d *Listener) handleNotifications(notification *pq.Notification, pubsub *pubsub.SubscriptionManager) {
	payload, err := d.parsePayload(notification.Extra)
	if err != nil {
		return
	}

	pubsub.Publish("", payload)
}

func (d *Listener) parsePayload(rawPayload string) (map[string]string, error) {
	var payload map[string]string

	err := json.Unmarshal([]byte(rawPayload), &payload)
	if err != nil {
		return payload, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	return payload, nil
}
