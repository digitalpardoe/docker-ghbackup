# docker-ghbackup

Docker version of https://github.com/qvl/ghbackup

#### Usage
```
docker create \
  -v </path/to/backup/folder>:/ghbackup \
  -e GITHUB_USER=username \
  -e GITHUB_SECRET=password_or_token \
  digitalpardoe/ghbackup
```
