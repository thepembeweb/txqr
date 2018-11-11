package protocol

import (
	"fmt"
	"strings"
)

// Decoder represents protocol decode.
type Decoder struct {
	buffer   []byte
	complete bool
	total    int
	frames   []frameInfo
	cache    map[string]struct{}
	progress int
}

// frameInfo represents the information about read frames.
// As frames can change size dynamically, we keep size info as well.
type frameInfo struct {
	offset, size int
}

// NewDecoder creats and inits a new decoder.
func NewDecoder() *Decoder {
	return &Decoder{
		buffer: []byte{},
		cache:  make(map[string]struct{}),
	}
}

// NewDecoderSize creats and inits a new decoder for the known size.
// Note, it doesn't limit the size of the input, but optimizes memory allocation.
func NewDecoderSize(size int) *Decoder {
	return &Decoder{
		buffer: make([]byte, size),
	}
}

// DecodeChunk takes a single chunk of data and decodes it.
func (d *Decoder) DecodeChunk(data string) error {
	if data == "" || len(data) < 4 {
		return fmt.Errorf("invalid frame: \"%s\"", data)
	}

	idx := strings.IndexByte(data, '|')
	if idx == -1 {
		return fmt.Errorf("invalid frame: \"%s\"", data)
	}

	header := data[:idx]
	// continuous QR reading often sends the same chunk in a row, skip it
	if d.isCached(header) {
		return nil
	}

	var offset, total int
	_, err := fmt.Sscanf(header, "%d/%d", &offset, &total)
	if err != nil {
		return fmt.Errorf("invalid header: %v (%s)", err, header)
	}

	// allocate enough memory at first total read
	if d.total == 0 {
		d.buffer = make([]byte, total)
		d.total = total
	}

	if total > d.total {
		return fmt.Errorf("total changed during sequence, aborting")
	}

	payload := data[idx+1:]
	size := len(payload)
	// TODO(divan): optmize memory allocation
	d.frames = append(d.frames, frameInfo{offset: offset, size: size})

	copy(d.buffer[offset:offset+size], payload)

	d.updateProgress()

	return nil
}

// Data returns decoded data.
func (d *Decoder) Data() string {
	return string(d.buffer)
}

// DataBytes returns decoded data as a byte slice.
func (d *Decoder) DataBytes() []byte {
	return d.buffer
}

// Progress returns reading progress in percentage.
func (d *Decoder) Progress() int {
	return d.progress
}

// IsCompleted reports whether the read was completed successfully or not.
func (d *Decoder) IsCompleted() bool {
	return d.complete
}

// updateProgress updates progress and complete state of reading.
// FIXME(divan): this approach might give false negatives in extreme cases, like
// many dynamic changes of chunk sizes.
func (d *Decoder) updateProgress() {
	var cur int
	for _, frame := range d.frames {
		cur += frame.size
	}

	d.progress = 100 * cur / d.total
	d.complete = cur == d.total
}

// isCached takes the header of chunk data and see if it's been cached.
// If not, it caches it.
func (d *Decoder) isCached(header string) bool {
	if _, ok := d.cache[header]; ok {
		return true
	}

	// cache it
	d.cache[header] = struct{}{}
	return false
}
