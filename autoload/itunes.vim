scriptencoding utf-8
" Location: autoload/thrasher/itunes.vim
" Author:   Robert Jakubowski <https://github.com/chew-z>
" 
if exists('g:autoloaded_itunes')
    finish
endif
let g:autoloaded_itunes = 1

if !executable('osascript')
    echoerr ('itunes.vim: Cannot find osascript')
    finish
endif

if !executable('fzf')
    echoerr ('itunes.vim: Cannot find fzf')
    finish
endif

let s:jxa_folder = expand('<sfile>:p:h')
let g:itunes_jxa = {
\ 'Play':       s:dir . '/iTunes_Play_Track.scpt',
\ 'Search':     s:dir . '/iTunes_Search_fzf.scpt'
\ }

function! itunes#handler(line)
    let l:track = split(a:line, ' | ')
    let l:title = l:track[len(l:track)-1]
    echom join(l:track, ' ')
    " This is never called unless we re-bind Enter in fzf
    call system('osascript -l JavaScript ' . g:itunes_jxa.Play . l:title)
endfunction

