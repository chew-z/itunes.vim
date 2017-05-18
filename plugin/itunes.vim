scriptencoding utf-8
" Location: autoload/thrasher/itunes.vim
" Author:   Robert Jakubowski <https://github.com/chew-z>
" 
if exists('g:loaded_itunes')
    finish
endif
let g:loaded_itunes = 1

command! -nargs=* Tunes             call itunes#search_and_play(<q-args>)
command! -nargs=0 TunesOnline       call itunes#toggleOnline()
command! -nargs=0 TunesRefresh      call itunes#refreshLibrary()
