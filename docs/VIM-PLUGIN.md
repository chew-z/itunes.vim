# Vim Plugin

`vim-plugin/` contains a Vim/Neovim frontend for the `itunes` CLI: fzf-powered
library search and playback without leaving the editor. It replaces the
original itunes.vim plugin (still preserved on the `master` branch), which
used compiled JXA scripts and a text-file cache.

## How it fits the architecture

The plugin is a thin shell client — it never touches the database or JXA
directly:

```
Vim (:Tunes) ── system() ──> bin/itunes ──> SQLite (search)
                                       └──> JXA → Music app (play/status)
```

- `itunes search --json <query>` → JSON `[]Track` → `json_decode()` → fzf
- On selection: `itunes play <playlist> <album> <track> <persistent_id>`
- `:TunesStatus` → `itunes now-playing`

No vim-side caching: search hits the SQLite/FTS5 database, so results are
instant and always as fresh as the last `refresh_library`.

## Files

| File | Purpose |
|---|---|
| `vim-plugin/plugin/itunes.vim` | Command definitions (`:Tunes`, `:TunesStatus`, `:TunesPlay`) |
| `vim-plugin/autoload/itunes.vim` | Search, fzf integration, playback sink |
| `vim-plugin/README.md` | User-facing install and usage docs |

## Installation

vim-plug — note `rtp`, since the vim files live in a subdirectory of the Go
repo, and `branch: 'main'` (the `master` branch is the legacy plugin):

```vim
Plug 'junegunn/fzf', { 'do': { -> fzf#install() } }
Plug 'chew-z/itunes.vim', { 'branch': 'main', 'rtp': 'vim-plugin' }
```

Do **not** symlink `~/.vim/plugged/itunes.vim` to a working copy — vim-plug
manages that directory with git, and a non-repo path there breaks
`:PlugUpdate` (`could not create work tree dir`).

Requirements: Vim 8.0+/Neovim, `fzf` binary in `$PATH` (only for `:Tunes`;
status and playlist playback work without it), and a built `bin/itunes`.
The binary is auto-detected (`$GOPATH/bin`, common repo paths, `$PATH`) or
set explicitly with `g:itunes_bin`.

## Design notes

- **Index-based fzf sink.** Display lines are prefixed with a hidden
  `<index>\t`; the sink parses the index and looks up the full track dict in
  a script-local list. Never re-parse display text — track/artist/album names
  may contain the ` | ` display delimiter.
- **Playlist context from `playlists[]`, not `collection`.** A track's
  `collection` field is usually its album name; passing it as the CLI's
  playlist argument fails with "Playlist not found". The sink uses
  `playlists[0]` when the track belongs to a real playlist and otherwise
  passes the album in the album slot.
- **fzf preview via indexed temp file.** One preview line per track, looked
  up with `awk -v n={1} 'NR==n+1'`. fzf substitutes `{1}` wrapped in single
  quotes and including the field delimiter, which breaks shell arithmetic like
  `$(({1}+1))` — awk's numeric coercion tolerates both.
- **`ITUNES_SEARCH_LIMIT` env override** raises the CLI's result cap (default
  15) to `g:itunes_search_limit` (default 50) so fzf has enough candidates.

## CLI contract

The plugin depends on:

- `search --json` emitting a JSON array (`[]` for no results, never `null`)
  with `name`, `artist`, `album`, `collection`, `playlists`, `persistent_id`.
- `play <playlist> <album> <track> <trackID>` positional order, empty strings
  allowed for unused slots.
- Non-zero exit code on failure with the error message on stdout.

Changing any of these in `itunes.go` requires a matching plugin update.
