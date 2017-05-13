# itunes.vim
**Fuzzy search and play iTunes tracks from VIM**

Install and try for yourself:


* Search tracks in your Library on your Mac ```:Tunes ```

* Search for a playlist ```:Tunes Women who know how to sing ```

* Search using only part of playlist name ```:Tunes Women who ```

* Include tracks that are not downloaded to your Mac (Apple Music) ```:Tunes Online Women who know ```

* Same as ```:Tunes``` - ```:Tunes Library``` or ```:Tunes Offline Library```

* Everything in your collection ```:Tunes Online```


## Installation


* You will need[^6] [fzf](https://github.com/junegunn/fzf) installed and activated in VIM. 

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

If you add a phrase after ```Tunes``` this plugin searches for playlists that contain that phrase (it doesn't need to be whole playlist title)

If you preced search phrase with word ```Online``` plugin will search also for subscribed tracks (in Apple parlance) - tracks that are in your playlists but haven't been downloaded to your Mac. 

Mind however that in my modest music collection there is currently 700 odd local tracks and 15 500 tracks altogether. Grabbing on-line tracks is heavy and this is blocking plugin. [^1] [^2]

If you still insists on getting on-line tracks every time just add to your .vimrc.

```let g:itunes_online = 1``` 

Or test [my fork of Thrasher plugin](https://github.com/chew-z/thrasher) which is grabbing tracks async.

2) Fuzzy search through songs

fzf is searching through

- playlist tittle
- track tittle
- track album
- track artist

Try, this is cool, fzf is great tool.

You can toggle preview window with '?'. Or clear your search phrase with Ctrl-U (like in terminal).

If more then one playlist matched your ```Tunes ``` search you can have multiple results (the track is part of more then one playlist).

3) Press Enter to select and play track.

This script only plays[^5] one selected track and then falls back to whatever is in your iTunes play queue. You can of course select and play another track. [^2] [^4]

4) Repeat 2) and 3) as long as you wish.

5) Press Escape to exit fzf and **do something productive in VIM**.


## Why is it cool?

Searching with fzf is cool. Also the plugin is using JXA instead of walking disaster cum enigma that AppleScript is.

## Why it isn't?

Not async - loading all tracks and playlists can take time.[^2]

It plays only single track. [^2] [^4] Next release should fix this. [^3]


## But I don't need yet another VIM plugin


Fair enough.

Just add to your .vimrc

```
let s:jxa_folder = 'WHERE DID YOU PUT JXA FILES?'

function! s:itunes_handler(line)
    let l:track = split(a:line, ' | ')
    " This is never called unless we re-bind Enter in fzf
    call system('osascript -l JavaScript ' . s:jxa_folder . '/iTunes_Play_Track.scpt ' . l:title)
endfunction

command! -nargs=* Itunes call fzf#run({
    \ 'source':  'osascript -l JavaScript ' . s:jxa_folder . '/iTunes_Search_fzf.scpt ' .  <q-args>,
    \ 'sink':   function('<sid>itunes_handler'),
    \ 'options': '--header "Enter to play track. Esc to exit."' . ' --bind "enter:execute-silent(echo -n {} | gsed -e ''s/^.*| //g''  | xargs osascript -l JavaScript ' . s:jxa_folder . '/iTunes_Play_Track.scpt ' . ')" ' . ' --preview="echo -e {} | tr ''|'' ''\n'' | sed -e ''s/^ //g'' | tail -r " ' . ' --preview-window down:4:wrap' . ' --bind "?:toggle-preview"'
    \ })
```


## Why use VIM just to play single track in iTunes?


You are right. Just add following alias to your .zshrc or whatever shell you use [zsh tested].

```
tunes() {
# usage: tunes [Online] [Partial name of playlist] or just tunes. Enter to play, Esc ro exit.
    local jxa_dir=''WHERE DID YOU PUT JXA FILES?'
    osascript -l JavaScript $jxa_dir/iTunes_Search_fzf.scpt $@ |\
    fzf \
        --header "Enter to play. Esc to exit. ? toggles preview window." \
        --bind "enter:execute-silent(echo -n {} | gsed -e 's/^.*| //g'  | xargs osascript -l JavaScript $jxa_dir/iTunes_Play_Track.scpt )" \
        --bind '?:toggle-preview' \
        --preview "echo -e {} | tr '|' '\n' | sed -e 's/^ //g' | tail -r " \
        --preview-window down:4:wrap |\
        sed -e 's/^.*| //g' |\
# This is never used unless we re-bind Enter. Esc simply quits fzf without any action.
    xargs osascript -l JavaScript $jxa_dir/play.scpt
}
```

Restart your Terminal/iTerm and type ```tunes```

## Footnotes.


[^0]: How do you create proper footnotes in this weird markdown flavour?

[^1]: This is not First World problem but I am developing this plugin on an island off Sumatra and Internet could be spotty and my mobile package is limited. 

Just right now internet slowed down to [EDGE (check in Wikipedia if you are too young to know what it is)](https://en.wikipedia.org/wiki/Enhanced_Data_Rates_for_GSM_Evolution) - cause of rain and heavy wind during the night probably. 

Even pushing commits is hard. Hence the Offline option is default. 

[^2]: If you prefer non-blocking plugin that is playing entire playlists and working asynchronously try [my fork of Thrasher plugin](https://github.com/chew-z/thrasher).

[^3]: I am sorry. English spellchecking is broken for markdown in my VIM. I have to fix this first.

[^4]: fzf has multiline select feature so we can create ad hoc playlists and play queues. I am thinking about it.

[^5]: This is using ```--bind=execute-silent``` a bit esotheric (and damm difficult to debug) feature of fzf

[^6]: Did I mention it works only on Mac?
