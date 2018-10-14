## ec2-metadatafs [![Build Status](https://travis-ci.org/jszwedko/ec2-metadatafs.svg?branch=master)](https://travis-ci.org/jszwedko/ec2-metadatafs) [![Go Report Card](https://goreportcard.com/badge/github.com/jszwedko/ec2-metadatafs)](https://goreportcard.com/report/github.com/jszwedko/ec2-metadatafs)

[FUSE](https://github.com/libfuse/libfuse) filesystem that exposes the [EC2
metadata
service](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html)
and, optionally, the tags on the instance in the form of a readonly filesystem.
This allows simple interaction interrogation of metadata using traditional unix
utilities like `ls`, `grep`, and `cat`.

Example:
```
$ mkdir /tmp/aws
$ ec2-metadatafs --tags /tmp/aws
$ tree /tmp/aws
/tmp/aws
├── dynamic
│   └── instance-identity
│       ├── document
│       ├── dsa2048
│       ├── pkcs7
│       ├── rsa2048
│       └── signature
├── meta-data
│   ├── ami-id
│   ├── ami-launch-index
│   ├── ami-manifest-path
│   ├── block-device-mapping
│   │   ├── ami
│   │   └── root
│   ├── hostname
│   ├── iam
│   │   ├── info
│   │   └── security-credentials
│   │       └── test
│   ├── instance-action
│   ├── instance-id
│   ├── instance-type
│   ├── local-hostname
│   ├── local-ipv4
│   ├── mac
│   ├── metrics
│   ├── network
│   │   └── interfaces
│   │       └── macs
│   │           └── 06:5e:69:f7:53:ed
│   │               ├── device-number
│   │               ├── interface-id
│   │               ├── local-hostname
│   │               ├── local-ipv4s
│   │               ├── mac
│   │               ├── owner-id
│   │               ├── security-group-ids
│   │               ├── security-groups
│   │               ├── subnet-id
│   │               ├── subnet-ipv4-cidr-block
│   │               ├── vpc-id
│   │               └── vpc-ipv4-cidr-block
│   ├── placement
│   │   └── availability-zone
│   ├── profile
│   ├── public-keys
│   │   └── 0
│   │       └── openssh-key
│   ├── reservation-id
│   ├── security-groups
│   └── services
│       └── domain
│           └── amazonaws.com
├── tags
│   ├── createdBy
│   ├── name
│   └── role
└── user-data

16 directories, 42 files
$ cat /tmp/aws/meta-data/instance-id
i-123456
$ cat /tmp/aws/user-data
#! /bin/bash
echo 'Hello world'
$ cat /tmp/aws/tags/name
My Instance Name
```

### Advantages over `curl http://169.254.169.254`

* No need to remember the special IP address of the service
* Can use traditional unix tools to walk and interrogate the tree
* Tab completion of paths
* **Support for tags**

### Advantages over the [`ec2-metadata`](http://aws.amazon.com/code/1825) tool

* No need to `cut` the output of commands to get just the field
* Access to all metadata fields, not just the limited subset the tool returns
* **Support for tags**

Feedback and feature requests are welcome!

## Installing

#### Linux (64 bit)

```bash
curl -sL https://github.com/jszwedko/ec2-metadatafs/releases/download/0.4.0/linux_amd64 > ec2-metadatafs
sudo mv ec2-metadatafs /usr/bin/
sudo chmod +x /usr/bin/ec2-metadatafs
```

#### Linux (32 bit)

```bash
curl -sL https://github.com/jszwedko/ec2-metadatafs/releases/download/0.4.0/linux_386 > ec2-metadatafs
sudo mv ec2-metadatafs /usr/bin/
sudo chmod +x /usr/bin/ec2-metadatafs
```

Install the latest via: `GOVENDOREXPERIMENT=1 go get
github.com/jszwedko/ec2-metadatafs` (requires Go >= 1.5 to be installed).

You can have it automatically mount by adding the following to `/etc/fstab`:

`ec2-metadatafs   /aws    fuse    _netdev,allow_other    0    0`

Or

`ec2-metadatafs   /aws    fuse    _netdev,allow_other,tags    0    0`

if you want to mount the tags as well (requires AWS API credentials -- described below).

## Usage

```
Usage:
  ec2-metadatafs [OPTIONS] mountpoint

ec2metadatafs mounts a FUSE filesystem which exposes the EC2 instance metadata
(and optionally the tags) of the host as files and directories rooted at the
given location.

Application Options:
  -v, --verbose                Print verbose logs, can be specified multiple times (up to 2)
  -f, --foreground             Run in foreground
  -V, --version                Display version info
  -e, --endpoint=              EC2 metadata service HTTP endpoint (default: http://169.254.169.254/latest/)
  -c, --cachesec=              Number of seconds to cache files attributes and directory listings. 0 to disable, -1 for
                               indefinite. (default: 0)
  -t, --tags                   Mount EC2 instance tags at <mount point>/tags
  -o, --options=               Mount options, see below for description
  -n, --no-syslog              Disable syslog when daemonized
  -F, --syslog-facility=       Syslog facility to use when daemonized (see below for options) (default: USER)

AWS Credentials (only used when mounting tags):
      --aws-access-key-id=     AWS Access Key ID (adds to credential chain, see below)
      --aws-secret-access-key= AWS Secret Access key (adds to credential chain, see below)
      --aws-session-token=     AWS session token (adds to credential chain, see below)

Help Options:
  -h, --help                   Show this help message

Arguments:
  mountpoint:                  Directory to mount the filesystem at

Mount options:
  -o debug                     Enable debug logging, same as -v
  -o fuse_debug                Enable fuse_debug logging (implies debug), same as -vv
  -o endpoint=ENDPOINT         EC2 metadata service HTTP endpoint, same as --endpoint=
  -o tags                      Mount the instance tags at <mount point>/tags, same as --tags
  -o aws_access_key_id=ID      AWS API access key (see below), same as --aws-access-key-id=
  -o aws_secret_access_key=KEY AWS API secret key (see below), same as --aws-secret-access-key=
  -o aws_session_token=KEY     AWS API session token (see below), same as --aws-session-token=
  -o cachesec=SEC              Number of seconds to cache files attributes and directory listings, same as --cachesec
  -o syslog_facility=                                    Syslog facility to send messages upon when daemonized (see below)
  -o no_syslog                 Disable logging to syslog when daemonized
  -o FUSEOPTION=OPTIONVALUE    FUSE mount option, please see the OPTIONS section of your FUSE manual for valid options

AWS credential chain:
  AWS credentials only required when mounting the instance tags (--tags or -o tags).

  Checks for credentials in the following places, in order:

  - Provided AWS credentials via flags or mount options
  - $AWS_ACCESS_KEY_ID, $AWS_SECRET_ACCESS_KEY, and $AWS_SESSION_TOKEN environment variables
  - Shared credentials file -- respects $AWS_DEFAULT_PROFILE and $AWS_SHARED_CREDENTIALS_FILE
  - IAM role associated with the instance

  Note that the AWS session token is only needed for temporary credentials from AWS security token service.

Caching:

Caching of the following is supported and controlled via the cachesec parameter:

* File attributes
* Directory attributes
* Directory listings

When accessed this metadata will be cached for the number of seconds specified
by cachesec. Use 0, the default, to disable caching and -1 to cache
indefinitely (good if you never expect instance metadata to change). This cache
is kept in memory and lost when the process is restarted.

Valid syslog facilities:
  KERN, USER, MAIL, DAEMON, AUTH, SYSLOG, LPR, NEWS, UUCP, CRON, AUTHPRIV, FTP, LOCAL0, LOCAL1, LOCAL2, LOCAL3, LOCAL4, LOCAL5, LOCAL6, LOCAL7

Version:
  0.3.0-16-gb73643f-dirty ('b73643f6a5aface7e405429779e8554a7b3767c8')

Author:
  Jesse Szwedko

Project Homepage:
  http://github.com/jszwedko/ec2-metadatafs

Report bugs to:
  http://github.com/jszwedko/ec2-metadatafs/issues
```

### AWS permissions

If you are mounting the instance tags, AWS API credentials are required. It is
recommended that you associate an IAM instance role with your instances to
support this (see
[iam-roles](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html)
for details) to avoid the usual issues with static credentials, but you can
also provide credentials via the environment, command line flags, or a file.

These credentials have access to query for the AWS API for tags -- example IAM policy:

```
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [ "ec2:DescribeTags"],
      "Resource": ["*"]
    }
  ]
}
```

See [Usage](#usage) section for more details on credential sources.

### Developing

Requires Go 1.5 and
[`GOVENDOREXPERIMENT=1`](https://docs.google.com/document/d/1Bz5-UB7g2uPBdOx-rw5t9MxJwkfpx90cqG9AFL0JAYo/edit)
to properly include dependencies.

Uses [`gvt`](https://github.com/FiloSottile/gvt) to manipulate dependencies.

- Building: `make build`
- Testing: `make test`
- Building cross compiled binaries: `make dist` (will install
  [gox](https://github.com/mitchellh/gox) if needed)
