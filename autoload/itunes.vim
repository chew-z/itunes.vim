scriptencoding utf-8
" Location: autoload/itunes/itunes.vim
" Author:   Robert Jakubowski <https://github.com/chew-z>
" 
if exists('g:itunes_autoloaded') || v:version < 800
    if v:version < 800
        echoerr 'itunes.vim: itunes#refresh() is using async and requires VIM version 8 or higher'
    endif
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

if !exists('g:itunes_verbose')
    let g:itunes_verbose = 0
endif


let s:cache = []
let s:jxa_folder = expand('<sfile>:p:h')
let s:jxa = {
\ 'Play':       s:jxa_folder .  '/iTunes_Play_Playlist_Track.scpt',
\ 'Search':     s:jxa_folder .  '/iTunes_Search_fzf.scpt',
\ 'Search2':    s:jxa_folder .  '/iTunes_Search2_fzf.scpt',
\ 'Cache':      s:jxa_folder .  'Library_cache.txt'
\ }
if g:itunes_verbose
    echom s:jxa_folder
    echom s:jxa.Play
    echom s:jxa.Search
    echom s:jxa.Cache
endif

" Helper functions
command! -nargs=1 Silent execute ':silent !'.<q-args> | execute ':redraw!'

function! s:saveVariable(var, file)
    " if !filewritable(a:file) | Silent touch a:file | endif
    call writefile([string(a:var)], a:file)
endfunction

function! s:restoreVariable(file)
    if filereadable(a:file)
        let l:recover = readfile(a:file)[0]
    else
        echoerr string(a:file) . ' not readable. Cannot restore variable!'
    endif
    execute 'let l:result = ' . l:recover
    return l:result
endfunction

" Async helpers

function! RefreshLibrary_JobEnd(channel)
    let s:cache = s:restoreVariable(g:itunes_refreshLibrary)
    if !filewritable(jxa_folder.Cache)
        Silent touch jxa_folder.Cache
    endif
    call s:saveVariable(s:cache,jxa_folder.Cache)
    " call itunes#refreshList()
    if filereadable(jxa_folder.Cache) | echom 'iTunes Library refreshed' | endif
    unlet g:itunes_refreshLibrary
endfunction

function! s:refreshLibrary(jxa_exec, mode)
    if exists('g:itunes_refreshLibrary')
        if g:itunes_verbose | echom 'refreshLibrary task is already running in background' | endif
    else
        let g:itunes_refreshLibrary = tempname()
        let l:cmd = ['osascript', '-l', 'JavaScript',  a:jxa_exec, a:mode]
        
        if g:itunes_verbose | echom string(l:cmd) | endif
        if g:itunes_verbose | echom string(g:itunes_refreshLibrary) | endif
        if g:itunes_verbose | echom 'Refreshing iTunes Library in background' | endif
        
        let l:job = job_start(l:cmd, {'close_cb': 'RefreshLibrary_JobEnd', 'out_io': 'file', 'out_name': g:itunes_refreshLibrary})
    endif
endfunction

" FZF sink function

function! s:handler(line)
    let l:track = split(a:line, ' | ')
    let l:title = l:track[len(l:track)-1]
    let l:playlist = substitute(l:track[0], ' $', '', '')
    " This is never called unless we re-bind Enter in fzf
    let cmd = 'osascript -l JavaScript ' . s:jxa.Play . shellescape(l:playlist) . ' ' . shellescape(l:title)
    call system(cmd)
endfunction

" Exposed methods

function! itunes#search_and_play(args)
    if g:itunes_online
        let l:args = 'Online ' . a:args
    else
        let l:args = a:args
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

function! itunes#refresh()
    let l:jxa_path = s:files.Search2
    if g:itunes_online
        call s:refreshLibrary(l:jxa_path, 'Online')
    else
        call s:refreshLibrary(l:jxa_path, 'Offline')
    endif  
endfunction
