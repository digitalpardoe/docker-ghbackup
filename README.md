# docker-ghbackup
[dockerstore]: https://hub.docker.com/r/schnorchersepp/docker-ghbackup/

[![Docker Build Status](https://img.shields.io/docker/build/schnorchersepp/docker-ghbackup.svg)][dockerstore]
[![Docker Pulls](https://img.shields.io/docker/pulls/schnorchersepp/docker-ghbackup.svg)][dockerstore]


Docker version of https://github.com/qvl/ghbackup

#### Usage
```
docker run -v /backup/folder/on/host:/ghbackup schnorchersepp/docker-ghbackup github-user-name
```
