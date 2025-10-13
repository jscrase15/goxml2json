package xml2json

import (
	"bytes"
	"io"
)

// Convert converts the given XML document to JSON
func Convert(r io.Reader, ps ...plugin) (*bytes.Buffer, error) {
	// Decode XML document
	root := &Node{}

	var err error
	d := NewDecoder(r, ps...)
	if d.useTokenRaw {
		err = d.DecodeRaw(root)
	} else {
		err = d.Decode(root)
	}
	if err != nil {
		return nil, err
	}

	// Then encode it in JSON
	buf := new(bytes.Buffer)
	e := NewEncoder(buf, ps...)
	err = e.Encode(root)
	if err != nil {
		return nil, err
	}

	return buf, nil
}
