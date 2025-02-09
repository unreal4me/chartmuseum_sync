depends on github.com/schollz/progressbar/v3
go get -u github.com/schollz/progressbar/v3

This uses the chartmuseum http api to fetch and upload differences between chartmuseum instances.
Alltough the name is 'sync' it will only add stuff if it's missing, will not delete charts.
