package metadatafs

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/jszwedko/ec2-metadatafs/logger"
)

// MetadataFs represents a filesystem that exposes metadata about EC2 instances
// Satisfies pathfs.FileSystem
type MetadataFs struct {
	pathfs.FileSystem

	Client MetadataClient

	Logger logger.LeveledLogger
}

// MetadataClient is a client for accessing the AWS Instance Metadata Service
type MetadataClient interface {
	Head(path string) (resp *http.Response, err error)
	Get(path string) (resp *http.Response, err error)
}

// New initializes a new MetadataFs that uses the given endpoint as the
// target of metadata requests
func New(client MetadataClient, l logger.LeveledLogger) *MetadataFs {
	return &MetadataFs{
		FileSystem: pathfs.NewReadonlyFileSystem(pathfs.NewDefaultFileSystem()),
		Client:     client,
		Logger:     l,
	}
}

// StatFs returns the statistics of the filesystem
//
// Currently stubbed to return the empty struct to satisfy programs like `df`
// until we are able to implement accurate filesystem statistics
func (fs *MetadataFs) StatFs(name string) *fuse.StatfsOut {
	return &fuse.StatfsOut{}
}

// GetAttr returns an fuse.Attr representing a read-only file or directory
func (fs *MetadataFs) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	resp, err := fs.Client.Head(name)
	if err != nil {
		fs.Logger.Errorf("failed to query AWS metadata API: %s", err)
		return nil, fuse.EIO
	}

	switch resp.StatusCode {
	case http.StatusNotFound:
		fs.Logger.Debugf("returning ENOENT for %s", name)
		return nil, fuse.ENOENT
	case http.StatusOK:
		if isDir(name) {
			fs.Logger.Debugf("determined '%s' is a directory", name)
			return fs.httpResponseToAttr(resp, true), fuse.OK
		}
		fs.Logger.Debugf("determined '%s' is a file", name)
		return fs.httpResponseToAttr(resp, false), fuse.OK
	default:
		fs.Logger.Errorf("unknown HTTP status code from AWS metadata API: %d", resp.StatusCode)
		return nil, fuse.EIO
	}
}

// OpenDir returns the list of paths under the given path
func (fs *MetadataFs) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	resp, err := fs.Client.Get(name)
	if err != nil {
		fs.Logger.Errorf("failed to query AWS metadata API: %s", err)
		return nil, fuse.EIO
	}

	switch resp.StatusCode {
	case http.StatusNotFound:
		fs.Logger.Debugf("returning file not found for %s", name)
		return nil, fuse.ENOENT
	case http.StatusOK:
		if !isDir(name) {
			fs.Logger.Debugf("returning ENOTDIR for %s", name)
			return nil, fuse.ENOTDIR
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fs.Logger.Errorf("failed to query AWS metadata API: %s", err)
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

			// special case for user-data which is always returned as a listing, but can be non-existent
			// as far as I can tell, this is the only path that this happens with
			switch file {
			case "user-data":
				if _, status := fs.GetAttr(path.Join(name, file), context); status == fuse.ENOENT {
					continue
				}
			}

			if isDir(path.Join(name, file)) {
				fs.Logger.Debugf("adding dir entry for '%s' as directory", file)
				dirEntries = append(dirEntries, fuse.DirEntry{Name: file, Mode: fuse.S_IFDIR})
			} else {
				fs.Logger.Debugf("adding dir entry for '%s' as file", file)
				dirEntries = append(dirEntries, fuse.DirEntry{Name: file, Mode: fuse.S_IFREG})
			}
		}

		return dirEntries, fuse.OK
	default:
		fs.Logger.Errorf("unknown HTTP status code from AWS metadata API: %d", resp.StatusCode)
		return nil, fuse.EIO
	}
}

// Open returns a datafile representing the HTTP response body
func (fs *MetadataFs) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	resp, err := fs.Client.Get(name)
	if err != nil {
		fs.Logger.Errorf("failed to query AWS metadata API: %s", err)
		return nil, fuse.EIO
	}

	switch resp.StatusCode {
	case http.StatusNotFound:
		return nil, fuse.ENOENT
	case http.StatusOK:
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fs.Logger.Errorf("failed to query AWS metadata API: %s", err)
			return nil, fuse.EIO
		}

		return nodefs.NewDataFile(body), fuse.OK
	default:
		fs.Logger.Errorf("unknown HTTP status code from AWS metadata API: %d", resp.StatusCode)
		return nil, fuse.EIO
	}
}

// httpResponseToAttr converts an http.Response from the AWS metadata service to a fuse.Attr
func (fs *MetadataFs) httpResponseToAttr(resp *http.Response, dir bool) *fuse.Attr {
	attr := &fuse.Attr{}

	lastModified, err := time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
	if err != nil {
		fs.Logger.Warningf("couldn't parse Last-Modified '%s' as time: %s", resp.Header.Get("Last-Modified"), err)
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

// Hardcoded directory pattern
// See http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html
// TODO dynamically determine and cache paths that are directories
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

func joinURL(base string, paths ...string) string {
	p := path.Join(paths...)
	return fmt.Sprintf("%s/%s", strings.TrimRight(base, "/"), strings.TrimLeft(p, "/"))
}
