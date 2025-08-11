package dque

import (
	"errors"
	"io"
	"os"

	"github.com/ncw/directio"
)

// DirectIOReader is a reader that reads data from the underlying reader using direct I/O.
type DirectIOReader struct {
	buf     []byte
	bufSize int
	off     int
	rd      io.Reader
}

var _ io.ByteReader = (*DirectIOReader)(nil)

var _ io.Reader = (*DirectIOReader)(nil)

func NewDirectIOReader(rd io.Reader) *DirectIOReader {
	return &DirectIOReader{
		buf: directio.AlignedBlock(directio.BlockSize),
		rd:  rd,
	}
}

func (d *DirectIOReader) Read(p []byte) (int, error) {
	// read data from the underlying reader if the buffer is empty
	if d.off == d.bufSize {
		var err error
		d.bufSize, err = io.ReadFull(d.rd, d.buf)
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			return 0, err
		}
		d.off = 0
	}
	// copy data to the buffer
	n := copy(p, d.buf[d.off:])
	d.off += n
	return n, nil
}

func (d *DirectIOReader) ReadByte() (byte, error) {
	panic("ReadByte not implemented, gob.Decode should not be using this")
}

// DirectIOWriter is a writer that writes data to the underlying writer using direct I/O.
type DirectIOWriter struct {
	buf      []byte
	fileSize int64
	off      int
	wr       *os.File
}

var _ io.Writer = (*DirectIOWriter)(nil)

func NewDirectIOWriter(wr *os.File) *DirectIOWriter {
	return &DirectIOWriter{
		buf: directio.AlignedBlock(directio.BlockSize),
		wr:  wr,
	}
}

func (d *DirectIOWriter) Close() error {
	// flush the buffered data
	_, err := d.wr.Write(d.buf)
	if err != nil {
		return err
	}
	// adjust file size
	err = d.wr.Truncate(d.fileSize)
	if err != nil {
		return err
	}
	return d.wr.Close()
}

func (d *DirectIOWriter) Write(p []byte) (int, error) {
	written := 0
	for written < len(p) {
		// copy data to the buffer
		var n int
		if len(p)-written > directio.BlockSize-d.off {
			// too big, copy only the fitting data to the buffer
			n = copy(d.buf[d.off:], p[written:written+directio.BlockSize-d.off])
			// write data to the underlying writer
			_, err := d.wr.Write(d.buf)
			if err != nil {
				return written, err
			}
			d.off = 0
		} else {
			n = copy(d.buf[d.off:], p[written:])
			d.off += n
		}
		written += n
	}
	d.fileSize += int64(written)
	return written, nil
}

func (d *DirectIOWriter) Sync() error {
	// direct IO only allows writing directio.BlockSize data, which is already done by Write()
	// reads and writes must be performed in a forward, sequential manner
	// we cannot flush buf to disk, which could lead to file corruption
	return d.wr.Sync()
}
