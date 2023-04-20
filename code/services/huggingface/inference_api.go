package huggingface

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"start-feishubot/initialization"
	"start-feishubot/services/loadbalancer"
	"strings"
	"time"

	"github.com/corpix/uarand"
)

const (
	ApiUrl = "https://api-inference.huggingface.co/models"
	Model  = "stabilityai/stable-diffusion-2-1"
)

type HuggingFace struct {
	Lb        *loadbalancer.LoadBalancer
	ApiKey    []string
	HttpProxy string
}

type HuggingFaceRequestBody struct {
	Inputs  string              `json:"inputs"`
	Options *HuggingFaceOptions `json:"options"`
}

type HuggingFaceOptions struct {
	WaitForModel bool `json:"wait_for_model"`
}

func (hf *HuggingFace) doAPIRequestWithRetry(url, method string,
	requestBody interface{}, client *http.Client, maxRetries int) (string, error) {
	var api *loadbalancer.API
	var requestBodyData []byte
	var err error

	api = hf.Lb.GetAPI()
	if api == nil {
		return "", errors.New("no available API")
	}

	requestBodyData, err = json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(requestBodyData))
	if err != nil {
		return "", err
	}

	// req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", uarand.GetRandom())
	req.Header.Set("Authorization", "Bearer "+api.Key)

	var response *http.Response
	var retry int
	for retry = 0; retry <= maxRetries; retry++ {
		response, err = client.Do(req)

		// read body
		if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
			body, _ := ioutil.ReadAll(response.Body)
			fmt.Println("body", string(body))

			hf.Lb.SetAvailability(api.Key, false)
			if retry == maxRetries {
				break
			}
			time.Sleep(time.Duration(retry+1) * time.Second)
		} else {
			break
		}
	}
	if response != nil {
		defer response.Body.Close()
	}

	if response == nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("%s api failed after %d retries", strings.ToUpper(method), retry)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	imgBase64 := base64.StdEncoding.EncodeToString(body)
	hf.Lb.SetAvailability(api.Key, true)

	return imgBase64, nil
}

func (hf *HuggingFace) sendRequest(link, method string,
	requestBody interface{}) (string, error) {
	var err error
	var imgBase64 string
	client := &http.Client{Timeout: 120 * time.Second}
	if hf.HttpProxy == "" {
		imgBase64, err = hf.doAPIRequestWithRetry(link, method,
			requestBody, client, 3)
	} else {
		proxyUrl, err := url.Parse(hf.HttpProxy)
		if err != nil {
			return "", err
		}
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		}
		proxyClient := &http.Client{
			Transport: transport,
			Timeout:   120 * time.Second,
		}
		imgBase64, err = hf.doAPIRequestWithRetry(link, method,
			requestBody, proxyClient, 3)
	}
	if err != nil {
		return "", err
	}
	return imgBase64, err
}

func NewHuggingFace(config initialization.Config) *HuggingFace {
	lb := loadbalancer.NewLoadBalancer(config.HuggingFaceApiKeys)

	return &HuggingFace{
		Lb:        lb,
		ApiKey:    config.HuggingFaceApiKeys,
		HttpProxy: config.HttpProxy,
	}
}

func (hf *HuggingFace) FullUrl(model string) string {
	url := fmt.Sprintf("%s/%s", ApiUrl, model)
	return url
}

func (hf *HuggingFace) GenerateImage(inputs string) (imgBase64 string,
	err error) {
	requestBody := HuggingFaceRequestBody{
		Inputs: inputs,
		Options: &HuggingFaceOptions{
			WaitForModel: true,
		},
	}
	url := hf.FullUrl(Model)
	return hf.sendRequest(url, http.MethodPost, requestBody)
}
