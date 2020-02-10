# bashhub-server

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
