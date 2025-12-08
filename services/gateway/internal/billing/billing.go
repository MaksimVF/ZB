








package billing

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

var (
	db       *sql.DB
	billMutex = &sync.Mutex{}
)

type UsageRecord struct {
	ID        string
	UserID    string
	Model     string
	Tokens    int
	Timestamp time.Time
	Cost      float64
}

func Init(dbConnString string) error {
	var err error
	db, err = sql.Open("postgres", dbConnString)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create usage table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS langchain_usage (
			id SERIAL PRIMARY KEY,
			user_id TEXT NOT NULL,
			model TEXT NOT NULL,
			tokens INTEGER NOT NULL,
			timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			cost NUMERIC(10, 2) NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create usage table: %w", err)
	}

	return nil
}

func TrackUsage(userID, model string, tokens int) error {
	billMutex.Lock()
	defer billMutex.Unlock()

	// Calculate cost based on model and token count
	cost := calculateCost(model, tokens)

	_, err := db.Exec(`
		INSERT INTO langchain_usage (user_id, model, tokens, cost)
		VALUES ($1, $2, $3, $4)
	`, userID, model, tokens, cost)

	if err != nil {
		return fmt.Errorf("failed to record usage: %w", err)
	}

	return nil
}

func GetUsageReport(userID string, start, end time.Time) ([]UsageRecord, error) {
	billMutex.Lock()
	defer billMutex.Unlock()

	rows, err := db.Query(`
		SELECT id, user_id, model, tokens, timestamp, cost
		FROM langchain_usage
		WHERE user_id = $1 AND timestamp BETWEEN $2 AND $3
		ORDER BY timestamp DESC
	`, userID, start, end)

	if err != nil {
		return nil, fmt.Errorf("failed to get usage report: %w", err)
	}
	defer rows.Close()

	var records []UsageRecord
	for rows.Next() {
		var r UsageRecord
		if err := rows.Scan(&r.ID, &r.UserID, &r.Model, &r.Tokens, &r.Timestamp, &r.Cost); err != nil {
			return nil, fmt.Errorf("failed to scan usage record: %w", err)
		}
		records = append(records, r)
	}

	return records, nil
}

func calculateCost(model string, tokens int) float64 {
	// Pricing per 1000 tokens
	pricing := map[string]float64{
		"gpt-4":     0.06,
		"gpt-3.5":   0.002,
		"claude-3":   0.04,
		"gemini-1.5": 0.03,
		"llama-3":    0.02,
	}

	price, ok := pricing[model]
	if !ok {
		price = 0.01 // default price
	}

	return float64(tokens) / 1000 * price
}

func GetTotalCost(userID string, start, end time.Time) (float64, error) {
	billMutex.Lock()
	defer billMutex.Unlock()

	var total float64
	err := db.QueryRow(`
		SELECT SUM(cost)
		FROM langchain_usage
		WHERE user_id = $1 AND timestamp BETWEEN $2 AND $3
	`, userID, start, end).Scan(&total)

	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to calculate total cost: %w", err)
	}

	return total, nil
}

func Close() {
	if db != nil {
		db.Close()
	}
}





