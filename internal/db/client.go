package db

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

type Client struct {
	conn     driver.Conn
	database string
}

type QueryResult struct {
	Columns     []string
	ColumnTypes []string // ClickHouse database type names, parallel to Columns
	Rows        [][]string
	Duration    time.Duration
}

func Connect(ctx context.Context, cfg Config) (*Client, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.User,
			Password: cfg.Password,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Client{conn: conn, database: cfg.Database}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Tables(ctx context.Context) ([]string, error) {
	start := time.Now()
	rows, err := c.conn.Query(ctx, "SHOW TABLES FROM "+c.database)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	_ = time.Since(start)
	return tables, rows.Err()
}

func (c *Client) Query(ctx context.Context, query string) (*QueryResult, error) {
	start := time.Now()
	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols := rows.ColumnTypes()
	columns := make([]string, len(cols))
	columnTypes := make([]string, len(cols))
	for i, col := range cols {
		columns[i] = col.Name()
		columnTypes[i] = col.DatabaseTypeName()
	}

	var resultRows [][]string
	for rows.Next() {
		ptrs := make([]any, len(cols))
		for i, col := range cols {
			ptrs[i] = reflect.New(col.ScanType()).Interface()
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make([]string, len(columns))
		for i, p := range ptrs {
			val := reflect.ValueOf(p).Elem().Interface()
			if t, ok := val.(time.Time); ok {
				row[i] = t.UTC().Format(time.RFC3339Nano)
			} else {
				row[i] = fmt.Sprintf("%v", val)
			}
		}
		resultRows = append(resultRows, row)
	}

	return &QueryResult{
		Columns:     columns,
		ColumnTypes: columnTypes,
		Rows:        resultRows,
		Duration:    time.Since(start),
	}, rows.Err()
}
