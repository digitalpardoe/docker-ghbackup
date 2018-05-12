# docker-ghbackup

Docker version of https://github.com/qvl/ghbackup

## Usage
```
docker create \
  -v </path/to/backup/folder>:/ghbackup \
  -e GITHUB_USER=<GITHUB_USER> \
  -e GITHUB_SECRET=<GITHUB_SECRET> \
  digitalpardoe/ghbackup
```
## Parameters

* `-v /ghbackup` - folder to store the GitHub backups
* `-e GITHUB_USER` - username for GitHub user to backup
* `-e GITHUB_SECRET` - either the password or personal access token (recommended) for the GitHub user
