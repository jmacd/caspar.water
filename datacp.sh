#!/bin/sh

echo From water

rsync -chavzP --stats jmacd@linux.local:/home/data /Volumes/sourcecode/src/caspar.water

echo To cloud

rsync -chavzP --stats /Volumes/sourcecode/src/caspar.water/data root@casparwater.us:/home
