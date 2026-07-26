// Ported from github.com/hanwen/go-fuse's unionfs package (BSD-licensed),
// which was removed when upgrading to go-fuse v2. Trimmed to only the
// attribute and directory-listing caching that ec2-metadatafs relies on.
package cachingfs

import (
	"fmt"
	"time"

	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/fuse/pathfs"
)

type attrResponse struct {
	*fuse.Attr
	fuse.Status
}

type dirResponse struct {
	entries []fuse.DirEntry
	fuse.Status
}

// cachingFileSystem caches file attributes and directory listings for a
// wrapped pathfs.FileSystem.
type cachingFileSystem struct {
	pathfs.FileSystem

	attributes *timedCache
	dirs       *timedCache
}

// New returns a pathfs.FileSystem that caches the results of GetAttr and
// OpenDir calls to fs for the given ttl. A ttl of 0 disables caching and a
// negative ttl caches indefinitely.
func New(fs pathfs.FileSystem, ttl time.Duration) pathfs.FileSystem {
	c := &cachingFileSystem{FileSystem: fs}
	c.attributes = newTimedCache(func(n string) (interface{}, bool) {
		a, code := fs.GetAttr(n, nil)
		return &attrResponse{Attr: a, Status: code}, code.Ok()
	}, ttl)
	c.dirs = newTimedCache(func(n string) (interface{}, bool) {
		entries, code := fs.OpenDir(n, nil)
		return &dirResponse{entries: entries, Status: code}, code.Ok()
	}, ttl)
	return c
}

func (fs *cachingFileSystem) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	r := fs.attributes.Get(name).(*attrResponse)
	return r.Attr, r.Status
}

func (fs *cachingFileSystem) OpenDir(name string, context *fuse.Context) (stream []fuse.DirEntry, status fuse.Status) {
	r := fs.dirs.Get(name).(*dirResponse)
	return r.entries, r.Status
}

func (fs *cachingFileSystem) String() string {
	return fmt.Sprintf("cachingFileSystem(%v)", fs.FileSystem)
}
