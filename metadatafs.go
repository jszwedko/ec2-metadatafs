package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

// MetadataFs represents a filesystem that exposes metadata about EC2 instances
// Satisfies pathfs.FileSystem
type MetadataFs struct {
	pathfs.FileSystem

	Client   *http.Client
	Endpoint string
}

// NewMetadataFs initializes a new MetadataFs that uses the given endpoint as the
// target of metadata requests
func NewMetadataFs(endpoint string) *MetadataFs {
	return &MetadataFs{
		FileSystem: pathfs.NewReadonlyFileSystem(pathfs.NewDefaultFileSystem()),
		Client:     &http.Client{},
		Endpoint:   endpoint,
	}
}

// GetAttr returns an fuse.Attr representing a read-only file or directory
func (fs *MetadataFs) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	resp, err := fs.Client.Head(fs.Endpoint + name)
	if err != nil {
		log.Printf("error querying AWS metadata API: %s", err)
		return nil, fuse.EIO
	}

	switch resp.StatusCode {
	case http.StatusNotFound:
		return nil, fuse.ENOENT
	case http.StatusOK:
		if isDir(name) {
			return httpResponseToAttr(resp, true), fuse.OK
		}
		return httpResponseToAttr(resp, false), fuse.OK
	default:
		log.Printf("unknown HTTP status code from AWS metadata API: %d", resp.StatusCode)
		return nil, fuse.EIO
	}
}

// OpenDir returns the list of paths under the given path
func (fs *MetadataFs) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	resp, err := fs.Client.Get(fs.Endpoint + name)
	if err != nil {
		log.Printf("error querying AWS metadata API: %s", err)
		return nil, fuse.EIO
	}

	switch resp.StatusCode {
	case http.StatusNotFound:
		return nil, fuse.ENOENT
	case http.StatusOK:
		if !isDir(name) {
			return nil, fuse.ENOTDIR
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("error querying AWS metadata API: %s", err)
			return nil, fuse.EIO
		}

		if name == "meta-data/public-keys" {
			body = []byte("0")
		}

		files := strings.Split(string(body), "\n")
		dirEntries := make([]fuse.DirEntry, 0, len(files))
		for _, file := range files {
			file = strings.TrimRight(strings.TrimSpace(file), "/")

			if len(file) == 0 {
				continue
			}

			if isDir(path.Join(name, file)) {
				dirEntries = append(dirEntries, fuse.DirEntry{Name: file, Mode: fuse.S_IFDIR})
			} else {
				dirEntries = append(dirEntries, fuse.DirEntry{Name: file, Mode: fuse.S_IFREG})
			}
		}

		return dirEntries, fuse.OK
	default:
		log.Printf("unknown HTTP status code from AWS metadata API: %d", resp.StatusCode)
		return nil, fuse.EIO
	}
}

// Open returns a datafile representing the HTTP response body
func (fs *MetadataFs) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	resp, err := fs.Client.Get(fs.Endpoint + name)
	if err != nil {
		log.Printf("error querying AWS metadata API: %s", err)
		return nil, fuse.EIO
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("error querying AWS metadata API: %s", err)
		return nil, fuse.EIO
	}

	switch resp.StatusCode {
	case http.StatusNotFound:
		return nil, fuse.ENOENT
	case http.StatusOK:
		return nodefs.NewDataFile(body), fuse.OK
	default:
		log.Printf("unknown HTTP status code from AWS metadata API: %d", resp.StatusCode)
		return nil, fuse.EIO
	}
}

// Hardcoded directory pattern
// See http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html
var directoryRexep = regexp.MustCompile(strings.Replace(`^(
|
meta-data|
meta-data/block-device-mapping|
meta-data/iam|
meta-data/iam/security-credentials|
meta-data/network/interfaces|
meta-data/network|
meta-data/network/interfaces/macs|
meta-data/network/interfaces/macs/[0-9a-f:]+|
meta-data/placement|
meta-data/placement/availability-zone|
meta-data/public-keys|
meta-data/public-keys/0|
meta-data/services|
meta-data/services/domain|
meta-data/spot|
meta-data/spot/termination-time|
dynamic|
dynamic/fws|
dynamic/fws/instance-monitoring|
dynamic/instance-identity)$`, "\n", "", -1))

func isDir(filename string) bool {
	return directoryRexep.MatchString(filename)
}

// httpResponseToAttr converts an http.Response from the AWS metadata service to a fuse.Attr
func httpResponseToAttr(resp *http.Response, dir bool) *fuse.Attr {
	attr := &fuse.Attr{}

	lastModified, err := time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
	if err != nil {
		log.Printf("couldn't parse Last-Modified from AWS metadata API: %s", resp.Header.Get("Last-Modified"))
	}

	attr.SetTimes(nil, &lastModified, &lastModified)

	if dir {
		attr.Size = 4096
		attr.Mode = fuse.S_IFDIR | 0555
	} else {
		attr.Size = uint64(resp.ContentLength)
		attr.Mode = fuse.S_IFREG | 0444
	}
	return attr
}
