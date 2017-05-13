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

let s:jxa_folder = expand('<sfile>:p:h')
echom s:jxa_folder
let s:jxa = {
\ 'Play':       s:jxa_folder . '/iTunes_Play_Track.scpt',
\ 'Search':     s:jxa_folder . '/iTunes_Search_fzf.scpt'
\ }
echom s:jxa.Play
echom s:jxa.Search


function! s:handler(line)
    let l:track = split(a:line, ' | ')
    let l:title = l:track[len(l:track)-1]
    echom join(l:track, ' ')
    " This is never called unless we re-bind Enter in fzf
    call system('osascript -l JavaScript ' . s:jxa.Play. l:title)
endfunction

function! itunes#search_and_play(args)
    call fzf#run({
    \ 'source':  'osascript -l JavaScript ' . s:jxa.Search .  ' ' . a:args,
    \ 'sink':   function('s:handler'),
    \ 'options': '--header "Enter to play track. Esc to exit."' . ' --bind "enter:execute-silent(echo -n {} | gsed -e ''s/^.*| //g''  | xargs osascript -l JavaScript ' .  s:jxa.Play . ')" ' . ' --preview="echo -e {} | tr ''|'' ''\n'' | sed -e ''s/^ //g'' | tail -r " ' . ' --preview-window down:4:wrap' . ' --bind "?:toggle-preview"'
    \ })
endfunction
