package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/jessevdk/go-flags"
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
	Foreground   bool         `short:"f" long:"foreground" description:"Run in foreground"`
	Version      bool         `short:"v" long:"version"    description:"Display version info"`
	Endpoint     string       `short:"e" long:"endpoint"   description:"EC2 metadata service HTTP endpoint" default:"http://169.254.169.254/latest/"`
	MountOptions mountOptions `short:"o" long:"options"    description:"Mount options, see below for description"`

	Args struct {
		Mountpoint string `positional-arg-name:"mountpoint" description:"Directory to mount the filesystem at"`
	} `positional-args:"yes" required:"yes"`
}

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

func mountAndServe(endpoint, mountpoint string, opts mountOptions) {
	nfs := pathfs.NewPathNodeFs(NewMetadataFs(endpoint), nil)
	server, err := fuse.NewServer(nodefs.NewFileSystemConnector(nfs.Root(), nil).RawFS(), mountpoint, &fuse.MountOptions{Options: opts.opts})
	if err != nil {
		log.Fatalf("mount fail: %v\n", err)
	}
	server.Serve()
}

func main() {
	options := &Options{}

	parser := flags.NewParser(options, flags.HelpFlag|flags.PassDoubleDash)
	parser.LongDescription = `
ec2metadafs mounts a FUSE filesystem at the given location which exposes the
EC2 instance metadata of the host as files and directories mirroring the URL
structure of the metadata service.`

	_, err := parser.Parse()
	if options.Version {
		fmt.Printf("%s (%s)\n", VersionString, RevisionString)
		os.Exit(0)
	}

	if parser.FindOptionByLongName("help").IsSet() {
		parser.WriteHelp(os.Stdout)
		fmt.Printf(`Mount options:
  -o endpoint=ENDPOINT       EC2 metadata service HTTP endpoint, same as --endpoint=
  -o FUSEOPTION=OPTIONVALUE  FUSE mount option, please see the OPTIONS section of your FUSE manual for valid options

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

	if options.Foreground {
		mountAndServe(options.Endpoint, options.Args.Mountpoint, options.MountOptions)
		return
	}

	// daemonize
	context := new(daemon.Context)
	child, err := context.Reborn()
	if err != nil {
		log.Fatalf("mount fail: %v\n", err)
	}

	if child == nil {
		defer context.Release()
		mountAndServe(options.Endpoint, options.Args.Mountpoint, options.MountOptions)
	}
}
