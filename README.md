# itunes.vim
Fuzzy search and play iTunes tracks from VIM

Install and try:

```
:Tunes
:Tunes Women who know how to sing
:Tunes Women who
:Tunes Offline Women who know
:Tunes Music
:Tunes Library
``` 


## Installation


You will need [fzf](https://github.com/junegunn/fzf) installed and activated in VIM. Read [fzf.vim](https://github.com/junegunn/fzf.vim)

Using [Vim-Plug](https://github.com/junegunn/vim-plug) add to your .vimrc:


``` chew-z/itunes.vim ``` and run  ```PlugInstall```

The plugin includes two compiled Javascript scripts - JXA (Javascript for Automation) - that work with iTunes. Tust me there is no malcious code inside.

But because yoy genrally should not trust people on the internet you can review the code (in .js files) and compile for yourself.

```
osacompile -l JavaScript -o iTunes_Search_fzf.scpt iTunes_Search_fzf.js

osacompile -l JavaScript -o iTunes_Play_Tracks.scpt iTunes_Play_Tracks.js
```

## How it works?

See for yourself. There are three stages. 

1) Enter Tunes command in VIM to search for playlist(s). 

Without any parameters Tunes searches entire Library (or however it is called in your locale) and only songs that are downloaded to your computer. If you add a phrase Tunes searches for playlist that contains your phrase (not necessary entire playlist title).

2) Fuzzy search results (with fzf) looking for tracks

3) Press Enter to select and play track.

4) Repeat 2) and 3) as long as you wish

5) Press Escape to exit fzf and do something productive in VIM.

If you add Online in front of search phrase the plugin will search also for subscribed tracks (in Apple parlance) - tracks that are in your playlists but haven't been downloaded to your Mac. 




