## Unreleased

Changes:
* Support for the more secure version 2 of the metadata service API

## 1.0.0 (May 11, 2019)

Primarily cut to start building OS packages, but this has been stable enough that a 1.0 release was cut.

Changes:
* Version number was simplified to remove the revision

## 0.4.0 (October 13, 2018)

Changes:
* Support for caching file and directory attributes as well as directory
  listings via `cachesec`
* Support for sending logs to syslog

Bug fixes:
* Wait for daemonized process to notify when mounting before exiting. This will
  help users determine if it mounted correctly or not
* Unmount when process is sent SIGINT or SIGTERM
* Don't return user-data in directory listing of / if it isn't set on the instance

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
