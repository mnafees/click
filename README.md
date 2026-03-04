# click

A TUI for ClickHouse. Connects over the native protocol, shows server info at a glance, and lets you browse tables and run queries without leaving the terminal.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

## Install

```sh
go install github.com/mnafees/click@latest
```

## Usage

```sh
click --host localhost --port 9000 --user default --password secret --database mydb
```

All flags are optional. Defaults to `localhost:9000`, user `default`, database `default`.

### Connection profiles

Create `~/.clickrc` to store named profiles:

```json
{
  "profiles": {
    "prod": {
      "host": "clickhouse.prod.internal",
      "port": 9000,
      "user": "readonly",
      "password": "secret",
      "database": "analytics"
    },
    "local": {
      "host": "localhost"
    }
  }
}
```

Then connect with `click --profile prod`.

## Keybindings

| Key | Action |
|---|---|
| `tab` | Cycle between tables, query editor, and results |
| `enter` | Select a table (runs `SELECT * ... LIMIT 100`) |
| `ctrl+r` | Run the query in the editor |
| `ctrl+d` | Describe the selected table (show columns and types) |
| `ctrl+b` | Switch database |
| `ctrl+x` | Toggle expanded display (vertical, like psql `\x`) |
| `ctrl+u` | Toggle datetime columns between local time and UTC |
| `ctrl+e` | Toggle EXPLAIN mode (prepends EXPLAIN to queries) |
| `ctrl+p` / `ctrl+n` | Browse query history (previous / next) |
| `ctrl+s` | Export results to CSV |
| `ctrl+j` | Export results to JSON |
| `j/k` or arrows | Scroll vertically (tables list or results) |
| `h/l` or arrows | Scroll results horizontally |
| mouse scroll | Scroll results |
| `q` | Quit (outside the query editor) |
| `ctrl+c` | Quit |

## Features

- Server info bar showing ClickHouse version, connection endpoint, and uptime
- Table browser with row counts and disk usage next to each table name
- Describe table view showing column definitions
- Database switcher to jump between databases without restarting
- Freeform SQL editor with results displayed alongside column types
- Query history persisted to `~/.click_history`, navigable with ctrl+p/n
- Sticky column headers that stay visible while scrolling
- Horizontal and vertical scrolling for wide or long result sets
- Mouse scroll support in the results view
- Expanded record view for inspecting rows one at a time
- Datetime timezone toggle between local time and UTC
- EXPLAIN mode to inspect query plans
- Export results to CSV or JSON
- Confirmation prompt for dangerous queries (DROP, TRUNCATE, ALTER, DELETE)
- Query stats: row count, data size, and elapsed time
- Connection profiles via `~/.clickrc`

## License

[MIT](LICENSE)
