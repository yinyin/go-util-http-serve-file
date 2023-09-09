package httpservefile

import (
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"
	"time"
)

// ServeFS is an implementation of HTTPFileServer with fs.FS as
// content backend.
type ServeFS struct {
	urlPathPrefixLen int
	fsRef            fs.FS
	fsPathPrefix     string
	contentModTime   time.Time
}

// NewServeFSWithPrefixLength create an instance of ServeFS
// with urlPathPrefixLen, fsRef, fsPathPrefix and contentModTime.
func NewServeFSWithPrefixLength(urlPathPrefixLen int, fsRef fs.FS, fsPathPrefix string, contentModTime time.Time) (s *ServeFS, err error) {
	if urlPathPrefixLen < 1 {
		urlPathPrefixLen = 1
	}
	if contentModTime.IsZero() {
		contentModTime = time.Now()
	}
	return &ServeFS{
		urlPathPrefixLen: urlPathPrefixLen,
		fsRef:            fsRef,
		fsPathPrefix:     strings.Trim(path.Clean(fsPathPrefix), "/\\"),
		contentModTime:   contentModTime,
	}, nil
}

// NewServeFSWithPrefix create an instance of ServeFS with
// urlPathPrefix, fsRef, fsPathPrefix and contentModTime.
//
// ** CAUTION **:
// Prefix of URL path will NOT be check. Make sure such check is done at routing logic.
func NewServeFSWithPrefix(urlPathPrefix string, fsRef fs.FS, fsPathPrefix string, contentModTime time.Time) (s *ServeFS, err error) {
	urlPathPrefix = sanitizeURLPathPrefix(urlPathPrefix)
	return NewServeFSWithPrefixLength(len(urlPathPrefix), fsRef, fsPathPrefix, contentModTime)
}

// NewServeFS create an instance of ServeFS with fsRef, fsPathPrefix and contentModTime.
func NewServeFS(fsRef fs.FS, fsPathPrefix string, contentModTime time.Time) (s *ServeFS, err error) {
	return NewServeFSWithPrefixLength(1, fsRef, fsPathPrefix, contentModTime)
}

func (s *ServeFS) ServeHTTP(w http.ResponseWriter, r *http.Request, defaultFileName, targetFileName string) {
	if targetFileName == "" {
		targetFileName = extractTargetContentPath(r, s.urlPathPrefixLen, defaultFileName)
	}
	targetFilePath := path.Join(s.fsPathPrefix, targetFileName)
	if !strings.HasPrefix(targetFilePath, s.fsPathPrefix) {
		http.NotFound(w, r)
		return
	}
	fp, err := s.fsRef.Open(targetFilePath)
	if nil != err {
		if errors.Is(err, fs.ErrNotExist) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "internal error (fs-FS)", http.StatusInternalServerError)
			log.Printf("WARN: failed on open file [%s]: %v", targetFilePath, err)
		}
		return
	}
	defer fp.Close()
	if fileinfo, err := fp.Stat(); nil != err {
		http.Error(w, "internal error (fs-FS)", http.StatusInternalServerError)
		log.Printf("WARN: failed on stat file [%s]: %v", targetFilePath, err)
		return
	} else if fileinfo.IsDir() {
		http.NotFound(w, r)
		return
	}
	fAccess, ok := fp.(io.ReadSeeker)
	if !ok {
		http.Error(w, "internal error (fs-FS)", http.StatusInternalServerError)
		log.Printf("WARN: failed on cast file reference [%s]: %v", targetFilePath, err)
	}
	http.ServeContent(w, r, targetFilePath, s.contentModTime, fAccess)
}

// Close free used resources.
func (s *ServeFS) Close() (err error) {
	return
}
