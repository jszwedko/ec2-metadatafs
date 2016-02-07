package tagsfs

import (
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

// TagsFs represents a filesystem that exposes the instance tags
// Satisfies pathfs.FileSystem
// Currently is readonly
type TagsFs struct {
	pathfs.FileSystem

	Client     *ec2.EC2
	InstanceID string
}

// New initializes a new TagsFs that uses the given AWS client
func New(client *ec2.EC2, instanceID string) *TagsFs {
	return &TagsFs{
		FileSystem: pathfs.NewReadonlyFileSystem(pathfs.NewDefaultFileSystem()),
		Client:     client,
		InstanceID: instanceID,
	}
}

// GetAttr returns an fuse.Attr representing a read-only file or directory
func (fs *TagsFs) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	if name == "" {
		return &fuse.Attr{Size: 4096, Mode: fuse.S_IFDIR | 0555}, fuse.OK
	}

	log.Printf("[DEBUG] issuing request to AWS API for tag: %s", name)

	resp, err := fs.Client.DescribeTags(&ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			{Name: aws.String("key"), Values: []*string{aws.String(name)}},
			{Name: aws.String("resource-id"), Values: []*string{aws.String(fs.InstanceID)}},
		},
	})

	if err != nil {
		log.Printf("[ERROR] failed to query AWS API: %s", err)
		return nil, fuse.EIO
	}

	if len(resp.Tags) == 0 {
		log.Printf("[DEBUG] no tag found for %s", name)
		return nil, fuse.ENOENT
	}

	return &fuse.Attr{
		Size: uint64(len(*resp.Tags[0].Value)),
		Mode: fuse.S_IFREG | 0444,
	}, fuse.OK
}

// OpenDir returns the list of paths under the given path
// GetAttr is called on the file first, so we do not worry about this being called on non-dirs
func (fs *TagsFs) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	log.Printf("[DEBUG] issuing request to AWS API for instance tags")

	resp, err := fs.Client.DescribeTags(&ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			{Name: aws.String("resource-id"), Values: []*string{aws.String(fs.InstanceID)}},
		},
	})

	if err != nil {
		log.Printf("[ERROR] failed to query AWS API: %s", err)
		return nil, fuse.EIO
	}

	dirEntries := make([]fuse.DirEntry, 0, len(resp.Tags))
	for _, tag := range resp.Tags {
		log.Printf("[DEBUG] adding dir entry for tag '%s'", *tag.Key)
		dirEntries = append(dirEntries, fuse.DirEntry{Name: *tag.Key, Mode: fuse.S_IFREG})
	}

	return dirEntries, fuse.OK
}

// Open returns a datafile representing the tag value
func (fs *TagsFs) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	log.Printf("[DEBUG] issuing request to AWS API for tag: %s", name)

	resp, err := fs.Client.DescribeTags(&ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			{Name: aws.String("key"), Values: []*string{aws.String(name)}},
			{Name: aws.String("resource-id"), Values: []*string{aws.String(fs.InstanceID)}},
		},
	})

	if err != nil {
		log.Printf("[ERROR] failed to query AWS API: %s", err)
		return nil, fuse.EIO
	}

	if len(resp.Tags) == 0 {
		log.Printf("[DEBUG] no tag found for %s", name)
		return nil, fuse.ENOENT
	}

	return nodefs.NewDataFile([]byte(*resp.Tags[0].Value)), fuse.OK
}
