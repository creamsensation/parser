package parser

import "errors"

var (
	ErrorInvalidMultipart = errors.New("request has not multipart content type")
	ErrorOpenFile         = errors.New("file cannot be opened")
	ErrorReadData         = errors.New("cannot read data")
	ErrorPointerTarget    = errors.New("target must be a pointer")
	ErrorQueryMissing     = errors.New("query param is missing")
	ErrorPathValueMissing = errors.New("path value is missing")
)
