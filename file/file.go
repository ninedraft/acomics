package file

import (
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
)

type File struct {
	Data io.ReadCloser
	Size int64
	Ext  string
}

func FromResponse(resp *http.Response) *File {
	return &File{
		Data: resp.Body,
		Size: resp.ContentLength,
		Ext:  parseFileExt(resp),
	}
}

func parseFileExt(resp *http.Response) string {
	var nameExt = path.Ext(resp.Request.URL.Path)
	if nameExt != "" {
		return strings.ToLower(nameExt)
	}
	var ct = resp.Header.Get("Content-Type")
	var mimeType, _, errMIME = mime.ParseMediaType(ct)
	if errMIME == nil {
		var exts, _ = mime.ExtensionsByType(mimeType)
		if len(exts) > 0 {
			return strings.ToLower(exts[0])
		}
	}
	return ".jpeg"
}
