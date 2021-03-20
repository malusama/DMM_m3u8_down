package DMM

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/gogf/gf/util/gconv"
	"github.com/levigross/grequests"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

const UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 11_2_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36 Edg/87.0.664.75"
var COOKIE string
var Service string
var ShopName string

type M3U8List struct {
	List struct {
		Cid  string `json:"cid"`
		Item []struct {
			Bitrate        string `json:"bitrate"`
			URL            string `json:"url"`
			QualityName    string `json:"quality_name"`
			DefaultQuality bool   `json:"default_quality"`
		} `json:"item"`
	} `json:"list"`
}

type PlayInfo struct {
	Action    string `json:"action"`
	Service   string `json:"service"`
	FirstPlay int    `json:"first_play"`
	List      struct {
		Item []struct {
			Index           int    `json:"index"`
			Name            string `json:"name"`
			ProductID       string `json:"product_id"`
			ParentProductID string `json:"parent_product_id"`
			ShopName        string `json:"shop_name"`
			Category        string `json:"category"`
			PackageImage    string `json:"package_image"`
			Part            int    `json:"part"`
			Media           string `json:"media"`
		} `json:"item"`
	} `json:"list"`
}

func init() {
	viper.SetConfigFile("./config.yaml") // 指定配置文件路径
	err := viper.ReadInConfig() // 查找并读取配置文件

	if err != nil { // 处理读取配置文件的错误
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	COOKIE = viper.GetString("cookie")
	Service = viper.GetString("Service")
	ShopName = viper.GetString("ShopName")
	logger.Info(fmt.Sprintf("service: %s\n shopname: %s \n cookie: %s", Service, ShopName, COOKIE))
}

func ParseGzip(data []byte) ([]byte, error) {
	b := new(bytes.Buffer)
	_ = binary.Write(b, binary.LittleEndian, data)
	r, err := gzip.NewReader(b)
	if err != nil {
		logger.Info("[ParseGzip] NewReader error: %v, maybe data is ungzip", err)
		return nil, err
	} else {
		defer r.Close()
		undatas, err := ioutil.ReadAll(r)
		if err != nil {
			logger.Warn("[ParseGzip]  ioutil.ReadAll error: %v", err)
			return nil, err
		}
		return undatas, nil
	}
}



func GetWebPage(url string) []string {
	response, _ := grequests.Get(url, &grequests.RequestOptions{
		RedirectLimit: 0,
		Headers: map[string]string{
			"User-Agent": UserAgent,
			"Accept-Language": "zh-Hans-CN,zh-CN;q=0.9,zh;q=0.8,en;q=0.7,en-GB;q=0.6,en-US;q=0.5,zh-TW;q=0.4,fr;q=0.3,ja;q=0.2",
			"cookie": COOKIE,
		},
	})

	return strings.Split(response.String(), "\n")
}

func GetM3U8List(productId, parentProductId string, part int) M3U8List {
	response, _ := grequests.Post("https://www.dmm.co.jp/service/digitalapi/-/html5/", &grequests.RequestOptions{
		JSON: map[string]interface{}{
			"action": "playinfo",
			"category": "adult",
			"format": "json",
			"index": 0,
			"mail": "yes",
			"media": "",
			"parent_product_id": parentProductId,
			"part": part,
			"product_id": productId,
			"service": Service,
			"shop_name": ShopName,

		},
		Headers: map[string]string{
			"User-Agent": UserAgent,
			"Accept-Encoding": "gzip, deflate, br",
			"Accept-Language": "zh-CN,zh;q=0.9,zh-TW;q=0.8,ja;q=0.7",
			"Referer": "https://www.dmm.co.jp/monthly/premium/-/detail/=/cid={product_id}/",
			"Connection": "keep-alive",
			"Host": "www.dmm.co.jp",
			"Origin": "https://www.dmm.co.jp",
			"DNT": "1",
			"Content-Type": "application/json",
			"X-Requested-With": "XMLHttpRequest",
			"cookie": COOKIE,
		},
	})

	res, _ := ParseGzip(response.Bytes())
	var m3u8List M3U8List
	_ = json.Unmarshal(res, &m3u8List)
	return m3u8List
}

func GetPlayInfo(productId, ParentProductID string) PlayInfo {
	//proxyURL, err :=  url.Parse("http://localhost:10086")
	//if err != nil {
	//	fmt.Print(err)
	//}
	// https://www.dmm.co.jp/service/digitalapi/-/html5/
	response, _ := grequests.Post("https://www.dmm.co.jp/service/digitalapi/-/html5/", &grequests.RequestOptions{
		//DisableCompression: true,
		JSON: map[string]string{
			"HTTP_HOST": "www.dmm.co.jp",
			"REQUEST_URI": fmt.Sprintf("/digital/-/player/=/player=html5/act=playlist/pid=%s/view_flag=1/parent_product_id=%sdl/", productId, ParentProductID),
			"act": "playlist",
			"action": "playlist",
			"adult_flag": "1",
			"browser": "chrome",
			"exploit_id": "vGdQqupBIQRKHNYRQ4y2XA==",
			"format": "json",
			"mail": "yes",
			"parent_product_id": fmt.Sprintf("%s", ParentProductID),
			"product_id": fmt.Sprintf("%s", productId),
			"service": Service,
			"shop_name": ShopName,
			"view_flag": "1",
		},
		//Proxies: map[string]*url.URL{proxyURL.Scheme: proxyURL},
		Headers: map[string]string{
			"User-Agent":"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3770.100 Safari/537.36",
			"Accept-Encoding":"gzip, deflate, br",
			"Accept-Language":"zh-CN,zh;q=0.9,zh-TW;q=0.8,ja;q=0.7",
			"REQUEST_URI": fmt.Sprintf("/digital/-/player/=/player=html5/act=playlist/pid=%s/view_flag=1/parent_product_id=%sdl/", productId, ParentProductID),
			"Connection":"keep-alive",
			"Host":"www.dmm.co.jp",
			"Origin":"https://www.dmm.co.jp",
			"DNT":"1",
			"Content-Type":"application/json",
			"X-Requested-With":"XMLHttpRequest",
			"cookie":COOKIE,
		},
	})
	res, _ := ParseGzip(response.Bytes())
	var playInfo PlayInfo
	_ = json.Unmarshal(res, &playInfo)
	return playInfo
}

func GetM3U8Url(m3u8List M3U8List) (string, int) {
	bitrate := 0
	var url string
	for _, v := range m3u8List.List.Item {
		if gconv.Int(v.Bitrate) > bitrate {
			bitrate = gconv.Int(v.Bitrate)
			url = v.URL
		}
	}
	return url, bitrate
}

func GetKey(url string) string {
	response, _ := grequests.Get(url, &grequests.RequestOptions{
		Headers: map[string]string{
			"User-Agent": UserAgent,
			"Accept-Language": "zh-Hans-CN,zh-CN;q=0.9,zh;q=0.8,en;q=0.7,en-GB;q=0.6,en-US;q=0.5,zh-TW;q=0.4,fr;q=0.3,ja;q=0.2",
			"cookie": COOKIE,
		},
	})

	return base64.StdEncoding.EncodeToString(response.Bytes())
}

func ReplaceM3U8(M3U8 []string, URL string) []string {
	var result []string
	for _, k := range M3U8 {
		if strings.HasPrefix(k,"#EXT-X-KEY:") {
			urlSplit := strings.Split(k, ",")[1]
			key := fmt.Sprintf("#EXT-X-KEY:METHOD=AES-128,URI=\"base64:%s\"", GetKey(urlSplit[5:len(urlSplit)-1]))
			result = append(result, key)
			continue
		}

		if strings.HasPrefix(k, "media") {
			result = append(result, fmt.Sprintf("%s/-/%s", URL, k))
			continue
		}
		result = append(result, k)
	}
	return result
}

func WriteM3U8(m3u8 []string, ProductID string, Part int) {
	file, err := os.OpenFile(fmt.Sprintf("./M3U8/%s-part%d.m3u8", ProductID, Part), os.O_WRONLY | os.O_CREATE, os.ModeAppend|os.ModePerm)
	if err != nil {
		fmt.Println(err)
	}

	defer file.Close()

	for _, v := range m3u8 {
		_, err = file.WriteString(fmt.Sprintf("%s\n", v))
		if err != nil {
			fmt.Println(err)
		}
	}
	logger.Info(fmt.Sprintf("write ./M3U8/%s-part%d.m3u8", ProductID, Part))
}


func GetLocation(url string) string {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse /* 不进入重定向 */
		},
	}
	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		log.Fatal(err)
	}
	return resp.Header.Get("Location")
}

func GetM3u8File(productId, parentProductId string) []string {
	logger.Info(fmt.Sprintf("get productId: %s", productId))
	playInfo := GetPlayInfo(productId, parentProductId)
	for _, v := range playInfo.List.Item {
		m3u8List := GetM3U8List(v.ProductID, v.ParentProductID, v.Part)
		productURL, _ := GetM3U8Url(m3u8List)
		content := GetWebPage(productURL)
		chunkList := strings.Split(productURL, "/-/")[0] + "/-/" + content[len(content)-2]
		m3u8 := GetWebPage(chunkList)

		location := GetLocation(productURL)
		return ReplaceM3U8(m3u8, strings.Split(location, "/-/")[0])
	}
	return nil
}

func Run(productId, parentProductId string) int {
	logger.Info(fmt.Sprintf("get productId: %s", productId))
	playInfo := GetPlayInfo(productId, parentProductId)
	logger.Info(playInfo)
	for Index, v := range playInfo.List.Item {
		m3u8List := GetM3U8List(v.ProductID, v.ParentProductID, v.Part)
		productURL, _ := GetM3U8Url(m3u8List)
		content := GetWebPage(productURL)
		chunkList := strings.Split(productURL, "/-/")[0] + "/-/" + content[len(content)-2]
		m3u8 := GetWebPage(chunkList)

		location := GetLocation(productURL)
		WriteM3U8(ReplaceM3U8(m3u8, strings.Split(location, "/-/")[0]), v.ProductID, Index)
	}
	return len(playInfo.List.Item)
}
