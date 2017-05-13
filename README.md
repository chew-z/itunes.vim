# itunes.vim
Fuzzy search and play iTunes tracks from VIM

Install and try for yourself:


* Search tracks in your Library on your Mac ```:Tunes ```

* Search for a playlist ```:Tunes Women who know how to sing ```

* Search using only part of playlist name ```:Tunes Women who ```

* Include tracks that are not downloaded to your Mac (Apple Music) ```:Tunes Online Women who know ```

* ```Tunes ```

* Every track in your collection ```Tunes Online```


## Installation


You will need [fzf](https://github.com/junegunn/fzf) installed and activated in VIM. Read [fzf.vim](https://github.com/junegunn/fzf.vim). If you are using MacVim you need a glue connecting fzf and MacVim. But this is all up to fzf installation and configuration.

I am using [Vim-Plug](https://github.com/junegunn/vim-plug) and don't know much about other plugin managers. In Vim-Plug just add to your .vimrc:

``` chew-z/itunes.vim ``` 

and run


```PlugInstall```

This plugin includes two compiled Javascript scripts - [JXA (Javascript for Automation)](https://gist.github.com/JMichaelTX/d29adaa18088572ce6d4) - that work with iTunes. Tust me there is no malcious code inside.

But because you should not trust people on the internet you can review the code (in .js files) and compile for yourself.

```
osacompile -l JavaScript -o iTunes_Search_fzf.scpt iTunes_Search_fzf.js

osacompile -l JavaScript -o iTunes_Play_Tracks.scpt iTunes_Play_Tracks.js
```

## How it works?

See for yourself. There are three stages. 

1) Enter ```Tunes``` command in VIM to search for playlist(s). 

Without any parameters Tunes searches your entire Library or however it is called in your locale but only tracks that are downloaded to your computer (file tracks as Apple calls them).

If you add a phrase ```Tunes``` plugin searches for playlists that contain that phrase (it doesn't need to be entire playlist title). 
If you add Online right after Tunes command (it can be followed by search phrase) also online tracks will be included in results. Mind however that in my modest music collection there is currently 700 local tracks and 15500 altogether. Grabbing online tracks takes longer.[^1] [^2]

2) Fuzzy search through song (with fzf) looking for tracks

fzf is searching through playlist tittle, track tittle, track album and track artists. This is cool. fzf is great tool.
You can toggle preview window with '?'. Or clear your search phrase with Ctrl-U (like in terminal).

If more then one playlist matched your ```Tunes ``` search you can have multiple results (the track is part of more then one playlist).

3) Press Enter to select and play track.

This script only plays one selected track and then falls back to whatever is in iTunes play queue. You can of course select and play another track.[^2]

4) Repeat 2) and 3) as long as you wish.

5) Press Escape to exit fzf and **do something productive in VIM**.

If you add Online in front of search phrase the plugin will search also for subscribed tracks (in Apple parlance) - tracks that are in your playlists but haven't been downloaded to your Mac. 

## Why is it cool?

Searching with fzf is cool. Also the plugin is using JXA instead of walking disaster cum enigma that AppleScript is.

## Why it isn't?

Not async - loading all tracks and playlist can take time.[^2]

It plays only single track.[^2]

Next release should fix this.[^3]

## But I yet don't need another plugin

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

## Footnotes.

[^1]: This is not First World problem but I am developing this plugin on an island off Sumatra and Internet could be spotty and my mobile package is limited. Hence the Offline option is default.

[^2]: If you prefer non-blocking plugin that is playing entire playlists and working asynchronously try [my fork of Thrasher plugin](https://github.com/chew-z/thrasher).

[^3]: I am sorry. English spellchecking is broken in my VIM. I have to fix this first.

