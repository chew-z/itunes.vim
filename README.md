# itunes.vim
**Fuzzy search and play iTunes tracks from VIM. You have no idea what gems are hidden in your Music Library**

Install and try for yourself:

** I have just changed internal logic of the plugin. It works OK but I need a day to update README.**


* Search tracks in Library on your Mac ```:Tunes ```

* Search through single playlist ```:Tunes Women who know how to sing ```

* Use only part of playlist name ```:Tunes Women who ```

* Same as ```:Tunes``` - ```:Tunes Library``` 


## Installation


* You will need [fzf](https://github.com/junegunn/fzf) installed and activated in VIM. [^6] 

* Read through [fzf.vim](https://github.com/junegunn/fzf.vim) and [fzf](https://github.com/junegunn/fzf) and configure fzf options to your taste. It helps a lot.

* If you are using MacVim you need a glue connecting fzf and MacVim. But this is up to fzf installation and configuration and not this plugin.

I am using [Vim-Plug](https://github.com/junegunn/vim-plug) and don't know much about other plugin managers. In Vim-Plug just add [in right place] to your .vimrc:

``` Plug 'chew-z/itunes.vim'```

and run


```:PlugInstall```

This plugin includes two compiled Javascript scripts - JXA [(Javascript for Automation)](https://gist.github.com/JMichaelTX/d29adaa18088572ce6d4) - that work with iTunes. Trust me there is no malcious code inside.

Because you should not trust people on the internet you can review the code (in .js files) and compile for yourself.

```
osacompile -l JavaScript -o iTunes_Search_fzf.scpt iTunes_Search_fzf.js

osacompile -l JavaScript -o iTunes_Play_Tracks.scpt iTunes_Play_Tracks.js
```

## How it works?

See for yourself. There are three stages. 

1) Enter ```:Tunes``` command in VIM to search for playlist(s). 

Without any parameters Tunes searches your entire Library (or however it is called in your locale) but only retrieves tracks that are downloaded to your computer (file tracks as Apple calls them).

If you insists on getting on-line tracks every time just add to your .vimrc.

```let g:itunes_online = 1```

You can also toggle Online/Offline mode with ```TunesOnline``` command.

Mind however that grabbing also on-line tracks (in entire collection) is heavy and this is blocking plugin. Try narrowing search. Otherwise you want enjoy using this plugin. [^1] [^2]

Test [my fork of Thrasher plugin](https://github.com/chew-z/thrasher) which is grabbing tracks async and remembering Library. Results are shown in instant..

2) Fuzzy search through songs

fzf is searching through

- playlist tittle
- track tittle
- track album
- track artist

Try, this is cool, fzf is great tool.

You can toggle preview window with '?'. Or clear your search phrase with Ctrl-U (like in terminal).

If more then one playlist matches your ```:Tunes ``` search you could have doubled/multiplied results (if the track belongs to more then one playlist). I think it is cool feature as I can finally locate where my tracks got lost.

3) Press Enter to select and play track.

Plugin now plays selected track in a context of choosen playlist. Play queue is filled with playlist and we start playing from selected tracks. It clears what has been in iTunes queue before.

4) Repeat 2) and 3) as long as you wish.

5) Press Escape to exit fzf window and **do something productive in VIM**.


## Why is it cool?

Searching with fzf is cool. Also the plugin is using JXA instead of walking disaster cum enigma that AppleScript is.

## Why it isn't?

Not async - loading all tracks and playlists can take time.[^2]


## But I don't need yet another VIM plugin


Fair enough.

Just add to your .vimrc

```
let s:jxa_folder = 'WHERE DID YOU PUT JXA FILES?'

function! s:itunes_handler(line)
    let l:track = split(a:line, ' | ')
    " call append(line('$'), a:line)
    " normal! ^zz
    let l:title = l:track[len(l:track)-1]
    let l:playlist = substitute(l:track[0], ' $', '', '')
    " echom l:playlist
    " echom join(l:track, ' ')
    " This is never called unless we re-bind Enter in fzf
    let cmd = 'osascript -l JavaScript ' . s:jxa_folder . '/iTunes_Play_Playlist_Track.scpt ' . shellescape(l:playlist) . ' ' . shellescape(l:title)
    echom cmd
    let l:resp = system(cmd)
    echom l:resp
endfunction

command! -nargs=* Tunes call fzf#run({
    \ 'source':  'osascript -l JavaScript ' . s:jxa_folder . '/iTunes_Search_fzf.scpt ' .  <q-args>,
    \ 'sink':   function('<sid>itunes_handler'),
    \ 'options': '--header "Enter to play track. Esc to exit."' . 
    \ ' --preview="echo -e {} | tr ''|'' ''\n'' | gsed -e ''s/^ //g'' | tail -r " ' .
    \ ' --preview-window down:4:wrap' . 
    \ ' --bind "?:toggle-preview"' .
    \ ' --bind "enter:execute-silent(echo -n {} | gsed -e ''s/^\(.*\) | \(.*\) | \(.*\) | \(.*$\)/\"\1\" \"\4\"/'' | xargs osascript -l JavaScript ' . s:jxa_folder . '/iTunes_Play_Playlist_Track.scpt ' .  ')" '
    \ }
```


## Why use VIM just to play single track in iTunes?


You are right. Just add following function to your .zshrc or whatever shell you use [zsh tested].

```
tunes() {
# usage: tunes [Online] [Partial name of playlist] or just tunes. Enter to play, Esc ro exit.
    local jxa_dir=''WHERE DID YOU PUT JXA FILES?'
    osascript -l JavaScript $jxa_dir/iTunes_Search_fzf.scpt $@ |\
    fzf \
        --header "Enter to play. Esc to exit. ? toggles preview window." \
        --bind "enter:execute-silent(echo -n {} | sed -e 's/^\(.*\) | \(.*\) | \(.*\) | \(.*$\)/\"\1\" \"\4\"/' | xargs osascript -l JavaScript $jxa_dir/iTunes_Play_Playlist_Track.scpt )" \
        --bind '?:toggle-preview' \
        --preview "echo -e {} | tr '|' '\n' | sed -e 's/^ //g' | tail -r " \
        --preview-window down:4:wrap |\
        sed -e 's/^.*| //g' |\
# This is never used unless we re-bind Enter. Esc simply quits fzf without any action.
    xargs osascript -l JavaScript $jxa_dir/iTunes_Play_Playlist_Track.scpt
}
```

Restart your Terminal/iTerm and type ```tunes```

## Footnotes.


[^0]: How do you create proper footnotes in this weird markdown flavour?

[^1]: This is not First World problem but I am developing this plugin on an island off Sumatra and Internet could be spotty and my mobile package is limited. 

Just right now internet slowed down to [EDGE (check in Wikipedia if you are too young to know what it is)](https://en.wikipedia.org/wiki/Enhanced_Data_Rates_for_GSM_Evolution) - cause of rain and heavy wind during the night probably. 

Even pushing commits is hard. Hence the Offline option is default. 

[^4]: fzf has multiline select feature so we can create ad hoc playlists and play queues. I am thinking about it.

[^5]: This is using ```--bind=execute-silent``` a bit esotheric (and damm difficult to debug) feature of fzf

[^6]: Did I mention it works only on Mac?
