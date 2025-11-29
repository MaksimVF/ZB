








package billing

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB() (*sql.DB, error) {
	// Create an in-memory SQLite database for testing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}

	// Create the usage table
	_, err = db.Exec(`
		CREATE TABLE langchain_usage (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			model TEXT NOT NULL,
			tokens INTEGER NOT NULL,
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			cost REAL NOT NULL
		)
	`)

	return db, err
}

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		model   string
		tokens  int
		expected float64
	}{
		{"gpt-4", 1000, 0.06},
		{"gpt-3.5", 1000, 0.002},
		{"claude-3", 1000, 0.04},
		{"gemini-1.5", 1000, 0.03},
		{"llama-3", 1000, 0.02},
		{"unknown-model", 1000, 0.01}, // default price
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			cost := calculateCost(tt.model, tt.tokens)
			assert.Equal(t, tt.expected, cost)
		})
	}
}

func TestTrackUsage(t *testing.T) {
	db, err := setupTestDB()
	require.NoError(t, err)
	defer db.Close()

	// Replace the global db with our test db
	originalDB := db
	db = db
	defer func() { db = originalDB }()

	err = TrackUsage("test-user", "gpt-4", 1000)
	require.NoError(t, err)

	// Verify the usage was recorded
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM langchain_usage WHERE user_id = ?", "test-user").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	var cost float64
	err = db.QueryRow("SELECT cost FROM langchain_usage WHERE user_id = ?", "test-user").Scan(&cost)
	require.NoError(t, err)
	assert.Equal(t, 0.06, cost)
}

func TestGetUsageReport(t *testing.T) {
	db, err := setupTestDB()
	require.NoError(t, err)
	defer db.Close()

	// Replace the global db with our test db
	originalDB := db
	db = db
	defer func() { db = originalDB }()

	// Add some test data
	_, err = db.Exec(`
		INSERT INTO langchain_usage (user_id, model, tokens, cost)
		VALUES
			('test-user', 'gpt-4', 1000, 0.06),
			('test-user', 'gpt-3.5', 2000, 0.004),
			('other-user', 'gpt-4', 500, 0.03)
	`)
	require.NoError(t, err)

	// Test getting usage report
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()

	records, err := GetUsageReport("test-user", start, end)
	require.NoError(t, err)
	assert.Len(t, records, 2)

	// Verify the records
	assert.Equal(t, "test-user", records[0].UserID)
	assert.Equal(t, 3000, records[0].Tokens+records[1].Tokens)
	assert.Equal(t, 0.064, records[0].Cost+records[1].Cost)
}

func TestGetTotalCost(t *testing.T) {
	db, err := setupTestDB()
	require.NoError(t, err)
	defer db.Close()

	// Replace the global db with our test db
	originalDB := db
	db = db
	defer func() { db = originalDB }()

	// Add some test data
	_, err = db.Exec(`
		INSERT INTO langchain_usage (user_id, model, tokens, cost)
		VALUES
			('test-user', 'gpt-4', 1000, 0.06),
			('test-user', 'gpt-3.5', 2000, 0.004),
			('other-user', 'gpt-4', 500, 0.03)
	`)
	require.NoError(t, err)

	// Test getting total cost
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()

	total, err := GetTotalCost("test-user", start, end)
	require.NoError(t, err)
	assert.Equal(t, 0.064, total)
}





