scriptencoding utf-8
" Location: autoload/thrasher/itunes.vim
" Author:   Robert Jakubowski <https://github.com/chew-z>
" 
if exists('g:loaded_itunes')
    finish
endif
let g:loaded_itunes = 1

command! -nargs=* Tunes call fzf#run({
    \ 'source':  'osascript -l JavaScript ' . g:itunes_jxa.Search .  <q-args>,
    \ 'sink':   function('itunes#handler'),
    \ 'options': '--header "Enter to play track. Esc to exit."' . ' --bind "enter:execute-silent(echo -n {} | gsed -e ''s/^.*| //g''  | xargs osascript -l JavaScript ' .  g:itunes_jxa.Play . ')" ' . ' --preview="echo -e {} | tr ''|'' ''\n'' | sed -e ''s/^ //g'' | tail -r " ' . ' --preview-window down:4:wrap' . ' --bind "?:toggle-preview"'
    \ })
