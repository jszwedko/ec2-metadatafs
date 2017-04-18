package metadatafs

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

// New initializes a new MetadataFs that uses the given endpoint as the
// target of metadata requests
func New(endpoint string) *MetadataFs {
	return &MetadataFs{
		FileSystem: pathfs.NewReadonlyFileSystem(pathfs.NewDefaultFileSystem()),
		Client:     &http.Client{},
		Endpoint:   endpoint,
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
	url := fs.Endpoint + name

	log.Printf("[DEBUG] issuing HTTP HEAD to AWS metadata API for path: %s", url)
	resp, err := fs.Client.Head(url)
	if err != nil {
		log.Printf("[ERROR] failed to query AWS metadata API: %s", err)
		return nil, fuse.EIO
	}

	log.Printf("[DEBUG] got %d from AWS metadata API for path: %s", resp.StatusCode, url)
	switch resp.StatusCode {
	case http.StatusNotFound:
		log.Printf("[DEBUG] returning ENOENT for %s", name)
		return nil, fuse.ENOENT
	case http.StatusOK:
		if isDir(name) {
			log.Printf("[DEBUG] determined '%s' is a directory", name)
			return httpResponseToAttr(resp, true), fuse.OK
		}
		log.Printf("[DEBUG] determined '%s' is a file", name)
		return httpResponseToAttr(resp, false), fuse.OK
	default:
		log.Printf("[ERROR] unknown HTTP status code from AWS metadata API: %d", resp.StatusCode)
		return nil, fuse.EIO
	}
}

// OpenDir returns the list of paths under the given path
func (fs *MetadataFs) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	url := fs.Endpoint + name

	log.Printf("[DEBUG] issuing HTTP GET to AWS metadata API for path: %s", url)
	resp, err := fs.Client.Get(url)
	if err != nil {
		log.Printf("[ERROR] failed to query AWS metadata API: %s", err)
		return nil, fuse.EIO
	}

	log.Printf("[DEBUG] got %d from AWS metadata API for path: %s", resp.StatusCode, url)
	switch resp.StatusCode {
	case http.StatusNotFound:
		log.Printf("[DEBUG] returning file not found for %s", name)
		return nil, fuse.ENOENT
	case http.StatusOK:
		if !isDir(name) {
			log.Printf("[DEBUG] returning ENOTDIR for %s", name)
			return nil, fuse.ENOTDIR
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("[ERROR] failed to query AWS metadata API: %s", err)
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
				log.Printf("[DEBUG] adding dir entry for '%s' as directory", file)
				dirEntries = append(dirEntries, fuse.DirEntry{Name: file, Mode: fuse.S_IFDIR})
			} else {
				log.Printf("[DEBUG] adding dir entry for '%s' as file", file)
				dirEntries = append(dirEntries, fuse.DirEntry{Name: file, Mode: fuse.S_IFREG})
			}
		}

		return dirEntries, fuse.OK
	default:
		log.Printf("[ERROR] unknown HTTP status code from AWS metadata API: %d", resp.StatusCode)
		return nil, fuse.EIO
	}
}

// Open returns a datafile representing the HTTP response body
func (fs *MetadataFs) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	url := fs.Endpoint + name

	log.Printf("[DEBUG] issuing HTTP GET to AWS metadata API for path: '%s'", url)
	resp, err := fs.Client.Get(url)
	if err != nil {
		log.Printf("[ERROR] failed to query AWS metadata API: %s", err)
		return nil, fuse.EIO
	}

	log.Printf("[DEBUG] got %d from AWS metadata API for path %s", resp.StatusCode, url)
	switch resp.StatusCode {
	case http.StatusNotFound:
		return nil, fuse.ENOENT
	case http.StatusOK:
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("[ERROR] failed to query AWS metadata API: %s", err)
			return nil, fuse.EIO
		}

		newlinebody := append(body, 0x0a) // newline
		return nodefs.NewDataFile(newlinebody), fuse.OK
	default:
		log.Printf("[ERROR] unknown HTTP status code from AWS metadata API: %d", resp.StatusCode)
		return nil, fuse.EIO
	}
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

// httpResponseToAttr converts an http.Response from the AWS metadata service to a fuse.Attr
func httpResponseToAttr(resp *http.Response, dir bool) *fuse.Attr {
	attr := &fuse.Attr{}

	lastModified, err := time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
	if err != nil {
		log.Printf("[WARN] couldn't parse Last-Modified '%s' as time: %s", resp.Header.Get("Last-Modified"), err)
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
