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

#### Linux (64 bit)

```bash
curl -sL https://github.com/jszwedko/ec2-metadatafs/releases/download/0.0.1/linux_amd64 > ec2-metadatafs
sudo mv ec2-metadatafs /usr/bin/
sudo chmod +x /usr/bin/ec2-metadatafs
```

#### Linux (32 bit)

```bash
curl -sL https://github.com/jszwedko/ec2-metadatafs/releases/download/0.0.1/linux_386 > ec2-metadatafs
sudo mv ec2-metadatafs /usr/bin/
sudo chmod +x /usr/bin/ec2-metadatafs
```

You can have it automatically mount by adding the following to `/etc/fstab`:

`ec2-metadatafs#default    /aws    fuse    _netdev,allow_other    0    0`

Alternatively, install the latest via: `GOVENDOREXPERIMENT=1 go get
github.com/jszwedko/ec2-metadatafs` (requires Go >= 1.5 to be installed).

## Usage

`ec2-metadatafs -f <mount point>` will mount the filesystem at the designated mount point.

Example:
```
$ sudo mkdir /aws
$ ec2-metadatafs /aws &
$ ls -1 /aws/meta-data/
ami-id
ami-launch-index
ami-manifest-path
block-device-mapping
hostname
instance-action
instance-id
instance-type
local-hostname
local-ipv4
mac
metrics
network
placement
profile
public-hostname
public-ipv4
public-keys
reservation-id
security-groups
services
$ cat /aws/meta-data/instance-id
i-123456
```

See `ec2-metadatafs -h` for more configuration options.

### Developing

Requires Go 1.5 and
[`GOVENDOREXPERIMENT=1`](https://docs.google.com/document/d/1Bz5-UB7g2uPBdOx-rw5t9MxJwkfpx90cqG9AFL0JAYo/edit)
to properly include dependencies.

Uses [`gvt`](https://github.com/FiloSottile/gvt) to manipulate dependencies.

- Building: `make build`
- Testing: `make test`
- Building cross compiled binaries: `make dist` (will install
  [gox](https://github.com/mitchellh/gox) if needed)
