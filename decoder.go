package xml2json

import (
	"encoding/xml"
	"io"
	"strconv"
	"unicode"

	"golang.org/x/net/html/charset"
)

const (
	attrPrefix     = "-"
	contentPrefix  = "#"
	sequencePrefix = "^"
)

// A Decoder reads and decodes XML objects from an input stream.
type Decoder struct {
	r                  io.Reader
	err                error
	attributePrefix    string
	contentPrefix      string
	sequencePrefix     string
	nsPrefix           string
	includeNSPrefix    bool
	useTokenRaw        bool
	excludeAttrs       map[string]bool
	formatters         []nodeFormatter
	NameSpacePrefix    map[string]string
	levelSequence      []int
	includeXMLSequence bool
}

type element struct {
	parent   *element
	nsPrefix string
	n        *Node
	label    string
	sequence int
}

func (dec *Decoder) SetSequencePrefix(prefix string) {
	dec.sequencePrefix = prefix
}

func (dec *Decoder) IncludeXMLSequence(v bool) {
	dec.includeXMLSequence = v
}

func (dec *Decoder) SetAttributePrefix(prefix string) {
	dec.attributePrefix = prefix
}

func (dec *Decoder) SetContentPrefix(prefix string) {
	dec.contentPrefix = prefix
}

func (dec *Decoder) AddFormatters(formatters []nodeFormatter) {
	dec.formatters = formatters
}

func (dec *Decoder) SetIncludePrefix(prefix string) {
	dec.includeNSPrefix = true
	dec.nsPrefix = prefix
}

func (dec *Decoder) ExcludeAttributes(attrs []string) {
	for _, attr := range attrs {
		dec.excludeAttrs[attr] = true
	}
}

func (dec *Decoder) DecodeWithCustomPrefixes(root *Node, contentPrefix string, attributePrefix string) error {
	dec.contentPrefix = contentPrefix
	dec.attributePrefix = attributePrefix
	return dec.Decode(root)
}

// NewDecoder returns a new decoder that reads from r.
func NewDecoder(r io.Reader, plugins ...plugin) *Decoder {
	d := &Decoder{r: r, sequencePrefix: sequencePrefix, contentPrefix: contentPrefix, attributePrefix: attrPrefix, excludeAttrs: map[string]bool{}, NameSpacePrefix: map[string]string{}}
	for _, p := range plugins {
		d = p.AddToDecoder(d)
	}
	return d
}

// Decode reads the next JSON-encoded value from its
// input and stores it in the value pointed to by v.
func (dec *Decoder) Decode(root *Node) error {
	xmlDec := xml.NewDecoder(dec.r)

	// That will convert the charset if the provided XML is non-UTF-8
	xmlDec.CharsetReader = charset.NewReaderLabel

	// Create first element from the root node
	elem := &element{
		parent: nil,
		n:      root,
	}
	dec.levelSequence = append(dec.levelSequence, 1)

	for {
		t, _ := xmlDec.Token()
		if t == nil {
			break
		}
		switch se := t.(type) {
		case xml.StartElement:
			// Build new a new current element and link it to its parent
			elem = &element{
				parent:   elem,
				n:        &Node{},
				label:    se.Name.Local,
				sequence: dec.levelSequence[len(dec.levelSequence)-1],
			}
			dec.levelSequence[len(dec.levelSequence)-1]++
			dec.levelSequence = append(dec.levelSequence, 1)

			// Extract attributes as children
			for _, a := range se.Attr {
				// add prefix (local name) to store under the Namespace key
				if a.Name.Space == "xmlns" && a.Name.Local != "" {
					dec.NameSpacePrefix[a.Value] = a.Name.Local
				}

				if _, ok := dec.excludeAttrs[a.Name.Local]; ok {
					continue
				}
				elem.n.AddChild(dec.attributePrefix+a.Name.Local, &Node{Data: a.Value})
			}

			if se.Name.Space != "" && dec.includeNSPrefix {
				prefix, exists := dec.NameSpacePrefix[se.Name.Space]
				if exists {
					elem.label = prefix + ":" + se.Name.Local
				}
			}

			if dec.includeXMLSequence {
				elem.n.AddChild(dec.sequencePrefix+"sequence", &Node{Data: strconv.Itoa(elem.sequence)})
			}
		case xml.CharData:
			// Extract XML data (if any)
			elem.n.Data = trimNonGraphic(string(xml.CharData(se)))
		case xml.EndElement:
			dec.levelSequence = dec.levelSequence[:len(dec.levelSequence)-1]
			// And add it to its parent list
			if elem.parent != nil {
				elem.parent.n.AddChild(elem.label, elem.n)
			}

			// Then change the current element to its parent
			elem = elem.parent
		}
	}

	for _, formatter := range dec.formatters {
		formatter.Format(root)
	}

	return nil
}

// DecodeRaw acts the same as Decode, but uses Decoder.RawToken instead.
// RawToken does not verify that start and end elements match, and,
// crucially, does not translate namespace prefixes to their corresponding URL.
// This is good for including the prefix in the resulting JSON.
func (dec *Decoder) DecodeRaw(root *Node) error {
	xmlDec := xml.NewDecoder(dec.r)

	// That will convert the charset if the provided XML is non-UTF-8
	xmlDec.CharsetReader = charset.NewReaderLabel

	// Create first element from the root node
	elem := &element{
		parent: nil,
		n:      root,
	}
	dec.levelSequence = append(dec.levelSequence, 1)

	for {
		t, _ := xmlDec.RawToken()
		if t == nil {
			break
		}
		switch se := t.(type) {
		case xml.StartElement:
			// Build new a new current element and link it to its parent
			elem = &element{
				parent:   elem,
				n:        &Node{},
				label:    se.Name.Local,
				sequence: dec.levelSequence[len(dec.levelSequence)-1],
			}
			dec.levelSequence[len(dec.levelSequence)-1]++
			dec.levelSequence = append(dec.levelSequence, 1)

			// Extract attributes as children
			for _, a := range se.Attr {
				if _, ok := dec.excludeAttrs[a.Name.Local]; ok {
					continue
				}
				elem.n.AddChild(dec.attributePrefix+a.Name.Local, &Node{Data: a.Value})
			}

			if se.Name.Space != "" && dec.includeNSPrefix {
				elem.label = se.Name.Space + ":" + se.Name.Local
			}

			if dec.includeXMLSequence {
				elem.n.AddChild(dec.sequencePrefix+"sequence", &Node{Data: strconv.Itoa(elem.sequence)})
			}
		case xml.CharData:
			// Extract XML data (if any)
			elem.n.Data = trimNonGraphic(string(xml.CharData(se)))
		case xml.EndElement:
			dec.levelSequence = dec.levelSequence[:len(dec.levelSequence)-1]
			// And add it to its parent list
			if elem.parent != nil {
				elem.parent.n.AddChild(elem.label, elem.n)
			}

			// Then change the current element to its parent
			elem = elem.parent
		}
	}

	for _, formatter := range dec.formatters {
		formatter.Format(root)
	}

	return nil
}

// trimNonGraphic returns a slice of the string s, with all leading and trailing
// non graphic characters and spaces removed.
//
// Graphic characters include letters, marks, numbers, punctuation, symbols,
// and spaces, from categories L, M, N, P, S, Zs.
// Spacing characters are set by category Z and property Pattern_White_Space.
func trimNonGraphic(s string) string {
	if s == "" {
		return s
	}

	var first *int
	var last int
	for i, r := range []rune(s) {
		if !unicode.IsGraphic(r) || unicode.IsSpace(r) {
			continue
		}

		if first == nil {
			f := i // copy i
			first = &f
			last = i
		} else {
			last = i
		}
	}

	// If first is nil, it means there are no graphic characters
	if first == nil {
		return ""
	}

	return string([]rune(s)[*first : last+1])
}
