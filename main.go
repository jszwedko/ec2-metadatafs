package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/hashicorp/logutils"
	"github.com/jessevdk/go-flags"
	"github.com/jszwedko/ec2-metadatafs/metadatafs"
	"github.com/jszwedko/ec2-metadatafs/tagsfs"
	"github.com/sevlyar/go-daemon"
)

var (
	// VersionString is the git tag this binary is associated with
	VersionString string
	// RevisionString is the git rev this binary is associated with
	RevisionString string
)

// Options holds the command line arguments and flags
// Intended for use with go-flags
type Options struct {
	Verbose      bool         `short:"v" long:"verbose"     description:"Print verbose logs"`
	Foreground   bool         `short:"f" long:"foreground"  description:"Run in foreground"`
	Version      bool         `short:"V" long:"version"     description:"Display version info"`
	Endpoint     string       `short:"e" long:"endpoint"    description:"EC2 metadata service HTTP endpoint" default:"http://169.254.169.254/latest/"`
	Tags         bool         `short:"t" long:"tags"        description:"Mount EC2 instance tags at <mount point>/tags"`
	MountOptions mountOptions `short:"o" long:"options"     description:"Mount options, see below for description"`

	AWSCredentials awsCredentials `group:"AWS Credentials (only used when mounting tags)"`

	Args struct {
		Mountpoint string `positional-arg-name:"mountpoint"   description:"Directory to mount the filesystem at"`
	} `positional-args:"yes" required:"yes"`
}

type awsCredentials struct {
	AWSAccessKeyID     string `long:"aws-access-key-id"     description:"AWS Access Key ID (adds to credential chain, see below)"`
	AWSSecretAccessKey string `long:"aws-secret-access-key" description:"AWS Secret Access key (adds to credential chain, see below)"`
	AWSSessionToken    string `long:"aws-session-token"     description:"AWS session token (adds to credential chain, see below)"`
}

func (a *awsCredentials) credentialChain() *credentials.Credentials {
	return credentials.NewChainCredentials([]credentials.Provider{
		&credentials.StaticProvider{Value: credentials.Value{
			AccessKeyID:     a.AWSAccessKeyID,
			SecretAccessKey: a.AWSAccessKeyID,
			SessionToken:    a.AWSSessionToken}},
		&credentials.EnvProvider{},
		&credentials.SharedCredentialsProvider{},
		&ec2rolecreds.EC2RoleProvider{Client: ec2metadata.New(session.New())},
	})
}

// mountOptions implements flags.Marshaller and flags.Unmarshaller interface to
// read `mount` style options from the user
type mountOptions struct {
	opts []string
}

func (o *mountOptions) String() string {
	return strings.Join(o.opts, ",")
}

func (o *mountOptions) MarshalFlag() (string, error) {
	return o.String(), nil
}

func (o *mountOptions) UnmarshalFlag(s string) error {
	if o.opts == nil {
		o.opts = []string{}
	}

	o.opts = append(o.opts, strings.Split(s, ",")...)

	return nil
}

// ExtractOption deletes the option specified and returns whether the option
// was found and its value (if it has one)
// E.g. endpoint=http://example.com or allow_other
func (o *mountOptions) ExtractOption(s string) (ok bool, value string) {
	if o.opts == nil {
		o.opts = []string{}
	}

	index := -1
	for i, opt := range o.opts {
		parts := strings.SplitN(opt, "=", 2)

		if parts[0] != s {
			continue
		}

		index = i
		if len(parts) == 2 {
			value = parts[1]
		}
		break
	}

	if index != -1 {
		o.opts = append(o.opts[:index], o.opts[index+1:]...)
	}

	return index != -1, value
}

// mountTags mounts another endpoint onto the FUSE FS at tags/ exposing the EC2
// instance tags as files
func mountTags(nfs *pathfs.PathNodeFs, options *Options) {
	svc := ec2metadata.New(session.New())
	instanceID, err := svc.GetMetadata("instance-id")
	if err != nil {
		log.Fatalf("[FATAL] failed to query instance id to initialize tags mount: %v\n", err)
	}
	region, err := svc.Region()
	if err != nil {
		log.Fatalf("[FATAL] failed to query instance region to initialize tags mount: %v\n", err)
	}

	sess := session.New(&aws.Config{
		Region:      aws.String(region),
		Credentials: options.AWSCredentials.credentialChain(),
	})

	status := nfs.Mount(
		"tags",
		pathfs.NewPathNodeFs(tagsfs.New(ec2.New(sess), instanceID), nil).Root(), nil)
	if status != fuse.OK {
		log.Fatalf("[FATAL] tags mount fail: %v\n", status)
	}
}

func mountAndServe(options *Options) {
	log.Printf("[DEBUG] mounting at %s directed at %s with options: %+v", options.Args.Mountpoint, options.Endpoint, options.MountOptions.opts)
	nfs := pathfs.NewPathNodeFs(metadatafs.New(options.Endpoint), nil)
	server, err := fuse.NewServer(
		nodefs.NewFileSystemConnector(nfs.Root(), nil).RawFS(),
		options.Args.Mountpoint,
		&fuse.MountOptions{Options: options.MountOptions.opts})
	if err != nil {
		log.Fatalf("mount fail: %v\n", err)
	}

	if options.Tags {
		go func() {
			server.WaitMount()
			log.Printf("[DEBUG] mounting tags")
			mountTags(nfs, options)
			log.Printf("[DEBUG] tags mounted")
		}()
	}
	log.Printf("[DEBUG] mounting")
	server.Serve()
}

func main() {
	options := &Options{}

	parser := flags.NewParser(options, flags.HelpFlag|flags.PassDoubleDash)
	parser.LongDescription = `
ec2metadafs mounts a FUSE filesystem which exposes the EC2 instance metadata
(and optionally the tags) of the host as files and directories rooted at the
given location.`

	_, err := parser.Parse()

	if options.Version {
		fmt.Printf("%s (%s)\n", VersionString, RevisionString)
		os.Exit(0)
	}

	if parser.FindOptionByLongName("help").IsSet() {
		parser.WriteHelp(os.Stdout)
		fmt.Printf(`
Mount options:
  -o endpoint=ENDPOINT         EC2 metadata service HTTP endpoint, same as --endpoint=
  -o tags                      Mount the instance tags at <mount point>/tags, same as --tags
  -o aws_access_key_id=ID      AWS API access key (see below), same as --aws-access-key-id=
  -o aws_secret_access_key=KEY AWS API secret key (see below), same as --aws-secret-access-key=
  -o aws_session_token=KEY     AWS API session token (see below), same as --aws-session-token=
  -o FUSEOPTION=OPTIONVALUE    FUSE mount option, please see the OPTIONS section of your FUSE manual for valid options

AWS credential chain:
  AWS credentials only required when mounting the instance tags (--tags or -o tags).

  Checks for credentials in the following places, in order:

  - Provided AWS credentials via flags or mount options
  - $AWS_ACCESS_KEY_ID, $AWS_SECRET_ACCESS_KEY, and $AWS_SESSION_TOKEN environment variables
  - Shared credentials file -- respects $AWS_DEFAULT_PROFILE and $AWS_SHARED_CREDENTIALS_FILE
  - IAM role associated with the instance

  Note that the AWS session token is only needed for temporary credentials from AWS security token service.

Version:
  %s (%s)

Author:
  Jesse Szwedko

Project Homepage:
  http://github.com/jszwedko/ec2-metadatafs

Report bugs to:
  http://github.com/jszwedko/ec2-metadatafs/issues
`, VersionString, RevisionString)
		os.Exit(0)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if ok, value := options.MountOptions.ExtractOption("endpoint"); ok {
		options.Endpoint = value
	}

	if ok, value := options.MountOptions.ExtractOption("aws_access_key_id"); ok {
		options.AWSCredentials.AWSAccessKeyID = value
	}

	if ok, value := options.MountOptions.ExtractOption("aws_secret_access_key"); ok {
		options.AWSCredentials.AWSSecretAccessKey = value
	}

	if ok, value := options.MountOptions.ExtractOption("aws_session_token"); ok {
		options.AWSCredentials.AWSSessionToken = value
	}

	if ok, _ := options.MountOptions.ExtractOption("tags"); ok {
		options.Tags = true
	}

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "WARN", "ERROR", "FATAL"},
		MinLevel: logutils.LogLevel("WARN"),
		Writer:   os.Stderr,
	}
	if options.Verbose {
		filter.MinLevel = logutils.LogLevel("DEBUG")
	}
	log.SetOutput(filter)

	if options.Foreground {
		mountAndServe(options)
		return
	}

	// daemonize
	context := new(daemon.Context)
	child, err := context.Reborn()
	if err != nil {
		log.Fatalf("fork fail: %v\n", err)
	}

	if child == nil {
		defer context.Release()
		mountAndServe(options)
	} else {
		log.Printf("forked child with PID %d", child.Pid)
	}
}
