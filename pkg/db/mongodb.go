package db

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type User struct {
	UserID           int64     `bson:"user_id"`
	Username         string    `bson:"username,omitempty"`
	FirstName        string    `bson:"first_name,omitempty"`
	LastName         string    `bson:"last_name,omitempty"`
	LanguageCode     string    `bson:"language_code,omitempty"`
	SelectedLanguage string    `bson:"selected_language,omitempty"`
	IsBlocked        bool      `bson:"is_blocked"`
	IsBanned         bool      `bson:"is_banned"`
	BanReason        string    `bson:"ban_reason,omitempty"`
	CreatedAt        time.Time `bson:"created_at"`
	LastActiveAt     time.Time `bson:"last_active_at"`
}

type MessageLog struct {
	UserID    int64     `bson:"user_id"`
	ChatID    int64     `bson:"chat_id"`
	MessageID int64     `bson:"message_id"`
	Text      string    `bson:"text,omitempty"`
	CreatedAt time.Time `bson:"created_at"`
}

type DownloadLog struct {
	UserID    int64     `bson:"user_id"`
	URL       string    `bson:"url"`
	Status    string    `bson:"status"`
	CreatedAt time.Time `bson:"created_at"`
}

type BroadcastLog struct {
	BroadcastID string    `bson:"broadcast_id"`
	OwnerID     int64     `bson:"owner_id"`
	TotalUsers  int       `bson:"total_users"`
	Delivered   int       `bson:"delivered"`
	Blocked     int       `bson:"blocked"`
	Failed      int       `bson:"failed"`
	StartedAt   time.Time `bson:"started_at"`
	CompletedAt time.Time `bson:"completed_at"`
}

type Stats struct {
	TotalUsers     int64
	Active24hUsers int64
	BlockedUsers   int64
	BannedUsers    int64
	TotalMessages  int64
	TotalDownloads int64
}

type DB struct {
	client      *mongo.Client
	database    *mongo.Database
	users       *mongo.Collection
	messages    *mongo.Collection
	downloads   *mongo.Collection
	broadcasts  *mongo.Collection
	initialized bool
}

var (
	instance *DB
	once     sync.Once
)

func Init(uri, dbName string) (*DB, error) {
	if uri == "" {
		log.Println("[MongoDB] URI not set; running without database storage.")
		return nil, nil
	}

	if dbName == "" {
		dbName = "slmedia"
	}

	var initErr error
	once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		clientOpts := options.Client().
			ApplyURI(uri).
			SetMaxPoolSize(50).
			SetMinPoolSize(1).
			SetMaxConnIdleTime(5 * time.Minute)

		client, err := mongo.Connect(clientOpts)
		if err != nil {
			initErr = fmt.Errorf("mongo connect error: %w", err)
			return
		}

		if err := client.Ping(ctx, nil); err != nil {
			initErr = fmt.Errorf("mongo ping error: %w", err)
			return
		}

		database := client.Database(dbName)
		usersColl := database.Collection("users")
		messagesColl := database.Collection("messages")
		downloadsColl := database.Collection("downloads")
		broadcastsColl := database.Collection("broadcasts")

		indexModel := mongo.IndexModel{
			Keys:    bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		}
		_, _ = usersColl.Indexes().CreateOne(ctx, indexModel)

		ttlSeconds := int32(30 * 24 * 3600)
		msgTTLModel := mongo.IndexModel{
			Keys:    bson.D{{Key: "created_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(ttlSeconds),
		}
		_, _ = messagesColl.Indexes().CreateOne(ctx, msgTTLModel)

		dlTTLModel := mongo.IndexModel{
			Keys:    bson.D{{Key: "created_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(ttlSeconds),
		}
		_, _ = downloadsColl.Indexes().CreateOne(ctx, dlTTLModel)

		instance = &DB{
			client:      client,
			database:    database,
			users:       usersColl,
			messages:    messagesColl,
			downloads:   downloadsColl,
			broadcasts:  broadcastsColl,
			initialized: true,
		}
		log.Printf("[MongoDB] Connected successfully to database: %s", dbName)
	})

	return instance, initErr
}

func Get() *DB {
	return instance
}

func (d *DB) UpsertUser(ctx context.Context, userID int64, username, firstName, lastName, langCode string) (string, bool, error) {
	if d == nil || !d.initialized {
		return "", false, nil
	}

	now := time.Now()
	filter := bson.M{"user_id": userID}
	update := bson.M{
		"$set": bson.M{
			"username":       username,
			"first_name":     firstName,
			"last_name":      lastName,
			"language_code":  langCode,
			"last_active_at": now,
			"is_blocked":     false,
		},
		"$setOnInsert": bson.M{
			"created_at": now,
			"is_banned":  false,
		},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var doc User
	err := d.users.FindOneAndUpdate(ctx, filter, update, opts).Decode(&doc)
	if err != nil {
		return "", false, err
	}
	return doc.SelectedLanguage, doc.IsBanned, nil
}

func (d *DB) SetUserLanguage(ctx context.Context, userID int64, lang string) error {
	if d == nil || !d.initialized {
		return nil
	}
	filter := bson.M{"user_id": userID}
	update := bson.M{"$set": bson.M{"selected_language": lang, "last_active_at": time.Now()}}
	_, err := d.users.UpdateOne(ctx, filter, update)
	return err
}

func (d *DB) BanUser(ctx context.Context, userID int64, reason string) error {
	if d == nil || !d.initialized {
		return fmt.Errorf("database not initialized")
	}
	filter := bson.M{"user_id": userID}
	update := bson.M{"$set": bson.M{"is_banned": true, "ban_reason": reason}}
	_, err := d.users.UpdateOne(ctx, filter, update)
	return err
}

func (d *DB) UnbanUser(ctx context.Context, userID int64) error {
	if d == nil || !d.initialized {
		return fmt.Errorf("database not initialized")
	}
	filter := bson.M{"user_id": userID}
	update := bson.M{"$set": bson.M{"is_banned": false, "ban_reason": ""}}
	_, err := d.users.UpdateOne(ctx, filter, update)
	return err
}

func (d *DB) IsUserBanned(ctx context.Context, userID int64) (bool, string) {
	if d == nil || !d.initialized {
		return false, ""
	}
	var doc struct {
		IsBanned  bool   `bson:"is_banned"`
		BanReason string `bson:"ban_reason"`
	}
	err := d.users.FindOne(ctx, bson.M{"user_id": userID}).Decode(&doc)
	if err != nil {
		return false, ""
	}
	return doc.IsBanned, doc.BanReason
}

func (d *DB) LogMessage(ctx context.Context, userID, chatID, messageID int64, text string) error {
	if d == nil || !d.initialized {
		return nil
	}

	doc := MessageLog{
		UserID:    userID,
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
		CreatedAt: time.Now(),
	}

	_, err := d.messages.InsertOne(ctx, doc)
	return err
}

func (d *DB) LogDownload(ctx context.Context, userID int64, url, status string) error {
	if d == nil || !d.initialized {
		return nil
	}

	doc := DownloadLog{
		UserID:    userID,
		URL:       url,
		Status:    status,
		CreatedAt: time.Now(),
	}

	_, err := d.downloads.InsertOne(ctx, doc)
	return err
}

func (d *DB) RecordBroadcast(ctx context.Context, b BroadcastLog) error {
	if d == nil || !d.initialized {
		return nil
	}
	_, err := d.broadcasts.InsertOne(ctx, b)
	return err
}

func (d *DB) MarkUserBlocked(ctx context.Context, userID int64) error {
	if d == nil || !d.initialized {
		return nil
	}

	filter := bson.M{"user_id": userID}
	update := bson.M{"$set": bson.M{"is_blocked": true}}
	_, err := d.users.UpdateOne(ctx, filter, update)
	return err
}

func (d *DB) GetAllUserIDs(ctx context.Context, includeBlocked bool) ([]int64, error) {
	if d == nil || !d.initialized {
		return nil, fmt.Errorf("database is not configured")
	}

	filter := bson.M{"is_banned": bson.M{"$ne": true}}
	if !includeBlocked {
		filter["is_blocked"] = bson.M{"$ne": true}
	}

	opts := options.Find().SetProjection(bson.M{"user_id": 1})
	cursor, err := d.users.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []int64
	for cursor.Next(ctx) {
		var u struct {
			UserID int64 `bson:"user_id"`
		}
		if err := cursor.Decode(&u); err == nil && u.UserID != 0 {
			results = append(results, u.UserID)
		}
	}
	return results, cursor.Err()
}

func (d *DB) GetStats(ctx context.Context) (*Stats, error) {
	if d == nil || !d.initialized {
		return nil, fmt.Errorf("database is not configured")
	}

	totalUsers, err := d.users.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	blockedUsers, _ := d.users.CountDocuments(ctx, bson.M{"is_blocked": true})
	bannedUsers, _ := d.users.CountDocuments(ctx, bson.M{"is_banned": true})

	since24h := time.Now().Add(-24 * time.Hour)
	active24h, _ := d.users.CountDocuments(ctx, bson.M{"last_active_at": bson.M{"$gte": since24h}})

	totalMessages, _ := d.messages.CountDocuments(ctx, bson.M{})
	totalDownloads, _ := d.downloads.CountDocuments(ctx, bson.M{})

	return &Stats{
		TotalUsers:     totalUsers,
		Active24hUsers: active24h,
		BlockedUsers:   blockedUsers,
		BannedUsers:    bannedUsers,
		TotalMessages:  totalMessages,
		TotalDownloads: totalDownloads,
	}, nil
}
