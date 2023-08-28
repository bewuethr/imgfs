package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	iofs "io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	_ "golang.org/x/image/webp"
)

var progName = filepath.Base(os.Args[0])

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", progName)
	fmt.Fprintf(os.Stderr, "  %s IMAGE MOUNTPOINT\n", progName)
	flag.PrintDefaults()
}

func main() {
	log.SetFlags(0)
	log.SetPrefix(progName + ": ")

	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 2 {
		usage()
		os.Exit(2)
	}

	path := flag.Arg(0)
	mountpoint := flag.Arg(1)

	if err := mount(path, mountpoint); err != nil {
		log.Fatal(err)
	}
}

func mount(path, mountpoint string) error {
	reader, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening %v: %w", path, err)
	}
	defer reader.Close() //nolint:errcheck

	img, _, err := image.Decode(reader)
	if err != nil {
		return fmt.Errorf("decoding: %w", err)
	}

	c, err := fuse.Mount(mountpoint)
	if err != nil {
		return err
	}
	defer c.Close() //nolint:errcheck

	filesys := &FS{img: img}

	if os.Getenv("IMGFS_DEBUG") != "" {
		fuse.Debug = func(msg any) {
			log.Printf("FUSE: %s\n", msg)
		}
	}

	return fs.Serve(c, filesys)
}

type FS struct {
	img image.Image
}

func (f *FS) Root() (fs.Node, error) {
	return &Dir{
		img:     f.img,
		dirType: root,
	}, nil
}

type dirType int

const (
	root = dirType(iota + 1)
	row
	pixel
)

type Dir struct {
	img     image.Image
	dirType dirType
	value   []int // [y] if row, [y, x] if column
}

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = iofs.ModeDir | 0755
	return nil
}

func (d *Dir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
	switch d.dirType {
	case root:
		if strings.HasPrefix(req.Name, "row") {
			rowNum, err := strconv.Atoi(strings.TrimPrefix(req.Name, "row"))
			if err != nil {
				return nil, syscall.ENOENT
			}

			if rowNum > d.img.Bounds().Max.Y {
				return nil, syscall.ENOENT
			}

			return &Dir{
				img:     d.img,
				dirType: row,
				value:   []int{rowNum},
			}, nil
		}

	case row:
		if strings.HasPrefix(req.Name, "col") {
			colNum, err := strconv.Atoi(strings.TrimPrefix(req.Name, "col"))
			if err != nil {
				return nil, syscall.ENOENT
			}

			if colNum > d.img.Bounds().Max.X {
				return nil, syscall.ENOENT
			}

			return &Dir{
				img:     d.img,
				dirType: pixel,
				value:   append(d.value, colNum),
			}, nil
		}

	case pixel:
		switch req.Name {
		case "r", "g", "b":
			return &File{
				img:  d.img,
				name: req.Name,
				x:    d.value[1],
				y:    d.value[0],
			}, nil
		}
	}

	return nil, syscall.ENOENT
}

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var res []fuse.Dirent

	switch d.dirType {
	case root:
		yMax := d.img.Bounds().Max.Y
		lenYMax := len(strconv.Itoa(yMax))

		for y := d.img.Bounds().Min.Y; y < yMax; y++ {
			res = append(res, fuse.Dirent{
				Name: "row" + fmt.Sprintf("%0*d", lenYMax, y),
				Type: fuse.DT_Dir,
			})
		}

	case row:
		xMax := d.img.Bounds().Max.X
		lenXMax := len(strconv.Itoa(xMax))

		for x := d.img.Bounds().Min.X; x < xMax; x++ {
			res = append(res, fuse.Dirent{
				Name: "col" + fmt.Sprintf("%0*d", lenXMax, x),
				Type: fuse.DT_Dir,
			})
		}

	case pixel:
		res = []fuse.Dirent{
			{Name: "r", Type: fuse.DT_File},
			{Name: "g", Type: fuse.DT_File},
			{Name: "b", Type: fuse.DT_File},
		}
	}

	return res, nil
}

type File struct {
	img  image.Image
	name string
	x, y int
}

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	colStr, err := f.getContents()
	if err != nil {
		return err
	}

	a.Size = uint64(len(colStr))
	a.Mode = 0644

	return nil
}

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	colStr, err := f.getContents()
	if err != nil {
		return nil, fmt.Errorf("getting color for file: %w", err)
	}

	return &FileHandle{r: io.NopCloser(strings.NewReader(colStr))}, nil
}

func (f *File) getContents() (string, error) {
	r, g, b, _ := f.img.At(f.x, f.y).RGBA()

	var val uint32

	switch f.name {
	case "r":
		val = r
	case "g":
		val = g
	case "b":
		val = b
	default:
		return "", fmt.Errorf("invalid filename %q", f.name)
	}

	return strconv.FormatUint(uint64(val>>8), 10) + "\n", nil
}

type FileHandle struct {
	r io.ReadCloser
}

func (fh *FileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	return fh.r.Close()
}

func (fh *FileHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	val := make([]byte, req.Size)
	_, err := fh.r.Read(val)

	resp.Data = val

	return err
}
