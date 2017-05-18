scriptencoding utf-8
" Location: autoload/thrasher/itunes.vim
" Author:   Robert Jakubowski <https://github.com/chew-z>
" 
if exists('g:loaded_itunes')
    finish
endif
let g:loaded_itunes = 1

" Search and play - just :Tunes shows all playlists, one could pass partial playlist 
" name as an argument (:Tunes Jazz or :Tunes Pop hits) to narrows down
" initial list.
command! -nargs=* Tunes             call itunes#search_and_play(<q-args>)
" Toggle Online mode (include all tracks) and Offline mode (only fileTracks -
" songs downloaded to Mac). Snd refresh Library.
command! -nargs=0 TunesOnline       call itunes#toggleOnline()
" Refresh Library cache in case new playlists or tracks had been added to ITunes 
" (toggleOnline refreshes Library automatically)
command! -nargs=0 TunesRefresh      call itunes#refreshLibrary()
" Without arguments fills scratch buffer with all playlists and their tracks. 
" Arguments allow to narrow down playlists (like in  :Tunes)
command! -nargs=* TunesList         call itunes#list(<q-args>)
