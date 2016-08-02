speechtotext
============

basic usage:
```sh
$ uname
Darwin
$ cat sample.wav | speechtotext -key (path to serviceaccount json key)
Alpha Beta gamma
$
$ ffmpeg -nostats -loglevel 0 -f avfoundation -i ":Built-in Microphone" -ar 16000 -acodec pcm_s16le -ac 1 -f wav - |tee out.wav | speechtotext -key (path to serviceaccount json key)
```
