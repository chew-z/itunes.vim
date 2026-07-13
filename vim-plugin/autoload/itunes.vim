" autoload/itunes.vim - Core logic for the itunes.vim plugin
" Talks to the `itunes` Go CLI via system() and feeds results to fzf.

if exists('g:itunes_autoloaded')
  finish
endif
let g:itunes_autoloaded = 1

if v:version < 800
  echoerr 'itunes.vim requires Vim 8.0 or later'
  finish
endif

" --- Configuration ----------------------------------------------------------

" Locate the itunes CLI binary.
function! s:find_bin() abort
  if exists('g:itunes_bin') && executable(g:itunes_bin)
    return g:itunes_bin
  endif

  let l:candidates = []

  if exists('$GOPATH')
    call add(l:candidates, $GOPATH . '/bin/itunes')
  endif

  call add(l:candidates, expand('~/Projekty/Go/src/itunes/bin/itunes'))
  call add(l:candidates, expand('~/go/src/itunes/bin/itunes'))
  call add(l:candidates, '/usr/local/bin/itunes')

  for l:candidate in l:candidates
    if executable(l:candidate)
      return l:candidate
    endif
  endfor

  if executable('itunes')
    return 'itunes'
  endif

  return ''
endfunction

" --- Internal helpers -------------------------------------------------------

function! s:bin() abort
  let l:bin = s:find_bin()
  if l:bin ==# ''
    echoerr 'itunes CLI not found. Set g:itunes_bin or add it to $PATH.'
    return ''
  endif
  return l:bin
endfunction

function! s:shellescape(s) abort
  return shellescape(a:s)
endfunction

" Parse JSON array from the CLI into a list of dicts.
" Vim 8 has no native JSON parsing from list, but json_decode works on strings.
function! s:parse_tracks(raw) abort
  let l:text = trim(a:raw)
  if l:text ==# '' || l:text ==# '[]'
    return []
  endif
  try
    return json_decode(l:text)
  catch
    echoerr 'itunes: failed to parse search results'
    return []
  endtry
endfunction

" Format one track dict into an fzf display line.
" Format: artist | album | track | ID
function! s:format_line(track) abort
  let l:artist = get(a:track, 'artist', '')
  let l:album  = get(a:track, 'album', '')
  let l:name   = get(a:track, 'name', '')
  let l:id     = get(a:track, 'persistent_id', get(a:track, 'id', ''))

  let l:parts = []
  if l:artist !=# ''
    call add(l:parts, l:artist)
  endif
  if l:album !=# ''
    call add(l:parts, l:album)
  endif
  call add(l:parts, l:name)
  call add(l:parts, l:id)

  return join(l:parts, ' | ')
endfunction

" Build a preview string from a track dict (single line for fzf preview file).
function! s:preview(track) abort
  let l:parts = []
  call add(l:parts, get(a:track, 'name', ''))
  let l:artist = get(a:track, 'artist', '')
  if l:artist !=# ''
    call add(l:parts, l:artist)
  endif
  let l:album = get(a:track, 'album', '')
  if l:album !=# ''
    call add(l:parts, l:album)
  endif
  let l:collection = get(a:track, 'collection', '')
  if l:collection !=# ''
    call add(l:parts, l:collection)
  endif
  let l:genre = get(a:track, 'genre', '')
  if l:genre !=# ''
    call add(l:parts, l:genre)
  endif
  return join(l:parts, ' | ')
endfunction

" fzf sink: extract tab-prefixed index, look up full track from s:track_list.
function! s:play_selection(line) abort
  let l:bin = s:bin()
  if l:bin ==# ''
    return
  endif

  let l:tab = stridx(a:line, "\t")
  if l:tab < 0
    echoerr 'itunes: could not parse selected line'
    return
  endif
  let l:idx = str2nr(a:line[:l:tab - 1])
  if l:idx < 0 || l:idx >= len(s:track_list)
    echoerr 'itunes: track index out of range'
    return
  endif

  let l:track = s:track_list[l:idx]
  let l:collection = get(l:track, 'collection', '')
  let l:track_name = get(l:track, 'name', '')
  let l:id = get(l:track, 'persistent_id', get(l:track, 'id', ''))

  let l:cmd = l:bin . ' play ' . s:shellescape(l:collection) . ' "" ' . s:shellescape(l:track_name) . ' ' . s:shellescape(l:id)
  let l:output = system(l:cmd)
  if v:shell_error
    echohl ErrorMsg
    echom 'itunes: play failed: ' . l:output
    echohl None
  else
    echo 'Playing: ' . l:track_name
  endif
endfunction

" --- Public API -------------------------------------------------------------

" Main entry point: search with fzf and play selected track.
function! itunes#search_and_play(query) abort
  if !executable('fzf')
    echoerr 'itunes.vim requires fzf (https://github.com/junegunn/fzf)'
    return
  endif
  let l:bin = s:bin()
  if l:bin ==# ''
    return
  endif

  let l:query = a:query
  if l:query ==# ''
    let l:query = input('Search: ')
    if l:query ==# ''
      return
    endif
  endif

  " Increase search limit for fzf (more choices are better)
  let l:limit = get(g:, 'itunes_search_limit', 50)
  let l:env_prefix = 'ITUNES_SEARCH_LIMIT=' . l:limit . ' '

  let l:raw = system(l:env_prefix . l:bin . ' search --json ' . s:shellescape(l:query))
  if v:shell_error
    echohl ErrorMsg
    echom 'itunes: search failed: ' . l:raw
    echohl None
    return
  endif

  let l:tracks = s:parse_tracks(l:raw)
  if empty(l:tracks)
    echo 'No tracks found for: ' . l:query
    return
  endif

  " Build fzf source lines with tab-prefixed index for reliable sink lookup
  let s:track_list = l:tracks
  let l:lines = []
  let l:preview_lines = []
  let l:idx = 0
  for l:track in l:tracks
    call add(l:lines, l:idx . "\t" . s:format_line(l:track))
    call add(l:preview_lines, s:preview(l:track))
    let l:idx += 1
  endfor

  " Write preview data to a temp file — one entry per line, indexed
  let l:preview_file = tempname()
  call writefile(l:preview_lines, l:preview_file)

  call fzf#run(fzf#wrap({
        \ 'source': l:lines,
        \ 'sink': function('s:play_selection'),
        \ 'options': [
        \   '-i',
        \   '--header', ':: ' . len(l:tracks) . ' tracks :: ENTER to play ::',
        \   '--delimiter', '\t',
        \   '--with-nth', '2..',
        \   '--nth', '2..',
        \   '--preview', 'awk -v n={1} ''NR==n+1'' ' . shellescape(l:preview_file),
        \   '--preview-window', 'right:40%:wrap',
        \   '--color', 'fg:15,bg:-1,hl:11,fg+:1,bg+:237,hl+:11',
        \   '--color', 'info:14,prompt:14,pointer:13,marker:13,spinner:13,header:11',
        \ ],
        \ }))
endfunction

" Show now-playing status.
function! itunes#status() abort
  let l:bin = s:bin()
  if l:bin ==# ''
    return
  endif

  let l:output = system(l:bin . ' now-playing')
  echo trim(l:output)
endfunction

" Play an entire playlist by name.
function! itunes#play_playlist(name) abort
  let l:bin = s:bin()
  if l:bin ==# ''
    return
  endif

  let l:name = a:name
  if l:name ==# ''
    let l:name = input('Playlist: ')
    if l:name ==# ''
      return
    endif
  endif

  let l:output = system(l:bin . ' play ' . s:shellescape(l:name))
  if v:shell_error
    echohl ErrorMsg
    echom 'itunes: play failed: ' . l:output
    echohl None
  else
    echo 'Playing playlist: ' . l:name
  endif
endfunction
