package wildcat

// @Title
// @Description
// @Author
// @Update

import (
	"sync"

	bsPool "github.com/panjf2000/gnet/v2/pkg/pool/byteslice"
)

var (
	parserPool = &sync.Pool{New: func() any {
		parser := &HTTPParser{
			TotalHeaders:  DefaultHeaderSlice,
			contentLength: -1,
		}
		parser.initHeaders()
		return parser
	}}
)

func NewHTTPParserUsePool() *HTTPParser {
	return parserPool.Get().(*HTTPParser)
}

func (hp *HTTPParser) Release() {
	if hp.inputCopy != nil {
		bsPool.Put(hp.inputCopy)
		hp.inputCopy = nil
	}
	hp.Reset()
	parserPool.Put(hp)
}

func (hp *HTTPParser) ParseUsePool(input []byte) (int, error) {
	if len(input) <= 0 {
		return 0, ErrMissingData
	}
	hp.inputCopy = bsPool.Get(len(input))
	copy(hp.inputCopy[0:], input)
	return hp.parseInput(hp.inputCopy)
}
