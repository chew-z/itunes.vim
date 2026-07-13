# itunes.vim

Vim plugin for Apple Music, powered by the `itunes` Go CLI with SQLite/FTS5 search and fzf fuzzy picking.

## Requirements

- **Vim 8.0+** or **Neovim**
- **[fzf](https://github.com/junegunn/fzf)** in `$PATH` (includes vim integration via `fzf#run`)
- The `itunes` CLI binary (built from this repo: `go build -o bin/itunes .`)

## Installation

### vim-plug

```vim
Plug 'junegunn/fzf', { 'do': { -> fzf#install() } }
Plug 'chew-z/itunes.vim', { 'branch': 'main', 'rtp': 'vim-plugin' }
```

### Manual

Copy `vim-plugin/plugin/itunes.vim` and `vim-plugin/autoload/itunes.vim` into `~/.vim/`.

## Configuration

```vim
" Path to the itunes CLI binary (auto-detected if not set)
let g:itunes_bin = '~/Projekty/Go/src/itunes/bin/itunes'

" Max search results fed to fzf (default: 50)
let g:itunes_search_limit = 100
```

## Commands

| Command | Description |
|---|---|
| `:Tunes [query]` | Fuzzy-search the library and play the selected track |
| `:TunesStatus` | Show currently playing track in the echo area |
| `:TunesPlay [playlist]` | Play an entire playlist |

## Usage

1. Type `:Tunes beatles` (or just `:Tunes` and enter a query at the prompt)
2. fzf opens with matching tracks: `artist | album | track`
3. Press `Enter` to play the selected track

The search uses FTS5 full-text search across track name, artist, album, and genre — results are ranked by relevance, popularity, and recency.

## How it works

The plugin shells out to the `itunes` CLI:

- `itunes search --json <query>` returns a JSON array of tracks
- The vim plugin parses JSON, feeds lines to fzf, and on selection calls `itunes play <playlist> "" <track> <persistent_id>`
- `itunes now-playing` returns the current playback status

No vim-side caching is needed — the CLI uses a persistent SQLite database.

## Differences from the old plugin

| Old (JXA + text cache) | New (Go CLI + SQLite) |
|---|---|
| Two compiled `.scpt` scripts | Single CLI binary |
| Text-file cache (`iTunes_Library_Cache.txt`) | SQLite FTS5 — no vim-side cache |
| Playlist-only search | Full-text search across all fields |
| Async JXA refresh (slow first run) | Instant search from indexed database |
| No now-playing | `:TunesStatus` |
