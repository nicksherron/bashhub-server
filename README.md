# bashhub-server
[![Go Report Card](https://goreportcard.com/badge/github.com/nicksherron/bashhub-server)](https://goreportcard.com/report/github.com/nicksherron/bashhub-server)

bashub-server is a open-source server for  [bashhub-client](https://github.com/rcaloras/bashhub-client) with some
added features like regex search.
 
## Features 

- All the benefits of bashhub without having to send your shell history to a third-party
- Regex search  
- Very simple and easy to install and get running with existing bashhub-client
- Written in Go so it's fast and is actively maintained

## Why? 
I love the idea behing bashhub. Having my shell history centralized and queryable from various systems whether it 
be  my home computer or from an ssh session on a server is great. BUT not if that means sending my shell history to a  third-party, 
regardless of their intentions or trustworthiness, so bashhub-server was created.


## Installation

#### Homebrew or Linuxbrew
```shell script
brew install bashhub-server/latest/bashhub-server
```
#### Docker 
```shell script
docker pull nicksherron/bashhub-server
```
#### Releases 
Static binaries for various os and architectures can be found in [releases](https://github.com/nicksherron/bashhub-server/releases).
If your system is not listed just add an issue requesting your os and architecture.

## Usage 
```shell script
bashhub-server --help

Usage:
   [flags]
   [command]

Available Commands:
  help        Help about any command
  version     Print the version number and build info

Flags:
  -a, --addr string   Ip and port to listen and serve on. (default "0.0.0.0:8080")
      --db string     DB location (sqlite or postgres)
  -h, --help          help for this command
      --log string    Set filepath for HTTP log. "" logs to stderr.

Use " [command] --help" for more information about a command.

```
### Running
Just run the server 

```shell script
bashhub-server
```
or on docker 

```shell script
docker run -d -p 8080:8080 --name bashhub-server  nicksherron/bashhub-server 

```
Then add ```export BH_HOST=localhost:8080``` (or whatever you set your bashhub-server address to) to your .zshrc or .bashrc 

Thats it! Restart your shell with `$SHELL` then re-configure bashhub-client to use your new
server by re-running ```bashhub setup``` .

### Setting DB

### Using Regex
