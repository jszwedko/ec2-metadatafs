## Unreleased

Changes:
* Support for caching file and directory attributes as well as directory
  listings via `cachesec`

Bug fixes:
* Wait for daemonized process to notify when mounting before exiting. This will
  help users determine if it mounted correctly or not.
* Unmount when process is sent SIGINT or SIGTERM

## 0.3.0 (August 23, 2016)

Changes:
* `-v` can now be specified multiple times (up to 2) to additionally print FUSE
  logs

Bug fixes
* Implement a stubbed StatFS so that programs like `df` do not complain.
  However, it appears that it ignores the filesystem completely. A future
  version will attempt to return accurate statistics.

## 0.2.0 (February 13, 2016)

Add support for optionally mounting instance tags via `--tags`. See
[README.md](README.md) for more details on how to use this feature.

## 0.1.0 (January 24, 2016)

Initial release
