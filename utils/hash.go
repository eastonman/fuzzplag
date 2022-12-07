/*
 * @Author: Easton Man manyang.me@outlook.com
 * @Date: 2022-12-07 13:14:20
 * @LastEditors: Easton Man manyang.me@outlook.com
 * @LastEditTime: 2022-12-07 16:35:05
 * @FilePath: /fuzzplag/utils/hash.go
 * @Description:
 */
package utils

import (
	"archive/zip"
	"bytes"
	"io"
	"sync"

	"github.com/glaslos/tlsh"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/types"
	log "github.com/sirupsen/logrus"
)

var FileThreshold int = 256

type Hash struct {
	Path string
	Hash string
}

func InMemoryHash(data *bytes.Buffer, path string) []Hash {
	archive, err := zip.NewReader(bytes.NewReader(data.Bytes()), int64(data.Len()))
	if err != nil {
		log.Warnf("Error opening in-memory zip file: %s", err.Error())
	}
	hash := make([]Hash, 0)
	// Loop through all input archive
	for _, fd := range archive.File {
		if fd.FileInfo().IsDir() { // Ignore directories
			continue
		}
		log.Debug(fd.Name)
		f, err := fd.Open()
		if err != nil {
			log.Warnf("Error opening in-memory file: %s", err.Error())
		}
		defer f.Close()
		fileBuf := bytes.NewBuffer(nil)
		io.Copy(fileBuf, f)

		if fileBuf.Len() < FileThreshold { // Ignore empty or small files
			continue
		}

		fileKind, _ := filetype.Match(fileBuf.Bytes())
		log.Debugf("File type: %v", fileKind.MIME.Value)

		if fileKind == types.Get("zip") {
			log.Debugf("Recurring into %s", fd.Name)
			hash = append(hash, InMemoryHash(fileBuf, path+"/"+fd.Name)...)
			continue
		} else if fileKind == types.Get("rar") {
			log.Warnf("File %s is RAR", fd.Name)
			continue
		} else {
			// log.Warnf("File type not supported: %s", fileKind.MIME.Type)
			hashString, err := tlsh.HashBytes(fileBuf.Bytes())
			if err != nil {
				log.Warnf("Error hashing in-memory file %s: %s", fd.Name, err.Error())
				continue
			}
			hash = append(hash, Hash{
				Path: path + fd.Name,
				Hash: hashString.String(),
			})
			log.WithFields(log.Fields{
				"Path": fd.Name,
				"Hash": hashString.String(),
			}).Debug("Hash success")
			continue
		}
	}
	return hash
}

func HashForZip(path string, parallelNum int) []Hash {

	archive, err := zip.OpenReader(path)
	if err != nil {
		log.Fatalf("Error opening zip file: %s", err.Error())
	}
	defer archive.Close()
	log.Infof("Open zip file: %s success", path)

	hash := make([][]Hash, parallelNum)
	for i := 0; i < parallelNum; i++ {
		hash[i] = make([]Hash, 0)
	}

	produceQueue := make(chan *zip.File, parallelNum)
	wg := sync.WaitGroup{}

	for i := 0; i < parallelNum; i++ {
		wg.Add(1)
		go func(thread_id int) {
			for fd := range produceQueue {
				if fd.FileInfo().IsDir() {
					continue
				}
				f, err := fd.Open()
				if err != nil {
					log.Fatalf("Error opening file: %s", err.Error())
				}

				fileBuf := bytes.NewBuffer(nil)
				io.Copy(fileBuf, f)

				fileKind, _ := filetype.Match(fileBuf.Bytes())
				log.Debugf("File type: %v", fileKind.MIME.Value)

				if fileKind == filetype.Unknown {
					log.Warnf("File type not supported: %s", fileKind.MIME.Type)
					f.Close()
					continue
				}
				if fileKind == types.Get("zip") {
					log.Debugf("Recurring into %s", fd.Name)
					hash[thread_id] = append(hash[thread_id], InMemoryHash(fileBuf, fd.Name+":")...)
					f.Close()
					continue
				}
			}
			wg.Done()
		}(i)
	}

	// Send fds
	for _, fd := range archive.File {
		produceQueue <- fd
	}
	close(produceQueue)
	wg.Wait()

	flattened := make([]Hash, 0)

	for i := 0; i < parallelNum; i++ {
		flattened = append(flattened, hash[i]...)
	}

	return flattened
}
