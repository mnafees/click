# click

A terminal UI for ClickHouse databases, built with [Charm](https://charm.sh) libraries.

## Features

- Connect to any ClickHouse instance
- Browse databases and tables
- Run SQL queries with an interactive editor
- View results in formatted tables with query timing
- ClickHouse-themed yellow UI

## Installation

```sh
go install github.com/mnafees/click@latest
```

## Usage

```sh
click --host localhost --port 9000 --user default
```

## License

[MIT](LICENSE)
