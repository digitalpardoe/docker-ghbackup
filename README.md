# docker-ghbackup
[dockerstore]: https://hub.docker.com/r/digitalpardoe/ghbackup/

Docker version of https://github.com/qvl/ghbackup

#### Usage
```
docker run -v /backup/folder:/ghbackup -e GITHUB_USER=username -e GITHUB_SECRET=password_or_token digitalpardoe/ghbackup
```
