package parser

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"reflect"
	
	"github.com/creamsensation/form"
	"github.com/creamsensation/util"
)

type Parse interface {
	QueryParam(key string, target any) error
	PathValue(key string, target any) error
	File(filename string) (form.Multipart, error)
	Files(filesnames ...string) ([]form.Multipart, error)
	Json(target any) error
	Text() (string, error)
	Xml(target any) error
	Url(target any) error
	
	MustQueryParam(key string, target any)
	MustPathValue(key string, target any)
	MustFile(filename string) form.Multipart
	MustFiles(filesnames ...string) []form.Multipart
	MustJson(target any)
	MustText() string
	MustXml(target any)
	MustUrl(target any)
}

type Parser struct {
	r     *http.Request
	bytes []byte
	limit int64
}

func New(r *http.Request, defaultBytes []byte, limit int64) *Parser {
	return &Parser{
		r:     r,
		bytes: defaultBytes,
		limit: limit,
	}
}

func (p *Parser) QueryParam(key string, target any) error {
	t := reflect.TypeOf(target)
	if t.Kind() != reflect.Ptr {
		return ErrorPointerTarget
	}
	v := reflect.ValueOf(target)
	if !p.r.URL.Query().Has(key) {
		return ErrorQueryParamMissing
	}
	stringValue := p.r.URL.Query().Get(key)
	util.SetValueToReflected(t.Elem().Kind(), v.Elem(), stringValue)
	return nil
}

func (p *Parser) MustQueryParam(key string, target any) {
	err := p.QueryParam(key, target)
	if err != nil {
		panic(err)
	}
}

func (p *Parser) PathValue(key string, target any) error {
	t := reflect.TypeOf(target)
	if t.Kind() != reflect.Ptr {
		return ErrorPointerTarget
	}
	v := reflect.ValueOf(target)
	pathValue := p.r.PathValue(key)
	if len(pathValue) == 0 {
		return ErrorPathValueMissing
	}
	util.SetValueToReflected(t.Elem().Kind(), v.Elem(), pathValue)
	return nil
}

func (p *Parser) MustPathValue(key string, target any) {
	err := p.PathValue(key, target)
	if err != nil {
		panic(err)
	}
}

func (p *Parser) Url(target any) error {
	t := reflect.TypeOf(target)
	if t.Kind() != reflect.Ptr {
		return errors.New("target must be a pointer")
	}
	v := reflect.ValueOf(target)
	for i := 0; i < t.Elem().NumField(); i++ {
		queryKey := t.Elem().Field(i).Tag.Get("query")
		if len(queryKey) > 0 && p.r.URL.Query().Has(queryKey) {
			queryParam := p.r.URL.Query().Get(queryKey)
			util.SetValueToReflected(t.Elem().Field(i).Type.Kind(), v.Elem().Field(i), queryParam)
		}
		pathKey := t.Elem().Field(i).Tag.Get("path")
		pathValue := p.r.PathValue(pathKey)
		if len(pathKey) > 0 && len(pathValue) > 0 {
			util.SetValueToReflected(t.Elem().Field(i).Type.Kind(), v.Elem().Field(i), pathValue)
		}
	}
	return nil
}

func (p *Parser) MustUrl(target any) {
	err := p.Url(target)
	if err != nil {
		panic(err)
	}
}

func (p *Parser) Text() (string, error) {
	if len(p.bytes) > 0 {
		return string(p.bytes), nil
	}
	if p.r.Body == nil {
		return "", nil
	}
	bytes, err := io.ReadAll(p.r.Body)
	return string(bytes), err
}

func (p *Parser) MustText() string {
	r, err := p.Text()
	if err != nil {
		panic(err)
	}
	return r
}

func (p *Parser) Json(target any) error {
	if len(p.bytes) > 0 {
		return json.Unmarshal(p.bytes, target)
	}
	if p.r.Body == nil {
		return nil
	}
	err := json.NewDecoder(p.r.Body).Decode(target)
	if err == io.EOF {
		return nil
	}
	return err
}

func (p *Parser) MustJson(target any) {
	err := p.Json(target)
	if err != nil {
		panic(err)
	}
}

func (p *Parser) Xml(value any) error {
	if len(p.bytes) > 0 {
		return xml.Unmarshal(p.bytes, value)
	}
	if p.r.Body == nil {
		return nil
	}
	return xml.NewDecoder(p.r.Body).Decode(value)
}

func (p *Parser) MustXml(target any) {
	err := p.Xml(target)
	if err != nil {
		panic(err)
	}
}

func (p *Parser) File(filename string) (form.Multipart, error) {
	if len(p.bytes) > 0 {
		return form.Multipart{}, nil
	}
	err := p.parseMultipartForm()
	if err != nil {
		return form.Multipart{}, err
	}
	multiparts, err := p.createMultiparts(filename)
	if err != nil {
		return form.Multipart{}, err
	}
	if len(multiparts) == 0 {
		return form.Multipart{}, nil
	}
	return multiparts[0], nil
}

func (p *Parser) MustFile(filename string) form.Multipart {
	file, err := p.File(filename)
	if err != nil {
		panic(err)
	}
	return file
}

func (p *Parser) Files(filesname ...string) ([]form.Multipart, error) {
	if len(p.bytes) > 0 {
		return []form.Multipart{}, nil
	}
	err := p.parseMultipartForm()
	if err != nil {
		return []form.Multipart{}, err
	}
	multiparts, err := p.createMultiparts(filesname...)
	if err != nil {
		return []form.Multipart{}, err
	}
	return multiparts, nil
}

func (p *Parser) MustFiles(filesnames ...string) []form.Multipart {
	files, err := p.Files(filesnames...)
	if err != nil {
		panic(err)
	}
	return files
}

func (p *Parser) createMultiparts(filename ...string) ([]form.Multipart, error) {
	var fn string
	if len(filename) > 0 {
		fn = filename[0]
	}
	fnLen := len(fn)
	result := make([]form.Multipart, 0)
	for name, files := range p.r.MultipartForm.File {
		if fnLen > 0 && name != fn {
			continue
		}
		for _, file := range files {
			f, err := file.Open()
			if err != nil {
				return result, errors.Join(ErrorOpenFile, err)
			}
			data, err := io.ReadAll(f)
			if err != nil {
				return result, errors.Join(ErrorReadData, err)
			}
			result = append(
				result, form.Multipart{
					Key:    name,
					Name:   file.Filename,
					Type:   http.DetectContentType(data),
					Suffix: util.GetFilenameSuffix(file.Filename),
					Data:   data,
				},
			)
		}
	}
	return result, nil
}

func (p *Parser) parseMultipartForm() error {
	if !util.IsRequestMultipart(p.r) {
		return ErrorInvalidMultipart
	}
	return p.r.ParseMultipartForm(p.limit << 20)
}
