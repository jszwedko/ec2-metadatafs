## ec2-metadatafs

[FUSE](https://github.com/libfuse/libfuse) filesystem that exposes [EC2
metadata
service](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html)
in the form of a readonly filesystem. This allows simple interaction
interrogation of metadata using traditional unix utilities like `ls`, `grep`,
and `cat`.

Example:
```
$ mkdir /tmp/aws
$ ec2-metadatafs default /tmp/aws
$ cat /tmp/aws/meta-data/instance-id
i-123456
$ fusermount -u /tmp/aws
```

### Advantages over `curl http://169.254.169.254`

* No need to remember the special IP address of the service
* Can use traditional unix tools to walk and interrogate the tree

### Advantages over the [`ec2-metadata`](http://aws.amazon.com/code/1825) tool

* No need to `cut` the output of commands to get just the field
* Access to all metadata fields, not just the limited subset the tool returns

Feedback and feature requests are welcome!

## Installing

Install the latest via: `GOVENDOREXPERIMENT=1 go get
github.com/jszwedko/ec2-metadatafs` (requires Go >= 1.5 to be installed).

You can have it automatically mount by adding the following to `/etc/fstab`:

`ec2-metadatafs#default    /aws    fuse    _netdev,allow_other    0    0`

Prebuilt packages will be provided shortly.

## Usage

```
Usage:
  ec2-metadatafs [OPTIONS] endpoint mountpoint

ec2metadafs mounts a FUSE filesystem at the given location which exposes the
EC2 instance metadata of the host as files and directories mimicking the URL
structure of the metadata service.

Application Options:
  -f, --foreground  Run in foreground
  -v, --version     Display version info
  -o, --options=    These options will be passed through to FUSE. Please see the OPTIONS section of the FUSE manual for valid options

Help Options:
  -h, --help        Show this help message

Arguments:
  endpoint:         Endpoint of the EC2 metadata service, set to 'default' to use http://169.254.169.254/latest/
  mountpoint:       Directory to mount the filesystem
```

### Developing

Requires Go 1.5 and
[`GOVENDOREXPERIMENT=1`](https://docs.google.com/document/d/1Bz5-UB7g2uPBdOx-rw5t9MxJwkfpx90cqG9AFL0JAYo/edit)
to properly include dependencies.

Uses [`gvt`](https://github.com/FiloSottile/gvt) to manipulate dependencies.

- Building: `make build`
- Testing: `make test`
- Building cross compiled binaries: `make dist` (will install
  [gox](https://github.com/mitchellh/gox) if needed)
