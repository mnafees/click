package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/mnafees/click/internal/db"
	"github.com/mnafees/click/internal/tui"
)

func main() {
	host := flag.String("host", "localhost", "ClickHouse host")
	port := flag.Int("port", 9000, "ClickHouse native port")
	user := flag.String("user", "default", "ClickHouse user")
	password := flag.String("password", "", "ClickHouse password")
	database := flag.String("database", "default", "ClickHouse database")
	flag.Parse()

	client, err := db.Connect(context.Background(), db.Config{
		Host:     *host,
		Port:     *port,
		User:     *user,
		Password: *password,
		Database: *database,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "connection failed: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	if err := tui.Run(client); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
