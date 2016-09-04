package tagsfs

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"syscall"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/jszwedko/ec2-metadatafs/internal/logging"
)

func setup(t *testing.T) (svc *ec2.EC2, dir string, cleanup func()) {
	svc = ec2.New(session.New())
	svc.Handlers.Clear()

	tmpDir, err := ioutil.TempDir("", "ec2metadata-test")
	if err != nil {
		t.Fatalf("creating tempdir failed: %v", err)
	}

	fs := New(svc, "i-123456", logging.NewLogger())
	nfs := pathfs.NewPathNodeFs(fs, nil)
	state, _, err := nodefs.MountRoot(tmpDir, nfs.Root(), nodefs.NewOptions())
	if err != nil {
		t.Fatalf("mounting filesystem failed: %v", err)
	}

	go state.Serve()

	return svc, tmpDir, func() {
		state.Unmount()
		os.RemoveAll(tmpDir)
	}
}

func includesString(values []*string, needle string) bool {
	for _, value := range values {
		if needle == *value {
			return true
		}
	}
	return false
}

func serveTags(tags map[string]string) func(*request.Request) {
	return func(r *request.Request) {
		input, ok := r.Params.(*ec2.DescribeTagsInput)
		if !ok {
			r.Error = awserr.NewRequestFailure(
				awserr.New("UnknownError", "500 Internal Server Error", fmt.Errorf("unsupported request")), 500, "",
			)
			return
		}

		restrictToKey := []*string{}
		for _, filter := range input.Filters {
			switch *filter.Name {
			case "resource-id":
			case "key":
				restrictToKey = filter.Values
			default:
				r.Error = awserr.NewRequestFailure(
					awserr.New("UnknownError", "500 Internal Server Error", fmt.Errorf("unsupported filter %s", *filter.Name)), 500, "",
				)
			}
		}

		data := r.Data.(*ec2.DescribeTagsOutput)
		for key, value := range tags {
			if len(restrictToKey) > 0 && !includesString(restrictToKey, key) {
				continue
			}
			data.Tags = append(data.Tags, &ec2.TagDescription{Key: aws.String(key), Value: aws.String(value)})
		}
	}
}

func TestTagsFs_GetAttr_regularFile(t *testing.T) {
	client, dir, cleanup := setup(t)
	defer cleanup()

	client.Handlers.Send.PushBack(serveTags(map[string]string{"name": "MyName"}))

	info, err := os.Stat(path.Join(dir, "name"))
	if err != nil {
		t.Fatalf(`error retrieving stat %s`, err)
	}
	if !info.Mode().IsRegular() {
		t.Errorf(`expected regular file`)
	}
	if info.Size() != 6 {
		t.Errorf(`file size was %d, expected %d`, info.Size(), 6)
	}
}

func TestTagsFs_GetAttr_root(t *testing.T) {
	client, dir, cleanup := setup(t)
	defer cleanup()

	client.Handlers.Send.PushBack(serveTags(map[string]string{"name": "MyName"}))

	info, err := os.Stat(path.Join(dir, ""))
	if err != nil {
		t.Fatalf(`error retrieving stat %s`, err)
	}
	if !info.Mode().IsDir() {
		t.Errorf(`expected directory`)
	}
	if info.Size() != 4096 {
		t.Errorf(`file size was %d, expected %d`, info.Size(), 6)
	}
}

func TestTagsFs_GetAttr_noFile(t *testing.T) {
	client, dir, cleanup := setup(t)
	defer cleanup()

	client.Handlers.Send.PushBack(serveTags(map[string]string{"name": "MyName"}))

	_, err := os.Stat(path.Join(dir, "foobar"))
	if !os.IsNotExist(err) {
		t.Fatalf(`expected to get an error that the file doesn't exist, got %s`, err)
	}
}

func TestTagsFs_GetAttr_error(t *testing.T) {
	client, dir, cleanup := setup(t)
	defer cleanup()

	client.Handlers.Send.PushBack(func(r *request.Request) {
		r.Error = awserr.NewRequestFailure(
			awserr.New("UnknownError", "500 Internal Server Error", fmt.Errorf("mock error")), 500, "",
		)
	})

	_, err := os.Stat(path.Join(dir, "name"))
	if err.(*os.PathError).Err != syscall.EIO {
		t.Fatalf(`expected EIO, got %s`, err)
	}
}

func TestTagsFs_OpenDir(t *testing.T) {
	client, dir, cleanup := setup(t)
	defer cleanup()

	client.Handlers.Send.PushBack(serveTags(map[string]string{"name": "MyName", "role": "MyRole"}))

	fileInfos, err := ioutil.ReadDir(path.Join(dir, "/"))
	if err != nil {
		t.Fatalf(`error listing directory: %s`, err)
	}

	names := make([]string, len(fileInfos))
	for i, fileInfo := range fileInfos {
		names[i] = fileInfo.Name()

		if !fileInfo.Mode().IsRegular() {
			t.Errorf(`returned that %s was not a regular file`, fileInfo.Name())
		}
	}

	if !reflect.DeepEqual([]string{"name", "role"}, names) {
		t.Errorf(`returned entries %+v, expected %+v`, names, []string{"name", "role"})
	}
}

func TestTagsFs_OpenDir_notDir(t *testing.T) {
	client, dir, cleanup := setup(t)
	defer cleanup()

	client.Handlers.Send.PushBack(serveTags(map[string]string{"name": "MyName"}))

	_, err := ioutil.ReadDir(path.Join(dir, "name"))
	if err.(*os.SyscallError).Err != syscall.ENOTDIR {
		t.Fatalf(`expected ENOTDIR, got %s`, err)
	}
}

func TestTagsFs_OpenDir_noFile(t *testing.T) {
	_, dir, cleanup := setup(t)
	defer cleanup()

	_, err := ioutil.ReadDir(path.Join(dir, "foobar"))
	if !os.IsNotExist(err) {
		t.Fatalf(`expected to get an error that the directory doesn't exist, got %s`, err)
	}
}

func TestTagsFs_OpenDir_error(t *testing.T) {
	client, dir, cleanup := setup(t)
	defer cleanup()

	client.Handlers.Send.PushBack(func(r *request.Request) {
		r.Error = awserr.NewRequestFailure(
			awserr.New("UnknownError", "500 Internal Server Error", fmt.Errorf("mock error")), 500, "",
		)
	})

	_, err := ioutil.ReadDir(path.Join(dir, "/"))
	if err.(*os.PathError).Err != syscall.EIO {
		t.Fatalf(`expected EIO, got %s`, err)
	}
}

func TestTagsFs_Open(t *testing.T) {
	client, dir, cleanup := setup(t)
	defer cleanup()

	client.Handlers.Send.PushBack(serveTags(map[string]string{"name": "MyName"}))

	contents, err := ioutil.ReadFile(path.Join(dir, "/name"))
	if err != nil {
		t.Fatalf(`error reading file: %s`, err)
	}

	if string(contents) != "MyName" {
		t.Fatalf(`contents were %s, expected %s`, string(contents), "MyName")
	}
}

func TestTagsFs_Open_noFile(t *testing.T) {
	client, dir, cleanup := setup(t)
	defer cleanup()

	numReqs := 0
	client.Handlers.Send.PushBack(serveTags(map[string]string{"name": "MyName"}))
	client.Handlers.Send.PushBack(func(r *request.Request) {
		if numReqs == 0 { // Allow GetAttr call to succeed
			numReqs++
			return
		}

		data := r.Data.(*ec2.DescribeTagsOutput)
		data.Tags = []*ec2.TagDescription{}
	})

	_, err := ioutil.ReadFile(path.Join(dir, "name"))
	if !os.IsNotExist(err) {
		t.Fatalf(`expected to get an error that the file doesn't exist, got %s`, err)
	}
}

func TestTagsFs_Open_error(t *testing.T) {
	client, dir, cleanup := setup(t)
	defer cleanup()

	numReqs := 0
	client.Handlers.Send.PushBack(serveTags(map[string]string{"name": "MyName"}))
	client.Handlers.Send.PushBack(func(r *request.Request) {
		if numReqs == 0 { // Allow GetAttr call to succeed
			numReqs++
			return
		}

		r.Error = awserr.NewRequestFailure(
			awserr.New("UnknownError", "500 Internal Server Error", fmt.Errorf("mock error")), 500, "",
		)
	})

	_, err := ioutil.ReadFile(path.Join(dir, "name"))
	if err.(*os.PathError).Err != syscall.EIO {
		t.Fatalf(`expected EIO, got %s`, err)
	}
}

func TestTagsFs_ReadOnly(t *testing.T) {
	client, dir, cleanup := setup(t)
	defer cleanup()

	client.Handlers.Send.PushBack(serveTags(map[string]string{"name": "MyName"}))

	err := ioutil.WriteFile(path.Join(dir, "name"), []byte("hello world"), os.ModePerm)
	if !os.IsPermission(err) {
		t.Fatalf(`expected to get permissions error, got %s`, err)
	}

	err = os.Chown(path.Join(dir, "name"), 0, 0)
	if !os.IsPermission(err) {
		t.Fatalf(`expected to get permissions error, got %s`, err)
	}
}
