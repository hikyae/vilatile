" vilatile - temporary GVim window initialization
set bufhidden=wipe
set noswapfile
set nobuflisted
set guioptions-=m
set guioptions-=T
set guioptions-=r
set guioptions-=R
set guioptions-=l
set guioptions-=L
set guioptions-=b
set guioptions-=e
set laststatus=0
set noruler
set showtabline=0
" Suppress the file info message when loading the stdin temp file; together
" with the buffer rename below it would trigger a hit-enter prompt.
set shortmess+=F
set title
let &titlestring = "vilatile"

" Name the buffer uniquely per process to avoid multi-instance conflicts
silent execute 'file vilatile_' . getpid()

augroup vilatile
  autocmd!
  autocmd BufWriteCmd vilatile_* call s:CopyToClipboard()
  autocmd QuitPre vilatile_* set nomodified
  " Start in insert mode only when the buffer is empty: text piped to stdin
  " is there to be reviewed and edited, so begin in normal mode instead.
  autocmd VimEnter * if line('$') == 1 && empty(getline(1)) | startinsert | endif
  autocmd VimEnter * call timer_start(20, function('s:SetSize'), {'repeat': 10})
augroup END

function! s:CopyToClipboard()
  let l:text = join(getline(1, '$'), "\n")
  if !empty($WAYLAND_DISPLAY) && executable('wl-copy')
    " Setting @+ only holds the selection while this process lives; wl-copy
    " forks a daemon that keeps serving it after ZZ kills gvim. system() is
    " synchronous, so the clipboard is guaranteed set when it returns.
    call system('wl-copy', l:text)
  else
    let @+ = l:text
  endif
  set nomodified
  echom "Copied to clipboard"
endfunction

" Set the size after the window manager has floated and resized the window;
" doing it earlier races with the WM's resize and can hang GVim.
function! s:SetSize(timer)
  let l:columns = get(g:, 'vilatile_columns', 0)
  let l:lines = get(g:, 'vilatile_lines', 0)
  if l:columns > 0
    let &columns = l:columns
    let &lines = l:lines
  endif
endfunction
