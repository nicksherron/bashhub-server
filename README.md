# bashhub-server
[![Go Report Card](https://goreportcard.com/badge/github.com/nicksherron/bashhub-server)](https://goreportcard.com/report/github.com/nicksherron/bashhub-server) [![Build Status](https://travis-ci.org/nicksherron/bashhub-server.svg?branch=master)](https://travis-ci.org/nicksherron/bashhub-server) [![Join the chat at https://gitter.im/bashhub_server/community](https://badges.gitter.im/bashhub_server/community.svg)](https://gitter.im/bashhub_server/community?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

bashhub-server is a private cloud alternative for  [bashhub-client](https://github.com/rcaloras/bashhub-client) with some
added features like regex search.
 
## Features 

- Very simple drop in replacement for bashhub.com server and easy to [install](https://github.com/nicksherron/bashhub-server#installation) and get running with existing bashhub-client and bh command.
- All the benefits of bashhub without having to send your shell history to a third-party
- [Regex](https://github.com/nicksherron/bashhub-server#using-regex) search
- [Import](https://github.com/nicksherron/bashhub-server#transferring-history-from-bashhubcom) old history from bashhub.com
- Quickly connect any client with  access to your server bashhub-sever address/port.
- Written in Go so it's fast and is actively maintained
- Salt hashed password encryption and jwt authentication protected endpoints

## Why? 
I love the idea behind bashhub. Having my shell history centralized and queryable from various systems whether it 
be my home computer or from an ssh session on a server is great. However, even with encryption, 
I was a little leary of sending my shell commands to a third-party server, so bashhub-server was created.


## Installation

#### Homebrew or Linuxbrew
```
$ brew install bashhub-server/latest/bashhub-server
```
#### Docker 
```
$ docker pull nicksherron/bashhub-server
```
#### Go
go modules are required 
```
$ GO111MODULE=on go get -u github.com/nicksherron/bashhub-server
```
#### Releases 
Binaries for various os and architectures can be found in [releases](https://github.com/nicksherron/bashhub-server/releases).
If your system is not listed just submit an issue requesting your os and architecture.

## Usage 
```
$ bashhub-server --help

Usage:
   [flags]
   [command]

Available Commands:
  help        Help about any command
  transfer    Transfer bashhub history from one server to another
  version     Print the version number and build info

Flags:
  -a, --addr string   Ip and port to listen and serve on. (default "http://0.0.0.0:8080")
      --db string     db location (sqlite or postgres)
  -h, --help          help for this command
      --log string    Set filepath for HTTP log. "" logs to stderr.

Use " [command] --help" for more information about a command.

```
### Running
Just run the server 

```
$ bashhub-server

 _               _     _           _
| |             | |   | |         | |		version: v0.2.1
| |__   __ _ ___| |__ | |__  _   _| |		address: http://0.0.0.0:8080
| '_ \ / _' / __| '_ \| '_ \| | | | '_ \
| |_) | (_| \__ \ | | | | | | |_| | |_) |
|_.__/ \__,_|___/_| |_|_| |_|\__,_|_.__/
 ___  ___ _ ____   _____ _ __
/ __|/ _ \ '__\ \ / / _ \ '__|
\__ \  __/ |   \ V /  __/ |
|___/\___|_|    \_/ \___|_|


2020/02/10 03:04:11 Listening and serving HTTP on http://0.0.0.0:8080
```
or on docker 

```
$ docker run -d -p 8080:8080 --name bashhub-server  nicksherron/bashhub-server 
```
Then add ```export BH_URL=http://localhost:8080``` (or whatever you set your bashhub-server address to) to your .zshrc or .bashrc 
```
$ echo "export BH_URL=http://localhost:8080" >> ~/.bashrc
```
or 
```
$ echo "export BH_URL=http://localhost:8080" >> ~/.zshrc
```
Thats it! Restart your shell and re-run bashhub setup.
```
$ $SHELL && bashhub setup
```

### Changing default db
By default the backend db uses sqlite, with the location for each os shown below.


| os      | default                                                                          |
|---------|----------------------------------------------------------------------------------|
| Unix    | $XDG_CONFIG_HOME/bashhub-server/data.db OR  $HOME/.config/bashhub-server/data.db |
| Darwin  | $HOME/Library/Application Support/bashhub-server/data.db                         |
| Windows | %AppData%\bashhub-server\data.db                                                 |
| Plan 9  | $home/lib/bashhub-server/data.db                                                 |


To set a different sqlite db file to use, run
```
$ bashhub-server --db path/to/file.db
```
Postgresql is also supported by bashhub-server. To use postgres specify the postgres uri in the --db flag with the
following format
```
$ bashhub-server --db "postgres://user:password@localhost:5432?sslmode=disable"
```

### Using Regex
bashhub-server supports regex queries sent by the bh command (bashhub-client)

Without regex
```
$ bh bash

bashhub setup
docker pull nicksherron/bashhub-server
bin/bashhub-server version
untar bashhub-server_v0.1.0_darwin_amd64.tar.gz
cd bashhub-server_v0.1.0_darwin_amd64
./bashhub-server version
make build && bin/bashhub-server
cd bashhub-server
brew install bashhub-server/latest/bashhub-server
bashhub-server version
bashhub-server --help
```
With regex
```
$ bh "^bash"

bashhub setup
bashhub-server version
bashhub-server --help
```
all commands with only 6 letters

```
$ bh "^[a-zA-Z]{6}$"

whoami
ggpush
goland
ggpull
```

### Transferring history from bashhub.com

You can transfer your command history from one server to another with then ```bashhub-server transfer``` 
command.

```
$ bashhub-server transfer \
    --src-user 'user' \
    --src-pass 'password' \
    --dst-user 'user' \
    --dst-pass 'password' 

transferring 872 / 8909 [-->____________________] 9.79% 45 inserts/sec
```

 If you're transferring from Bashhub.com they have a rate limit of 10 requests a seconds and you are limited to your last 10,000 commands.







 
