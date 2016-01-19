package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

var (
	// VersionString is the git tag this binary is associated with
	VersionString string
	// RevisionString is the git rev this binary is associated with
	RevisionString string
)

func main() {
	var (
		endpoint    string
		showVersion bool
	)
	flag.StringVar(&endpoint, "endpoint", "http://169.254.169.254/latest/", "AWS EC2 metadata endpoint")
	flag.BoolVar(&showVersion, "version", false, "Display version")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: ec2metadafs [flags] <mount point>")
		flag.PrintDefaults()

		fmt.Fprintf(os.Stderr, `
Description:
	ec2metadafs mounts a FUSE filesystem at the given location which exposes the
  EC2 instance metadata of the host as files and directories mimicking the URL
  structure of the metadata service.

Version:
  %s (%s)

Author:
  Jesse Szwedko

Project Homepage:
  http://github.com/jszwedko/ec2metadafs

Report bugs to:
  http://github.com/jszwedko/ec2metadafs/issues
`, VersionString, RevisionString)
	}
	flag.Parse()

	if showVersion {
		fmt.Printf("%s (%s)\n", VersionString, RevisionString)
		os.Exit(0)
	}

	if len(flag.Args()) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	nfs := pathfs.NewPathNodeFs(NewMetadataFs(endpoint), nil)
	server, _, err := nodefs.MountRoot(flag.Arg(0), nfs.Root(), nodefs.NewOptions())
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}
	server.Serve()
}
