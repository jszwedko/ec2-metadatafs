package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

func setup(t *testing.T) (mux *http.ServeMux, workdir string, cleanup func()) {
	mux = http.NewServeMux()
	server := httptest.NewServer(mux)

	tmpDir, err := ioutil.TempDir("", "ec2metadata-test")
	if err != nil {
		t.Fatalf("creating tempdir failed: %v", err)
	}

	fs := NewMetadataFs(server.URL + "/")
	nfs := pathfs.NewPathNodeFs(fs, nil)
	state, _, err := nodefs.MountRoot(tmpDir, nfs.Root(), nodefs.NewOptions())
	if err != nil {
		t.Fatalf("mounting filesystem failed: %v", err)
	}

	go state.Serve()

	return mux, tmpDir, func() {
		server.Close()
		state.Unmount()
		os.RemoveAll(tmpDir)
	}
}

func setupWithoutServer(t *testing.T) (workdir string, cleanup func()) {
	tmpDir, err := ioutil.TempDir("", "ec2metadata-test")
	if err != nil {
		t.Fatalf("creating tempdir failed: %v", err)
	}

	fs := NewMetadataFs("")
	nfs := pathfs.NewPathNodeFs(fs, nil)
	state, _, err := nodefs.MountRoot(tmpDir, nfs.Root(), nodefs.NewOptions())
	if err != nil {
		t.Fatalf("mounting filesystem failed: %v", err)
	}

	go state.Serve()

	return tmpDir, func() {
		state.Unmount()
		os.RemoveAll(tmpDir)
	}
}

func serveFile(mux *http.ServeMux, file string, body string, modified time.Time) {
	dir, filename := path.Split(file)
	serveDirectory(mux, dir, []string{filename}, modified)

	mux.HandleFunc(file, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Length", strconv.Itoa(len(body)))
		w.Header().Add("Last-Modified", modified.Format(time.RFC1123))
		if r.Method == "GET" {
			fmt.Fprint(w, body)
		}
	})
}

func serveDirectory(mux *http.ServeMux, dir string, entries []string, modified time.Time) {
	if dir != "/" {
		dir = strings.TrimRight(dir, "/")
	}

	mux.HandleFunc(dir, func(w http.ResponseWriter, r *http.Request) {
		directories := strings.Join(entries, "\n")
		w.Header().Add("Content-Length", strconv.Itoa(len(directories)))
		w.Header().Add("Last-Modified", modified.Format(time.RFC1123))
		if r.Method == "GET" {
			fmt.Fprint(w, directories)
		}
	})

	if dir == "/" {
		return
	}

	parent, file := path.Split(dir)
	if parent != "" {
		serveDirectory(mux, parent, []string{file}, modified)
	}
}

func TestMetadatFs_GetAttr_regularFile(t *testing.T) {
	mux, dir, cleanup := setup(t)
	defer cleanup()

	modified := time.Now().Truncate(time.Second)
	serveFile(mux, "/meta-data/instance-id", "i-123456", modified)

	info, err := os.Stat(path.Join(dir, "meta-data/instance-id"))
	if err != nil {
		t.Fatalf(`error retrieving stat %s`, err)
	}
	if !info.Mode().IsRegular() {
		t.Errorf(`expected regular file`)
	}
	if info.Size() != 8 {
		t.Errorf(`file size was %d, expected %d`, info.Size(), 8)
	}
	if !info.ModTime().Equal(modified) {
		t.Errorf(`modtime was %s, expected %s`, info.ModTime(), modified)
	}
}

func TestMetadatFs_GetAttr_badLastModified(t *testing.T) {
	mux, dir, cleanup := setup(t)
	defer cleanup()

	mux.HandleFunc("/meta-data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Last-Modified", "foobar")
	})

	info, err := os.Stat(path.Join(dir, "meta-data"))
	if err != nil {
		t.Fatalf(`error retrieving stat %s`, err)
	}
	if !info.ModTime().IsZero() {
		t.Errorf(`modtime was %s, expected the zero time`, info.ModTime())
	}
}

func TestMetadatFs_GetAttr_directory(t *testing.T) {
	mux, dir, cleanup := setup(t)
	defer cleanup()

	modified := time.Now().Truncate(time.Second)
	serveDirectory(mux, "/meta-data", []string{"ami-id", "block-device-mapping/"}, modified)

	info, err := os.Stat(path.Join(dir, "meta-data"))
	if err != nil {
		t.Fatalf(`error retrieving stat %s`, err)
	}
	if !info.Mode().IsDir() {
		t.Errorf(`expected directory`)
	}
	if info.Size() != 4096 {
		t.Errorf(`file size was %d, expected %d`, info.Size(), 8)
	}
	if !info.ModTime().Equal(modified) {
		t.Errorf(`modtime was %s, expected %s`, info.ModTime(), modified)
	}
}

func TestMetadatFs_GetAttr_noFile(t *testing.T) {
	_, dir, cleanup := setup(t)
	defer cleanup()

	_, err := os.Stat(path.Join(dir, "meta-data/foobar"))
	if !os.IsNotExist(err) {
		t.Fatalf(`expected to get an error that the file doesn't exist, got %s`, err)
	}
}

func TestMetadatFs_GetAttr_badResponse(t *testing.T) {
	mux, dir, cleanup := setup(t)
	defer cleanup()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	})

	_, err := os.Stat(path.Join(dir, "/"))
	if err.(*os.PathError).Err != syscall.EIO {
		t.Fatalf(`expected EIO, got %s`, err)
	}
}

func TestMetadatFs_GetAttr_noServer(t *testing.T) {
	dir, cleanup := setupWithoutServer(t)
	defer cleanup()

	_, err := os.Stat(path.Join(dir, "/"))
	if err.(*os.PathError).Err != syscall.EIO {
		t.Fatalf(`expected EIO, got %s`, err)
	}
}

func TestMetadatFs_OpenDir(t *testing.T) {
	mux, dir, cleanup := setup(t)
	defer cleanup()

	serveDirectory(mux, "/meta-data", []string{"ami-id", "", "block-device-mapping/"}, time.Now())

	fileInfos, err := ioutil.ReadDir(path.Join(dir, "meta-data"))
	if err != nil {
		t.Fatalf(`error listing directory: %s`, err)
	}

	names := make([]string, len(fileInfos))
	for i, fileInfo := range fileInfos {
		names[i] = fileInfo.Name()

		if fileInfo.Name() == "ami-id" && !fileInfo.Mode().IsRegular() {
			t.Errorf(`returned that ami-id was not a regular file`)
		}
		if fileInfo.Name() == "block-device-mapping" && !fileInfo.Mode().IsDir() {
			t.Errorf(`returned that block-device-mapping was not a directory`)
		}
	}

	if !reflect.DeepEqual([]string{"ami-id", "block-device-mapping"}, names) {
		t.Errorf(`returned entries %+v, expected %+v`, names, []string{"ami-id", "block-device-mapping"})
	}
}

// Test edge case where meta-data/public-keys returns differently formatted HTTP response
func TestMetadatFs_OpenDir_publicKeys(t *testing.T) {
	mux, dir, cleanup := setup(t)
	defer cleanup()

	serveDirectory(mux, "/meta-data/public-keys", []string{"0=id_rsa"}, time.Now())

	fileInfos, err := ioutil.ReadDir(path.Join(dir, "meta-data/public-keys"))
	if err != nil {
		t.Fatalf(`error listing directory: %s`, err)
	}

	if len(fileInfos) != 1 {
		t.Errorf(`expected 1 file in response, got %d`, len(fileInfos))
	}

	if fileInfos[0].Name() != "0" {
		t.Errorf(`expected there to be one entry, 0, got %s`, fileInfos[0].Name())
	}
}

func TestMetadatFs_OpenDir_notDir(t *testing.T) {
	mux, dir, cleanup := setup(t)
	defer cleanup()

	serveFile(mux, "/meta-data/instance-id", "i-123456", time.Now())

	_, err := ioutil.ReadDir(path.Join(dir, "meta-data/instance-id"))
	if err.(*os.SyscallError).Err != syscall.ENOTDIR {
		t.Fatalf(`expected ENOTDIR, got %s`, err)
	}
}

func TestMetadatFs_OpenDir_noFile(t *testing.T) {
	mux, dir, cleanup := setup(t)
	defer cleanup()

	mux.HandleFunc("/meta-data", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			w.WriteHeader(404)
		}
	})

	_, err := ioutil.ReadDir(path.Join(dir, "meta-data"))
	if !os.IsNotExist(err) {
		t.Fatalf(`expected to get an error that the directory doesn't exist, got %s`, err)
	}
}

func TestMetadatFs_OpenDir_badResponse(t *testing.T) {
	mux, dir, cleanup := setup(t)
	defer cleanup()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			w.WriteHeader(500)
		}
	})

	_, err := ioutil.ReadDir(path.Join(dir, "/"))
	if err.(*os.PathError).Err != syscall.EIO {
		t.Fatalf(`expected EIO, got %s`, err)
	}
}

func TestMetadatFs_OpenDir_noServer(t *testing.T) {
	mux, dir, cleanup := setup(t)
	defer cleanup()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Add("Content-Length", "0")
		} else {
			panic("cannot read")
		}
	})

	_, err := ioutil.ReadDir(path.Join(dir, "/"))
	if err.(*os.PathError).Err != syscall.EIO {
		t.Fatalf(`expected EIO, got %s`, err)
	}
}

func TestMetadatFs_Open(t *testing.T) {
	mux, dir, cleanup := setup(t)
	defer cleanup()

	serveFile(mux, "/meta-data/instance-id", "i-123456", time.Now())

	contents, err := ioutil.ReadFile(path.Join(dir, "/meta-data/instance-id"))
	if err != nil {
		t.Fatalf(`error reading file: %s`, err)
	}

	if string(contents) != "i-123456" {
		t.Fatalf(`contents were %s, expected %s`, string(contents), "i-123456")
	}
}

func TestMetadatFs_Open_noFile(t *testing.T) {
	mux, dir, cleanup := setup(t)
	defer cleanup()

	mux.HandleFunc("/user-data", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			w.WriteHeader(404)
		}
	})

	_, err := ioutil.ReadFile(path.Join(dir, "user-data"))
	if !os.IsNotExist(err) {
		t.Fatalf(`expected to get an error that the file doesn't exist, got %s`, err)
	}
}

func TestMetadatFs_Open_badResponse(t *testing.T) {
	mux, dir, cleanup := setup(t)
	defer cleanup()

	mux.HandleFunc("/user-data", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			w.WriteHeader(500)
		}
	})

	_, err := ioutil.ReadFile(path.Join(dir, "user-data"))
	if err.(*os.PathError).Err != syscall.EIO {
		t.Fatalf(`expected EIO, got %s`, err)
	}
}

func TestMetadatFs_Open_noServer(t *testing.T) {
	mux, dir, cleanup := setup(t)
	defer cleanup()

	mux.HandleFunc("/user-data", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Add("Content-Length", "0")
		} else {
			panic("cannot read")
		}
	})

	_, err := ioutil.ReadFile(path.Join(dir, "user-data"))
	if err.(*os.PathError).Err != syscall.EIO {
		t.Fatalf(`expected EIO, got %s`, err)
	}
}

func TestMetadatFs_ReadOnly(t *testing.T) {
	mux, dir, cleanup := setup(t)
	defer cleanup()

	modified := time.Now().Truncate(time.Second)
	serveFile(mux, "/meta-data/instance-id", "i-123456", modified)

	err := ioutil.WriteFile(path.Join(dir, "/meta-data/instance-id"), []byte("hello world"), os.ModePerm)
	if !os.IsPermission(err) {
		t.Fatalf(`expected to get permissions error, got %s`, err)
	}

	err = os.Chown(path.Join(dir, "/meta-data/instance-id"), 0, 0)
	if !os.IsPermission(err) {
		t.Fatalf(`expected to get permissions error, got %s`, err)
	}
}
