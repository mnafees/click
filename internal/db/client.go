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
	cfg      Config
	database string
}

type ServerInfo struct {
	Version  string
	Uptime   string
	Host     string
	Port     int
	Database string
}

type TableStats struct {
	Rows      uint64
	DiskBytes uint64
}

type QueryResult struct {
	Columns     []string
	ColumnTypes []string // ClickHouse database type names, parallel to Columns
	Rows        [][]string
	BytesRead   uint64
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
	return &Client{conn: conn, cfg: cfg, database: cfg.Database}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) ServerInfo(ctx context.Context) (ServerInfo, error) {
	var version string
	var uptime uint32
	row := c.conn.QueryRow(ctx, "SELECT version() AS version, uptime() AS uptime")
	if err := row.Scan(&version, &uptime); err != nil {
		return ServerInfo{}, err
	}
	return ServerInfo{
		Version:  version,
		Uptime:   formatUptime(uint64(uptime)),
		Host:     c.cfg.Host,
		Port:     c.cfg.Port,
		Database: c.database,
	}, nil
}

func formatUptime(seconds uint64) string {
	d := seconds / 86400
	h := (seconds % 86400) / 3600
	m := (seconds % 3600) / 60
	if d > 0 {
		return fmt.Sprintf("%dd %dh %dm", d, h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func (c *Client) Databases(ctx context.Context) ([]string, error) {
	rows, err := c.conn.Query(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dbs []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		dbs = append(dbs, name)
	}
	return dbs, rows.Err()
}

func (c *Client) SwitchDatabase(name string) {
	c.database = name
}

func (c *Client) Database() string {
	return c.database
}

func (c *Client) TableStatsBatch(ctx context.Context) (map[string]TableStats, error) {
	rows, err := c.conn.Query(ctx,
		"SELECT name, coalesce(total_rows, 0), coalesce(total_bytes, 0) FROM system.tables WHERE database = @db",
		clickhouse.Named("db", c.database))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stats := make(map[string]TableStats)
	for rows.Next() {
		var name string
		var totalRows, totalBytes uint64
		if err := rows.Scan(&name, &totalRows, &totalBytes); err != nil {
			continue
		}
		stats[name] = TableStats{Rows: totalRows, DiskBytes: totalBytes}
	}
	return stats, rows.Err()
}

func (c *Client) DescribeTable(ctx context.Context, table string) (*QueryResult, error) {
	return c.Query(ctx, "DESCRIBE TABLE "+table)
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

	var bytesRead uint64
	for _, row := range resultRows {
		for _, cell := range row {
			bytesRead += uint64(len(cell))
		}
	}

	return &QueryResult{
		Columns:     columns,
		ColumnTypes: columnTypes,
		Rows:        resultRows,
		BytesRead:   bytesRead,
		Duration:    time.Since(start),
	}, rows.Err()
}
