# click

A TUI for ClickHouse databases. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

## Install

```sh
go install github.com/mnafees/click@latest
```

## Usage

```sh
click --host localhost --port 9000 --user default --password secret --database mydb
```

All flags are optional and default to `localhost:9000` with user `default` and database `default`.

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

## What it does

- Connects to ClickHouse over the native protocol
- Lists tables in the sidebar; select one to browse its contents
- Query editor with freeform SQL, results shown with column types in the header
- Sticky column headers that stay visible while scrolling
- Horizontal and vertical scrolling for wide/long result sets
- Expanded record view for inspecting individual rows
- Datetime timezone toggle (local/UTC)
- Query timing displayed with row count

## License

[MIT](LICENSE)
