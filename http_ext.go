package wildcat

import (
	"bytes"

	bs "github.com/panjf2000/gnet/v2/pkg/bs"
	bsPool "github.com/panjf2000/gnet/v2/pkg/pool/byteslice"
	"github.com/vektra/errors"
)

// @Title
// @Description
// @Author
// @Update

var (
	DefaultInputCap int
)

func NewHTTPParserExt() *HTTPParser {
	parser := &HTTPParser{
		TotalHeaders:  DefaultHeaderSlice,
		contentLength: -1,
		inputCopy:     make([]byte, DefaultInputCap),
	}
	parser.initHeaders()
	return parser
}

func (hp *HTTPParser) Reset() {
	hp.Method = nil
	hp.Path = nil
	hp.Version = nil
	hp.host = nil
	// fmt.Println("wildcat HTTPParser Release headersSize:", hp.headersSize)
	for i := range hp.headersSize {
		header := hp.Headers[i]
		// fmt.Println(fmt.Sprintf("wildcat HTTPParser Release idx:%d,%d release:%v header:%s", hp.headersSize, i, header.needRelease, header.Name))
		if header.needRelease {
			header.needRelease = false
			bsPool.Put(header.Value)
		}
		header.Name = nil
		header.Value = nil
	}
	hp.headersSize = 0
}

func (hp *HTTPParser) initHeaders() {
	hp.Headers = make([]header, hp.TotalHeaders)
	for i := range hp.TotalHeaders {
		hp.Headers[i] = header{}
	}
}

func (hp *HTTPParser) growHeaders() {
	hp.Headers = append(hp.Headers, header{}, header{}, header{}, header{}, header{}, header{}, header{}, header{}, header{}, header{})
	hp.TotalHeaders += 10
}

func (hp *HTTPParser) FindHeaderExt(name string) []byte {
	for i := range hp.headersSize {
		header := hp.Headers[i]
		if bs.BytesToString(header.Name) == name || bytes.EqualFold(header.Name, bs.StringToBytes(name)) {
			return header.Value
		}
	}
	return nil
}

func (hp *HTTPParser) parseInput(input []byte) (int, error) {
	var headers int
	var path int
	var ok bool

	total := len(input)

method:
	for i := range total {
		switch input[i] {
		case ' ', '\t':
			hp.Method = input[0:i]
			ok = true
			path = i + 1
			break method
		}
	}

	if !ok {
		return 0, ErrMissingData
	}

	var version int

	ok = false

path:
	for i := path; i < total; i++ {
		switch input[i] {
		case ' ', '\t':
			ok = true
			hp.Path = input[path:i]
			version = i + 1
			break path
		}
	}

	if !ok {
		return 0, ErrMissingData
	}

	var readN bool

	ok = false
loop:
	for i := version; i < total; i++ {
		c := input[i]

		switch readN {
		case false:
			switch c {
			case '\r':
				hp.Version = input[version:i]
				readN = true
			case '\n':
				hp.Version = input[version:i]
				headers = i + 1
				ok = true
				break loop
			}
		case true:
			if c != '\n' {
				return 0, errors.Context(ErrBadProto, "missing newline in version")
			}
			headers = i + 1
			ok = true
			break loop
		}
	}

	if !ok {
		return 0, ErrMissingData
	}

	var headerName []byte

	state := eNextHeader

	start := headers

	for i := headers; i < total; i++ {
		switch state {
		case eNextHeader:
			switch input[i] {
			case '\r':
				state = eNextHeaderN
			case '\n':
				return i + 1, nil
			case ' ', '\t':
				state = eMLHeaderStart
			default:
				start = i
				state = eHeader
			}
		case eNextHeaderN:
			if input[i] != '\n' {
				return 0, ErrBadProto
			}

			return i + 1, nil
		case eHeader:
			if input[i] == ':' {
				headerName = input[start:i]
				state = eHeaderValueSpace
			}
		case eHeaderValueSpace:
			switch input[i] {
			case ' ', '\t':
				continue
			}

			start = i
			state = eHeaderValue
		case eHeaderValue:
			switch input[i] {
			case '\r':
				state = eHeaderValueN
			case '\n':
				state = eNextHeader
			default:
				continue
			}

			hp.Headers[hp.headersSize].Name, hp.Headers[hp.headersSize].Value = headerName, input[start:i]
			hp.headersSize++

			if hp.headersSize == hp.TotalHeaders {
				hp.growHeaders()
			}
		case eHeaderValueN:
			if input[i] != '\n' {
				return 0, ErrBadProto
			}
			state = eNextHeader

		case eMLHeaderStart:
			switch input[i] {
			case ' ', '\t':
				continue
			}

			start = i
			state = eMLHeaderValue
		case eMLHeaderValue:
			switch input[i] {
			case '\r':
				state = eHeaderValueN
			case '\n':
				state = eNextHeader
			default:
				continue
			}

			cur := hp.Headers[hp.headersSize-1].Value

			newheader := bsPool.Get(len(cur) + 1 + (i - start))
			copy(newheader, cur)
			copy(newheader[len(cur):], []byte(" "))
			copy(newheader[len(cur)+1:], input[start:i])

			hp.Headers[hp.headersSize-1].Value = newheader
			hp.Headers[hp.headersSize-1].needRelease = true
		}
	}

	return 0, ErrMissingData
}

func (hp *HTTPParser) ParseExt(input []byte) (int, error) {
	if len(input) <= 0 {
		return 0, ErrMissingData
	}
	copy(hp.inputCopy[0:], input)
	return hp.parseInput(hp.inputCopy[:len(input)])
}
