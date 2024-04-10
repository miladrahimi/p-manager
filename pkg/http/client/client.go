package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"github.com/cockroachdb/errors"
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
	"time"
)

type Client struct {
	E          *http.Client
	appName    string
	appVersion string
}

func (c *Client) Do(method, url string, body interface{}, headers map[string]string) ([]byte, error) {
	info := map[string]interface{}{
		"method":          method,
		"url":             url,
		"request_headers": headers,
	}

	var requestReader io.Reader
	var jsonBody []byte
	if body != nil {
		var err error
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot unmarshall request body, %v", info)
		}
		requestReader = bytes.NewBuffer(jsonBody)
		info["request_body"] = string(jsonBody)
	}

	request, err := http.NewRequest(method, url, requestReader)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create request, %v", info)
	}

	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set("X-App-Name", c.appName)
	request.Header.Set("X-App-AppVersion", c.appVersion)
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := c.E.Do(request)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot do request, %v", info)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	info["response_status"] = response.StatusCode

	is2xx := response.StatusCode >= 200 && response.StatusCode < 300
	is4xx := response.StatusCode >= 400 && response.StatusCode < 500
	if is2xx || is4xx {
		responseBody, err := io.ReadAll(response.Body)
		info["response_body"] = response.StatusCode
		if err != nil {
			return nil, errors.Wrapf(err, "cannot unmarshall response body, %v", info)
		}
		if is4xx {
			return nil, errors.Errorf("bad request received, %v", info)
		}
		return responseBody, nil
	}

	return nil, errors.Errorf("unknown respose received, %s", info)
}

func New(timeout int, appName, appVersion string) *Client {
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return &Client{
		appName:    appName,
		appVersion: appVersion,
		E: &http.Client{
			Transport: customTransport,
			Timeout:   time.Duration(timeout) * time.Second,
		},
	}
}
