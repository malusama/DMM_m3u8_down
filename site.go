package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"m3u8_download/DMM"
	"os"
	"path/filepath"
)

func CompressFilesOrFolds(paths []string, dest string) (err error) {
	if len(paths) < 1 {
		return errors.New("No files to compress")
	}
	if dest == "" {
		dest = paths[0] + ".tar.gz"
	}
	files := make([]*os.File, len(paths))
	defer func() {
		//因为需要等压缩完成才关闭文件句柄，因此希望不要一次压缩太多文件
		for _, f := range files {
			if f != nil {
				f.Close()
			}
		}
	}()
	for i, name := range paths {
		files[i], err = os.Open(name)
		if err != nil {
			return err
		}
	}
	return Compress(files, dest)
}

func Compress(files []*os.File, dest string) error {
	d, _ := os.Create(dest)
	defer d.Close()
	gw := gzip.NewWriter(d)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()
	for _, file := range files {
		err := compress(file, "", tw)
		if err != nil {
			return err
		}
	}
	return nil
}


func compress(file *os.File, prefix string, tw *tar.Writer) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		prefix = filepath.Join(prefix, info.Name())
		fileInfos, err := file.Readdir(-1)
		if err != nil {
			return err
		}
		for _, fi := range fileInfos {
			f, err := os.Open(filepath.Join(file.Name(), fi.Name()))
			if err != nil {
				return err
			}
			err = compress(f, prefix, tw)
			if err != nil {
				return err
			}
		}
	} else {
		header, err := tar.FileInfoHeader(info, "")
		header.Name = filepath.Join(prefix, header.Name)
		if err != nil {
			return err
		}
		err = tw.WriteHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, file)
		file.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func main()  {
	r := gin.Default()

	r.GET("/api/service_status", func(c *gin.Context ) {
		c.JSON(200, gin.H{
			"status": 200,
			"message": "服务正常开启中",
			"data": true,
		})
	})
	r.GET("/api/videoa", func(c *gin.Context) {
		productId := c.Query("productId")
		item := DMM.Run(productId, productId)

		compression := c.Query("compression")

		if compression == "zip" {
			var fileList []string
			for i := 0; i < item; i++ {
				fileList = append(fileList, fmt.Sprintf("./M3U8/%s-part%d.m3u8", productId, i))
			}
			_ = CompressFilesOrFolds(fileList, fmt.Sprintf("./M3U8/%s.gzip", productId))

			c.Header("Content-Description", "File Transfer")
			c.Header("Content-Transfer-Encoding", "binary")
			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", productId) )
			c.Header("Content-Type", "application/octet-stream")
			c.File(fmt.Sprintf("./M3U8/%s.gzip", productId))
		} else {
			c.File(fmt.Sprintf("./M3U8/%s-part0.m3u8", productId))
		}
	})
	_ = r.Run(":8081") // 在 0.0.0.0:8080 上监听并服务
}
