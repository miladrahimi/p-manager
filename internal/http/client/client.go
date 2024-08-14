package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
	"time"
)

type Client struct {
	e          *http.Client
	appName    string
	appVersion string
}

func (c *Client) Do(method, url, token string, body interface{}) ([]byte, error) {
	info := map[string]interface{}{
		"request_method": method,
		"request_url":    url,
		"request_token":  token,
	}

	var requestReader io.Reader
	var requestBody []byte
	if body != nil {
		var err error
		requestBody, err = json.Marshal(body)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot unmarshall request body, %v", info)
		}
		requestReader = bytes.NewBuffer(requestBody)
		info["request_body"] = string(requestBody)
	}

	request, err := http.NewRequest(method, url, requestReader)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create request, %v", info)
	}

	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
	request.Header.Set("X-App-Name", c.appName)
	request.Header.Set("X-App-Version", c.appVersion)

	response, err := c.e.Do(request)
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
		if err != nil {
			return nil, errors.Wrapf(err, "cannot read response body, %v", info)
		}
		info["response_body"] = string(responseBody)
		if is4xx {
			return responseBody, errors.Errorf("bad request received, %v", info)
		}
		return responseBody, nil
	}

	return nil, errors.Errorf("unknown respose received, %s", info)
}

func (c *Client) DoThrough(proxy, method, url, token string, body interface{}) ([]byte, error) {
	return c.Do(method, fmt.Sprintf("%s/?url=%s", proxy, url), token, body)
}

func New(timeout int, appName, appVersion string) *Client {
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return &Client{
		appName:    appName,
		appVersion: appVersion,
		e: &http.Client{
			Transport: customTransport,
			Timeout:   time.Duration(timeout) * time.Second,
		},
	}
}
