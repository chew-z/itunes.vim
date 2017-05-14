scriptencoding utf-8
" Location: autoload/thrasher/itunes.vim
" Author:   Robert Jakubowski <https://github.com/chew-z>
" 
if exists('g:itunes_autoloaded')
    finish
endif
let g:itunes_autoloaded = 1

if !executable('osascript')
    echoerr ('itunes.vim: Cannot find osascript')
    finish
endif

if !executable('fzf')
    echoerr ('itunes.vim: Cannot find fzf')
    finish
endif

if !exists('g:itunes_online')
    let g:itunes_online = 0
endif

let s:jxa_folder = expand('<sfile>:p:h')
let s:jxa = {
\ 'Play':       s:jxa_folder . '/iTunes_Play_Playlist_Track.scpt',
\ 'Search':     s:jxa_folder . '/iTunes_Search_fzf.scpt'
\ }

function! s:handler(line)
    let l:track = split(a:line, ' | ')
    let l:title = l:track[len(l:track)-1]
    let l:playlist = substitute(l:track[0], ' $', '', '')
    " This is never called unless we re-bind Enter in fzf
    let l:cmd = 'osascript -l JavaScript ' . s:jxa.Play . shellescape(l:playlist) . ' ' . shellescape(l:title)
    call system(l:cmd)
    "call system('osascript -l JavaScript ' . s:jxa.Play . l:title)
endfunction

function! itunes#search_and_play(args)
    if g:itunes_online
        let l:args = 'Online ' . a:args
    else
        let l:args = 'Offline ' . a:args
    endif
    call fzf#run({
    \ 'source':  'osascript -l JavaScript ' . s:jxa.Search .  ' ' . l:args,
    \ 'sink':   function('s:handler'),
    \ 'options': '--header "Enter to play track. Esc to exit."' 
        \. ' --bind "enter:execute-silent(echo -n {} | gsed -e ''s/^\(.*\) | \(.*\) | \(.*\) | \(.*$\)/\"\1\" \"\4\"/'' | xargs osascript -l JavaScript ' .  s:jxa.Play . ')" ' .
        \ ' --preview="echo -e {} | tr ''|'' ''\n'' | sed -e ''s/^ //g'' | tail -r " ' .
        \ ' --preview-window down:4:wrap' .
        \ ' --bind "?:toggle-preview"'
    \ })
endfunction
