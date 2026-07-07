.PHONY: all windows linux clean

all: windows linux

windows: vilatile.exe

vilatile.exe: $(wildcard *.go) go.mod go.sum init.vim
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H windowsgui" -o vilatile.exe .

linux: vilatile

vilatile: $(wildcard *.go) go.mod go.sum init.vim
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o vilatile .

clean:
	rm -f vilatile vilatile.exe
