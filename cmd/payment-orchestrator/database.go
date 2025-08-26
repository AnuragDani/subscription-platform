package main

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	conn *sql.DB
}

type Transaction struct {
	ID                     string    `json:"id"`
	SubscriptionID         string    `json:"subscription_id"`
	PaymentMethodID        string    `json:"payment_method_id"`
	ProcessorUsed          string    `json:"processor_used"`
	Amount                 float64   `json:"amount"`
	Currency               string    `json:"currency"`
	Status                 string    `json:"status"`
	TransactionType        string    `json:"transaction_type"`
	IdempotencyKey         string    `json:"idempotency_key"`
	ProcessorTransactionID string    `json:"processor_transaction_id,omitempty"`
	OriginalTransactionID  *string   `json:"original_transaction_id,omitempty"`
	ErrorCode              string    `json:"error_code,omitempty"`
	UserErrorMessage       string    `json:"user_error_message,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
}

type PaymentMethod struct {
	ID              string `json:"id"`
	UserID          string `json:"user_id"`
	NetworkToken    string `json:"network_token,omitempty"`
	ProcessorAToken string `json:"processor_a_token,omitempty"`
	ProcessorBToken string `json:"processor_b_token,omitempty"`
	TokenType       string `json:"token_type"`
	LastFour        string `json:"last_four"`
}

func NewDB(connectionString string) (*DB, error) {
	conn, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	if err := conn.Ping(); err != nil {
		return nil, err
	}

	return &DB{conn: conn}, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Ping() error {
	return db.conn.Ping()
}

func (db *DB) CreateTransaction(ctx context.Context, t *Transaction) error {
	query := `
		INSERT INTO transactions (
			id, subscription_id, payment_method_id, processor_used,
			amount, currency, status, idempotency_key,
			processor_transaction_id, original_transaction_id,
			error_code, user_error_message, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (idempotency_key) DO NOTHING`

	_, err := db.conn.ExecContext(ctx, query,
		t.ID, t.SubscriptionID, t.PaymentMethodID, t.ProcessorUsed,
		t.Amount, t.Currency, t.Status, t.IdempotencyKey,
		sql.NullString{String: t.ProcessorTransactionID, Valid: t.ProcessorTransactionID != ""},
		t.OriginalTransactionID,
		sql.NullString{String: t.ErrorCode, Valid: t.ErrorCode != ""},
		sql.NullString{String: t.UserErrorMessage, Valid: t.UserErrorMessage != ""},
		time.Now(),
	)
	return err
}

func (db *DB) GetTransaction(ctx context.Context, id string) (*Transaction, error) {
	query := `
		SELECT id, subscription_id, payment_method_id, processor_used,
			   amount, currency, status, idempotency_key,
			   processor_transaction_id, original_transaction_id,
			   error_code, user_error_message, created_at
		FROM transactions WHERE id = $1`

	var t Transaction
	var processorTxID, errorCode, errorMessage sql.NullString
	var originalTxID sql.NullString

	err := db.conn.QueryRowContext(ctx, query, id).Scan(
		&t.ID, &t.SubscriptionID, &t.PaymentMethodID, &t.ProcessorUsed,
		&t.Amount, &t.Currency, &t.Status, &t.IdempotencyKey,
		&processorTxID, &originalTxID,
		&errorCode, &errorMessage, &t.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	if processorTxID.Valid {
		t.ProcessorTransactionID = processorTxID.String
	}
	if originalTxID.Valid {
		t.OriginalTransactionID = &originalTxID.String
	}
	if errorCode.Valid {
		t.ErrorCode = errorCode.String
	}
	if errorMessage.Valid {
		t.UserErrorMessage = errorMessage.String
	}

	return &t, nil
}

func (db *DB) GetTransactionByIdempotencyKey(ctx context.Context, key string) (*Transaction, error) {
	query := `
		SELECT id, subscription_id, payment_method_id, processor_used,
			   amount, currency, status, idempotency_key,
			   processor_transaction_id, original_transaction_id,
			   error_code, user_error_message, created_at
		FROM transactions WHERE idempotency_key = $1`

	var t Transaction
	var processorTxID, errorCode, errorMessage sql.NullString
	var originalTxID sql.NullString

	err := db.conn.QueryRowContext(ctx, query, key).Scan(
		&t.ID, &t.SubscriptionID, &t.PaymentMethodID, &t.ProcessorUsed,
		&t.Amount, &t.Currency, &t.Status, &t.IdempotencyKey,
		&processorTxID, &originalTxID,
		&errorCode, &errorMessage, &t.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	if processorTxID.Valid {
		t.ProcessorTransactionID = processorTxID.String
	}
	if originalTxID.Valid {
		t.OriginalTransactionID = &originalTxID.String
	}
	if errorCode.Valid {
		t.ErrorCode = errorCode.String
	}
	if errorMessage.Valid {
		t.UserErrorMessage = errorMessage.String
	}

	return &t, nil
}

func (db *DB) GetPaymentMethod(ctx context.Context, id string) (*PaymentMethod, error) {
	query := `
		SELECT id, user_id, network_token, processor_a_token, processor_b_token,
			   token_type, last_four
		FROM payment_methods WHERE id = $1`

	var pm PaymentMethod
	var networkToken, processorAToken, processorBToken sql.NullString

	err := db.conn.QueryRowContext(ctx, query, id).Scan(
		&pm.ID, &pm.UserID, &networkToken, &processorAToken, &processorBToken,
		&pm.TokenType, &pm.LastFour,
	)

	if err != nil {
		return nil, err
	}

	if networkToken.Valid {
		pm.NetworkToken = networkToken.String
	}
	if processorAToken.Valid {
		pm.ProcessorAToken = processorAToken.String
	}
	if processorBToken.Valid {
		pm.ProcessorBToken = processorBToken.String
	}

	return &pm, nil
}

func (db *DB) GetTransactionStats(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(*) as total_transactions,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as successful,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed,
			SUM(amount) as total_volume,
			AVG(amount) as avg_transaction_size
		FROM transactions
		WHERE created_at > NOW() - INTERVAL '24 hours'`

	var stats struct {
		TotalTransactions  sql.NullInt64
		Successful         sql.NullInt64
		Failed             sql.NullInt64
		TotalVolume        sql.NullFloat64
		AvgTransactionSize sql.NullFloat64
	}

	err := db.conn.QueryRowContext(ctx, query).Scan(
		&stats.TotalTransactions,
		&stats.Successful,
		&stats.Failed,
		&stats.TotalVolume,
		&stats.AvgTransactionSize,
	)

	if err != nil {
		return nil, err
	}

	totalTx := int64(0)
	if stats.TotalTransactions.Valid {
		totalTx = stats.TotalTransactions.Int64
	}

	successful := int64(0)
	if stats.Successful.Valid {
		successful = stats.Successful.Int64
	}

	failed := int64(0)
	if stats.Failed.Valid {
		failed = stats.Failed.Int64
	}

	successRate := float64(0)
	if totalTx > 0 {
		successRate = float64(successful) / float64(totalTx) * 100
	}

	return map[string]interface{}{
		"total_transactions":   totalTx,
		"successful":           successful,
		"failed":               failed,
		"success_rate":         successRate,
		"total_volume":         stats.TotalVolume.Float64,
		"avg_transaction_size": stats.AvgTransactionSize.Float64,
	}, nil
}
