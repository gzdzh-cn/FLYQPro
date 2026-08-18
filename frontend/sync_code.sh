#!/bin/bash
rsync -avh --delete \
    --exclude-from='rsync-exclude.txt' \
    --filter=':- .gitignore' \
    /Volumes/disk/site/go/dzhgo/dzhgo-admin-vue/ /Volumes/disk/site/go/dzh3136/dzh3136-admin-vue/
