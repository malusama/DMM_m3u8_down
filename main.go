package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/levigross/grequests"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"m3u8_download/DMM"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var CLIENT influxdb2.Client
const token = "39m9J5Y8AKyWBtfe7FAcWqDCDIkjmSB-1YkqBCCfTHQhXYbOwQvZ4yTe3p-J67rtrOGy5a1t92lcS5zGBHZ5nw=="
const bucket = "M3U8"
const org = "chensichengmalu@gmail.com"
var DownloadBit = 0


func init() {


	type ServerStatus struct {
		Status int `json:"status"`
		Message string `json:"message"`
		Data bool `json:"data"`
	}

	url := "http://139.180.198.39:8081/api/service_status"
	response, _ := grequests.Get(url, nil)
	var serverStatus ServerStatus
	_ = response.JSON(&serverStatus)

	if response.StatusCode != 200 || serverStatus.Data == false {
		panic(fmt.Sprintf("服务未开启: %s", url))
	}

	PathExists("./done")
	PathExists("./M3U8")
	PathExists("./download")
	PathExists("./M3U8_Done")

	// You can generate a Token from the "Tokens Tab" in the UI
	CLIENT = influxdb2.NewClient("https://us-central1-1.gcp.cloud2.influxdata.com", token)
	// always close client at the end
	defer CLIENT.Close()
}

func PathExists(path string) {
	_, err := os.Stat(path)

	if os.IsNotExist(err) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			log.Error(fmt.Sprintf("%s mkdir failed![%v]\n", path, err))
		} else {
			log.Info(fmt.Sprintf("%s mkdir success!\n", path))
		}
	}
}

func PKCS5Padding(plaintext []byte, blockSize int) []byte{
	padding := blockSize-len(plaintext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)},padding)
	return append(plaintext,padtext...)
}

func AesEncrypt(origData, key []byte) ([]byte, error){
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	//AES 分组长度为 128 位，所以 blockSize=16，单位字节
	blockSize := block.BlockSize()
	origData = PKCS5Padding(origData,blockSize)
	blockMode := cipher.NewCBCEncrypter(block,key[:blockSize])	//初始向量的长度必须等于块 block 的长度 16 字节
	crypted := make([]byte, len(origData))
	blockMode.CryptBlocks(crypted,origData)
	return crypted, nil
}


func PKCS5UnPadding(origData []byte) []byte{
	length := len(origData)
	unPadding := int(origData[length-1])
	return origData[:(length - unPadding)]
}


func AesDecrypt(crypted, key []byte)([]byte, error) {

	block, err := aes.NewCipher(key)
	if err != nil {
		log.Error(err)
	}

	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, key[:blockSize])
	origData := make([]byte, len(crypted))
	blockMode.CryptBlocks(origData, crypted)
	origData = PKCS5UnPadding(origData)
	return origData, nil
}


func decrypt(content, key []byte) ([]byte, error) {
	decryptByte, err := AesDecrypt(content, key)
	if err != nil {
		return nil, err
	}

	return decryptByte, nil
}

func parseShard(url string) string {
	parse := strings.Split(url, "_")
	return parse[len(parse)-1]
}

func readM3U8(source []string) (link []string, key []byte) {
	for _, i := range source {
		if strings.HasPrefix(i, "#EXT-X-KEY") {
			key, _ = base64.StdEncoding.DecodeString(i[len(i)-25:len(i) - 1])
		}

		if strings.HasPrefix(i, "http") {
			link = append(link, i)
		}
	}
	return link, key
}

func Exist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}

func download(ch chan string, key []byte) {

	for {
		url := <- ch

		if Exist(fmt.Sprintf("./download/%s", parseShard(url))) {
			// 如果文件已经存在跳过
			log.Info("found  ", parseShard(url))
			continue
		}

		log.Info(fmt.Sprintf("download %s", parseShard(url)))
		res, err := grequests.Get(url, nil)

		if res.StatusCode != 200 || err != nil {
			log.Error(fmt.Sprintf("%s loss, StatusCode: %d, err:%s",  parseShard(url), res.StatusCode, err))
			ch <- url
			log.Info("len: $d", len(ch))
			continue
		}
		content := res.Bytes()
		DownloadBit += len(content)
		decryptByte, err := decrypt(content, key)
		if err != nil {
			log.Error("err: %s, fileName: ", err, parseShard(url))
			ch <- url
			continue
		}

		err = ioutil.WriteFile(fmt.Sprintf("./download/%s", parseShard(url)), decryptByte, os.FileMode(0666))
		if err != nil {
			log.Error("err: %s, filename: ", err, parseShard(url))
			ch <- url
			continue
		}
		log.Info("Downloading: ", parseShard(url), ", done")
	}
}

func RemoveContents(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}
	return nil
}

func convert(PathName string) {
	files, _ := ioutil.ReadDir("./download")

	var fileName []string

	for _, f := range files {
		if strings.HasSuffix(f.Name(), "ts") {
			fileName = append(fileName, f.Name())
		}
	}

	sort.Slice(fileName, func(i, j int) bool {
		iShard, _ := strconv.Atoi(strings.Split(fileName[i], ".")[0])
		jShard, _ := strconv.Atoi(strings.Split(fileName[j], ".")[0])
		return iShard < jShard
	})

	f, err := os.OpenFile("./list.txt", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	defer f.Close()

	if err != nil{
		log.Error(err)
	}


	var stdoutBuf, stderrBuf bytes.Buffer
	pwd, _ := os.Getwd()

	for _, i := range fileName {
		_, _ = f.WriteString(fmt.Sprintf("file %s/download/"+i+"\n", pwd))
	}


	args := []string{"-f", "concat", "-safe", "0", "-i", fmt.Sprintf("%s/list.txt", pwd), "-c", "copy",
		fmt.Sprintf("%s/done/%s.mp4", pwd, PathName)}
	cmd := exec.Command("ffmpeg", args...)

	stdoutIn, _ := cmd.StdoutPipe()
	stderrIn, _ := cmd.StderrPipe()
	var errStdout, errStderr error
	stdout := io.MultiWriter(os.Stdout, &stdoutBuf)
	stderr := io.MultiWriter(os.Stderr, &stderrBuf)
	err = cmd.Start()
	if err != nil {
		log.Fatalf("cmd.Start() failed with '%s'\n", err)
	}
	go func() {
		_, errStdout = io.Copy(stdout, stdoutIn)
	}()
	go func() {
		_, errStderr = io.Copy(stderr, stderrIn)
	}()
	err = cmd.Wait()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	if errStdout != nil || errStderr != nil {
		log.Fatal("failed to capture stdout or stderr\n")
	}
	outStr, _ := string(stdoutBuf.Bytes()), string(stderrBuf.Bytes())
	fmt.Printf("\r%s", outStr)
	_ = RemoveContents("./download")

}

func ParseM3U8File(filename string) []string {

	f, err := ioutil.ReadFile(fmt.Sprintf("./M3U8/%s", filename))
	if err != nil {
		log.Error(err)
	}
	var source []string
	for _, v := range f{
		source = append(source, string(v))
	}

	return source
}

func downloadTask(source []string, filename string) {
	ch := make(chan string)

	link, key := readM3U8(source)

	for i :=0; i <= 16; i++ {
		go download(ch, key)
	}

	for _, i := range link {
		ch <- i
	}
	//log.Info("convert")
	//convert(filename)
	//log.Info("done")
}

func MoveFile(sourcePath, destPath string) error {
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("Couldn't open source file: %s", err)
	}
	outputFile, err := os.Create(destPath)
	if err != nil {
		inputFile.Close()
		return fmt.Errorf("Couldn't open dest file: %s", err)
	}
	defer outputFile.Close()
	_, err = io.Copy(outputFile, inputFile)
	inputFile.Close()
	if err != nil {
		return fmt.Errorf("Writing to output file failed: %s", err)
	}
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	if err != nil {
		return fmt.Errorf("Failed removing original file: %s", err)
	}
	return nil
}

func run() {

	ts, err := ioutil.ReadDir("./M3U8")

	if err != nil {
		log.Error(err)
	}

	for _, fi := range ts {
		source := ParseM3U8File(fi.Name())
		downloadTask(source, fi.Name())
		_ = MoveFile(fmt.Sprintf("./M3U8/%s", fi.Name()), fmt.Sprintf("./M3U8_Done/%s", fi.Name()))
	}
}

func getIPs() (ips []string) {
	interfaceAddr, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Printf("fail to get net interface addrs: %v", err)
		return ips
	}

	for _, address := range interfaceAddr {
		ipNet, isValidIpNet := address.(*net.IPNet)
		if isValidIpNet && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				ips = append(ips, ipNet.IP.String())
			}
		}
	}
	return ips
}

func statistics(productId string, ctx context.Context) {
	// get non-blocking write client
	writeAPI := CLIENT.WriteAPI(org, bucket)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * 5):
			writeAPI.WriteRecord(fmt.Sprintf("stat,ProductId=%s DownloadBit=%d", productId, DownloadBit / 5))
			//Flush writes
			writeAPI.Flush()
			DownloadBit = 0
		}
	}
}


func FetchM3U8(productId string) []string {
	response, err := grequests.Get("http://139.180.198.39:8081/api/videoa", &grequests.RequestOptions{
		Params: map[string]string{
			"productId": productId,
		},
	})
	if err != nil {
		log.Error(err)
	}

	return strings.Split(response.String(), "\n")
}


func main()  {
	//log.Info("version: 0.0.2")
	//fmt.Println("输入 ProductId")
	//var productId string
	//_, _ = fmt.Scanln(&productId)

	DMM.Run("avop00304", "avop00304")
	//ctx, cancel := context.WithCancel(context.Background())
	//
	//go statistics(productId, ctx)
	//
	//source := FetchM3U8(productId)
	//downloadTask(source, productId)
	//
	//cancel()
	//log.Info("done")
	//select{}
}