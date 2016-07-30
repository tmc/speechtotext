speechtotext
============

basic usage:
```sh
$ uname
Darwin
$ ffmpeg -f avfoundation -i ":Built-in Microphone" -c:a flac -sample_fmt s16 -ar 16000 -f flac - > sample.flac
(speak)
<q>
$ cat sample.flac | speechtotext -key (path to serviceaccount json key) -bufSize 1024
```
