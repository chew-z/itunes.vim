scriptencoding utf-8
" Location: autoload/itunes/itunes.vim
" Author:   Robert Jakubowski <https://github.com/chew-z>
" 
if exists('g:itunes_autoloaded') || v:version < 800
    if v:version < 800
        echoerr 'itunes.vim: itunes#refreshLibrary() is using async and requires VIM version 8 or higher'
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


" Local functions
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

function! s:bufferFromCache()
	let l:tracks = s:cache
	let currentBuf = bufnr('%')
	let g:iTunesBufNum = bufnr('itunes_cache', 1)
	let l:tracks = a:cache
    if !empty(l:tracks)
        let l:i = 1
        for l:t in l:tracks
            let l:line = join([l:t.collection, l:t.artist, l:t.album, l:t.name], ' | ')
            let l:res = append(l:i, l:line)
            let l:i += 1
        endfor
    endif
    " exec g:iTunesBufNum . 'bufdo %d'
    " exec 'b ' . g:iTunesBufNum
endfunction 

function! s:refreshTracks(query)
" refresh s:tracks
" if empty(a:query) | return s:cache | endif
    let s:tracks = []
	let l:tracks = s:cache
    if !empty(l:tracks)
        let l:i = 1
        if empty(a:query)
            for l:t in l:tracks
                " transform to format expected by fzf
                let l:line = join([l:t.collection, l:t.artist, l:t.album, l:t.name], ' | ')
                let s:tracks= add(s:tracks, l:line)
                let l:i += 1
            endfor
        else
            for l:t in l:tracks
				if l:t.collection =~ a:query
					" transform to format expected by fzf
					let l:line = join([l:t.collection, l:t.artist, l:t.album, l:t.name], ' | ')
					let s:tracks= add(s:tracks, l:line)
				endif
				let l:i += 1
			endfor
			if g:itunes_verbose | echom 'Found' len(s:tracks) 'tracks matching' a:query | endif
        endif
    endif
endfunction 

" Async helpers

function! RefreshLibrary_JobEnd(channel)
    let s:cache = s:restoreVariable(g:itunes_refreshLibrary)
    if !filewritable(s:files.Cache)
        Silent touch s:files.Cache
    endif
    call s:saveVariable(s:cache, s:files.Cache)
    " TODO call itunes#refreshList()
    if filereadable(s:files.Cache) | echom 'iTunes Library refreshed' | endif
    unlet g:itunes_refreshLibrary
	if g:itunes_verbose | echom 'RefreshLibrary job finished with' len(s:cache) 'items' | endif 
	let s:online = g:itunes_online
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

" Local varaiables

let s:folder = expand('<sfile>:p:h')
let s:files = {
\ 'Play':       s:folder .  '/iTunes_Play_Playlist_Track.scpt',
\ 'Search':     s:folder .  '/iTunes_Search_fzf.scpt',
\ 'Search2':    s:folder .  '/iTunes_Search2_fzf.scpt',
\ 'Cache':      s:folder .  '/iTunes_Library_Cache.txt'
\ }
if g:itunes_verbose
    echom s:folder
    echom s:files.Play
    echom s:files.Search
    echom s:files.Cache
endif

" s:cache stores copy of iTunes_Library_Cache.txt
if filereadable(s:files.Cache)
	let s:cache = s:restoreVariable(s:files.Cache)
else
	let s:cache = []
endif
" s:tracks stores transformed s:cache for feeding fzf
" TODO - it could also filtered with initial query
let s:tracks = []
" refreshed when async job finished
let s:online = g:itunes_online

" FZF sink function

function! s:handler(line)
    let l:track = split(a:line, ' | ')
    let l:title = l:track[len(l:track)-1]
    let l:playlist = substitute(l:track[0], ' $', '', '')
    " This is never called unless we re-bind Enter in fzf
    let cmd = 'osascript -l JavaScript ' . s:files.Play . shellescape(l:playlist) . ' ' . shellescape(l:title)
    call system(cmd)
endfunction

" Exposed global methods

function! itunes#search_and_play(args)
    " restore Music Library form disk file
    if filereadable(s:files.Cache) | let s:cache = s:restoreVariable(s:files.Cache) | endif 
    if empty(s:cache)
        if exists('g:itunes_refreshLibrary')
            echom 'iTunes Library Cache is refreshing'
            return 1
        else
            echom 'Let me get iTunes Library first '
            call itunes#refreshLibrary()
            return 1
        endif
    endif
	let l:online = 'Offline'
	if s:online | let l:online = 'Online' | endif
    call itunes#transform(a:args)
"   TODO - live refresh ? 
"   TODO - search mode <all><track name><playlist name><track album> ?
    call fzf#run({
    \ 'source': s:tracks,
    \ 'sink':   function('s:handler'),
    \ 'options': '--header "Enter to play track Esc to exit ? toggles preview ['  . l:online . ']"'
        \. ' --bind "enter:execute-silent(echo -n {} | gsed -e ''s/^\(.*\) | \(.*\) | \(.*\) | \(.*$\)/\"\1\" \"\4\"/'' | xargs osascript -l JavaScript ' .  s:files.Play . ')" ' .
        \ ' --preview="echo -e {} | tr ''|'' ''\n'' | sed -e ''s/^ //g'' | tail -r " ' .
        \ ' --preview-window down:4:wrap' .
        \ ' --bind "?:toggle-preview"'
    \ })
endfunction

function! itunes#refreshLibrary()
    let l:jxa_path = s:files.Search2
    if g:itunes_online
        call s:refreshLibrary(l:jxa_path, 'Online')
    else
        call s:refreshLibrary(l:jxa_path, 'Offline')
    endif  
endfunction

function! itunes#transform(query)
" from JSON to VIM list with lines and '|' separators
    if empty(s:cache)
        if filereadable(s:files.Cache)
            let s:cache = s:restoreVariable(s:files.Cache)
        else
            call itunes#refreshLibrary()
            return 1
        endif
    endif
    call s:refreshTracks(a:query)
    " call itunes#search_and_play('Dummy')
endfunction

function! itunes#toggleOnline()
" Toggle On-line and refresh List Cache
	if g:itunes_verbose | echom 'Online' g:itunes_online | endif
	if g:itunes_online
        let g:itunes_online = 0
    else
        let g:itunes_online = 1
    endif
    call itunes#refreshLibrary()
	if g:itunes_verbose | echom 'Online' g:itunes_online | endif
endfunction

function! itunes#list()
	call s:bufferFromCache()
endfunction

