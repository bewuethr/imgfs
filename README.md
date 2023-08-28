# imgfs

imgfs lets you mount an image as a [FUSE] userspace file system. It uses
[bazil.org/fuse][bazil], a pure-Go implementation of FUSE.

[bazil]: <https://github.com/bazil/fuse>

## Usage

Install with Go:

```sh
go install github.com/bewuethr/imgfs@latest
```

Create a directory as the mount target, and then mount an image file:

```sh
mkdir mnt
imgfs image.png mnt &
```

Supported image formats are PNG, JPEG, GIF, and WebP.

Navigate the directory tree representing the image; in the root directory,
directories represent rows of pixels:

```console
$ tree -L 1 mnt
mnt
├── row0
├── row1
└── row2
```

Within a pixel row, directories correspond to pixels ("columns"):

```console
$ tree -L 1 mnt/row0
mnt/row0
├── col0
├── col1
└── col2
```

And within a pixel directory, there are files containing RGB values of the
pixel:

```console
$ tree -L 1 mnt/row0/col0
mnt/row0/col0
├── b
├── g
└── r
```

Individual values are scaled to 0..255:

```console
$ head mnt/row0/col0/*
==> mnt/row0/col0/b <==
0

==> mnt/row0/col0/g <==
0

==> mnt/row0/col0/r <==
255
```

So the pixel at (0,0) has RGB values of (255,0,0), i.e., is full red.

## Limitations

- The file system is read-only
- Probably a bunch of bugs

[fuse]: <https://en.wikipedia.org/wiki/Filesystem_in_Userspace>