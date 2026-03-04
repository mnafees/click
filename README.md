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

## Keybindings

| Key | Action |
|---|---|
| `tab` | Cycle between tables, query editor, and results |
| `enter` | Select a table (runs `SELECT * ... LIMIT 100`) |
| `ctrl+r` | Run the query in the editor |
| `ctrl+x` | Toggle expanded display (vertical, like psql `\x`) |
| `ctrl+u` | Toggle datetime columns between local time and UTC |
| `j/k` or arrows | Scroll vertically (tables list or results) |
| `h/l` or arrows | Scroll results horizontally |
| `q` | Quit (outside the query editor) |
| `ctrl+c` | Quit |

## Features

- Server info bar showing ClickHouse version, connection endpoint, and uptime
- Table browser in the sidebar -- select a table to preview its contents
- Freeform SQL editor with results displayed alongside column types
- Sticky column headers that stay visible while scrolling
- Horizontal and vertical scrolling for wide or long result sets
- Expanded record view for inspecting rows one at a time
- Datetime timezone toggle between local time and UTC
- Query stats: row count, data size, and elapsed time

## License

[MIT](LICENSE)
