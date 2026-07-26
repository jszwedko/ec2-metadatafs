## ec2-metadatafs: `cat` your AWS EC2 metadata

`ec2-metadatafs` exposes [AWS EC2
metadata](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html) as a filesystem for easy `ls`,
`cat`, and `grep`ing. It relies on [FUSE](https://github.com/libfuse/libfuse) to mount a user-space filesystem with
files exposing the EC2 metadata and, optionally, the tags on the instance in the form of a readonly filesystem.

Example:
```
$ mkdir /var/run/aws
$ ec2-metadatafs --tags /var/run/aws
$ tree /var/run/aws
/var/run/aws
├── dynamic
│   └── instance-identity
│       ├── document
│       ├── pkcs7
│       ├── rsa2048
│       └── signature
└── meta-data
    ├── ami-id
    ├── ami-launch-index
    ├── ami-manifest-path
    ├── block-device-mapping
    │   ├── ami
    │   ├── ephemeral0
    │   ├── ephemeral1
    │   └── root
    ├── events
    ├── hostname
    ├── identity-credentials
    ├── instance-action
    ├── instance-id
    ├── instance-life-cycle
    ├── instance-type
    ├── local-hostname
    ├── local-ipv4
    ├── mac
    ├── metrics
    ├── network
    │   └── interfaces
    │       └── macs
    │           └── 06:6f:de:da:f0:3d
    │               ├── device-number
    │               ├── interface-id
    │               ├── ipv4-associations
    │               ├── local-hostname
    │               ├── local-ipv4s
    │               ├── mac
    │               ├── owner-id
    │               ├── public-hostname
    │               ├── public-ipv4s
    │               ├── security-group-ids
    │               ├── security-groups
    │               ├── subnet-id
    │               ├── subnet-ipv4-cidr-block
    │               ├── vpc-id
    │               ├── vpc-ipv4-cidr-block
    │               └── vpc-ipv4-cidr-blocks
    ├── placement
    │   ├── availability-zone
    │   ├── availability-zone-id
    │   └── region
    ├── profile
    ├── public-hostname
    ├── public-ipv4
    ├── public-keys
    │   └── 0
    │       └── openssh-key
    ├── reservation-id
    ├── security-groups
    ├── services
    │   ├── domain
    │   └── partition
    └── system

14 directories, 49 files
$ cat /var/run/aws/meta-data/instance-id
i-0b22a22eec53b9321
$ cat /var/run/aws/user-data
#! /bin/bash
echo 'Hello world'
$ cat /var/run/aws/tags/name
My Instance Name
```

### Advantages over `curl http://169.254.169.254`

* **Support for tags**
* Use filesystem permissions to control access
* Use traditional unix tools to walk and interrogate the tree
* Tab completion of paths
* No need to remember the special IP address of the service

### Advantages over the [`ec2-metadata`](http://aws.amazon.com/code/1825) tool

* **Support for tags**
* No need to `cut` the output of commands to get just the field
* Can use filesystem permissions to control access
* Access to all metadata fields, not just the limited subset the tool returns

Feedback and feature requests are welcome!

## Installing

### Packages

Packages are built as `.deb`, `.rpm`, `.apk`, and Arch Linux packages. See
[releases](https://github.com/jszwedko/ec2-metadatafs/releases) to install one of these.

### From source

Install the latest via: `go install github.com/jszwedko/ec2-metadatafs@latest`

## Usage

```
Usage:
  ec2-metadatafs [OPTIONS] [mountpoint]

ec2metadatafs mounts a FUSE filesystem which exposes the EC2 instance metadata
(and optionally the tags) of the host as files and directories rooted at the
given location.

Application Options:
  -v, --verbose                                   Print verbose logs, can be specified multiple times (up to 2)
  -f, --foreground                                Run in foreground
  -V, --version                                   Display version info
      --endpoint=                                 Deprecated alias for --instance-metadata-service-endpoint
  -e, --instance-metadata-service-endpoint=       Instance Metadata Service HTTP endpoint (default: http://169.254.169.254/latest/)
  -m, --instance-metadata-service-version=[v1|v2] Instance Metadata Service version (default: v2)
  -T, --instance-metadata-service-token-ttl=      Instance Metadata Service token TTL (only valid for Instance Metadata Service version v2) (default: 6h)
  -c, --cachesec=                                 Number of seconds to cache files attributes and directory listings. 0 to disable, -1 for indefinite. (default: 0)
  -t, --tags                                      Mount EC2 instance tags at <mount point>/tags
  -o, --options=                                  Mount options, see below for description
  -n, --no-syslog                                 Disable syslog when daemonized
  -F, --syslog-facility=                          Syslog facility to use when daemonized (see below for options) (default: USER)

AWS Credentials (only used when mounting tags):
      --aws-access-key-id=                        AWS Access Key ID (adds to credential chain, see below)
      --aws-secret-access-key=                    AWS Secret Access key (adds to credential chain, see below)
      --aws-session-token=                        AWS session token (adds to credential chain, see below)

Help Options:
  -h, --help                                      Show this help message

Arguments:
  mountpoint:                                     Directory to mount the filesystem at

Mount options:
  -o debug                                        Enable debug logging, same as -v
  -o fuse_debug                                   Enable fuse_debug logging (implies debug), same as -vv
  -o endpoint=ENDPOINT                            Deprecated alias for -o instance_metadata_service_endpoint=
  -o instance_metadata_service_endpoint=ENDPOINT  Instance metadata service HTTP endpoint, same as --instance-metadata-service-endpoint=
  -o instance_metadata_service_version=VERSION    Instance Metadata Service version, v1 or v2, same as --instance-metadata-service-version=
  -o instance_metadata_service_token_ttl=TTL      Instance Metadata Service token TTL, only valid with service_version=v2, same as --instance-metadata-service-token-ttl=
  -o tags                                          Mount the instance tags at <mount point>/tags, same as --tags
  -o aws_access_key_id=ID                         AWS API access key (see below), same as --aws-access-key-id=
  -o aws_secret_access_key=KEY                    AWS API secret key (see below), same as --aws-secret-access-key=
  -o aws_session_token=KEY                        AWS API session token (see below), same as --aws-session-token=
  -o cachesec=SEC                                 Number of seconds to cache files attributes and directory listings, same as --cachesec
  -o syslog_facility=                             Syslog facility to send messages upon when daemonized (see below)
  -o no_syslog                                    Disable logging to syslog when daemonized
  -o FUSEOPTION=OPTIONVALUE                       FUSE mount option, please see the OPTIONS section of your FUSE manual for valid options

AWS credential chain:
  AWS credentials only required when mounting the instance tags (--tags or -o tags).

  Checks for credentials in the following places, in order:

  - Provided AWS credentials via flags or mount options
  - $AWS_ACCESS_KEY_ID, $AWS_SECRET_ACCESS_KEY, and $AWS_SESSION_TOKEN environment variables
  - Shared credentials file -- respects $AWS_DEFAULT_PROFILE and $AWS_SHARED_CREDENTIALS_FILE
  - IAM role associated with the instance

  Note that the AWS session token is only needed for temporary credentials from AWS security token service.

Instance Metadata Service (IMDS) Version:

AWS has two modes for interacting with the metadata API:

* v1: request/response method (traditional)
* v2: session-oriented method (more secure)

If you are unsure, choose v2, which is the default.

See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html for additional details.

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


Author:
  Jesse Szwedko

Project Homepage:
  http://github.com/jszwedko/ec2-metadatafs

Report bugs to:
  http://github.com/jszwedko/ec2-metadatafs/issues
```

## Automatic mounting

You can have it automatically mount by adding the following to `/etc/fstab`:

`ec2-metadatafs   /var/run/aws    fuse    _netdev,allow_other    0    0`

Or

`ec2-metadatafs   /var/run/aws    fuse    _netdev,allow_other,tags    0    0`

if you want to mount the tags as well (requires AWS API credentials -- described below).

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

- Building: `make build`
- Testing: `make test`
- Building release artifacts (cross compiled binaries and packages): `goreleaser release --snapshot --clean` (requires [goreleaser](https://goreleaser.com))
