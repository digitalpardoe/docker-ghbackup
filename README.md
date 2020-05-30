This is an image based on [https://github.com/qvl/ghbackup](https://github.com/qvl/ghbackup) with a bit of cron sprinkled in for good measure.

It performs hourly backups of all the GitHub repositories you have access to (it'll exclude organisations it doesn't have specific permission to access), automatically downloading any new repositories and updating any existing ones.

You can generate a personal access token here [ https://github.com/settings/tokens](https://github.com/settings/tokens).

This image isn't interactive, you won't see much, if any, output when it runs.

## Usage

```
docker run \
  -v </path/to/backup/folder>:/ghbackup \
  -e GITHUB_SECRET=<GITHUB_SECRET> \
  digitalpardoe/ghbackup
```

## Parameters

* `-v /ghbackup` - folder to store the GitHub backups
* `-e GITHUB_SECRET` - either the password or personal access token (recommended) for the GitHub user
