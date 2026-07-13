" itunes.vim - Apple Music controller for Vim
" Wraps the `itunes` Go CLI with fzf-powered search and playback.
" Requires: fzf (https://github.com/junegunn/fzf)

if exists('g:loaded_itunes')
  finish
endif
let g:loaded_itunes = 1

command! -nargs=? Tunes       call itunes#search_and_play(<q-args>)
command! -nargs=0 TunesStatus call itunes#status()
command! -nargs=? TunesPlay   call itunes#play_playlist(<q-args>)
