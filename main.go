package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/mnafees/click/internal/config"
	"github.com/mnafees/click/internal/db"
	"github.com/mnafees/click/internal/tui"
)

func main() {
	profile := flag.String("profile", "", "connection profile from ~/.clickrc")
	host := flag.String("host", "localhost", "ClickHouse host")
	port := flag.Int("port", 9000, "ClickHouse native port")
	user := flag.String("user", "default", "ClickHouse user")
	password := flag.String("password", "", "ClickHouse password")
	database := flag.String("database", "default", "ClickHouse database")
	flag.Parse()

	var cfg db.Config
	if *profile != "" {
		var ok bool
		cfg, ok = config.LoadProfile(*profile)
		if !ok {
			fmt.Fprintf(os.Stderr, "profile %q not found in ~/.clickrc\n", *profile)
			os.Exit(1)
		}
	} else {
		cfg = db.Config{
			Host:     *host,
			Port:     *port,
			User:     *user,
			Password: *password,
			Database: *database,
		}
	}

	client, err := db.Connect(context.Background(), cfg)
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
