package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

type OcrResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func (o OcrResponse) String() string {
	return fmt.Sprintf("{\n \"code\": %d,\n \"message\": \"%s\",\n \"data\": \"%s\"\n}", o.Code, o.Message, o.Data)
}

var defaultApi = "http://localhost:28888/ocr"

func CallWithFile(filepath string, apis ...string) (ocrResp *OcrResponse, err error) {
	api2use := chooseApi(apis)
	imageData, err := os.ReadFile(filepath)
	if err != nil {
		return
	}
	base64Image := base64.StdEncoding.EncodeToString(imageData)
	return CallWithBase64(base64Image, api2use)
}

func CallWithBase64(base64Image string, apis ...string) (ocrResp *OcrResponse, err error) {
	api2use := chooseApi(apis)
	data := url.Values{}
	data.Set("image", base64Image)
	data.Set("probability", "false")
	data.Set("png_fix", "false")
	http.DefaultClient.Timeout = time.Second * 5
	resp, err := http.PostForm(api2use, data)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, &ocrResp)
	return
}

func chooseApi(apis []string) (api2Use string) {
	if len(apis) == 0 || apis[0] == "" {
		api2Use = defaultApi
	} else {
		api2Use = apis[0]
	}
	return
}
