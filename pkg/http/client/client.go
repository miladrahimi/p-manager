package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"github.com/cockroachdb/errors"
	"io"
	"net/http"
	"time"
)

type Client struct {
	E *http.Client
}

func (f *Client) Do(method, url string, body interface{}, headers map[string]string) ([]byte, error) {
	var requestReader io.Reader
	var jsonBody []byte
	if body != nil {
		var err error
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		requestReader = bytes.NewBuffer(jsonBody)
	}

	request, err := http.NewRequest(method, url, requestReader)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create request, method: %s, url: %s", method, url)
	}

	for key, value := range headers {
		request.Header.Add(key, value)
	}

	response, err := f.E.Do(request)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot do request, method: %s, url: %s", method, url)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	is2xx := response.StatusCode >= 200 && response.StatusCode < 300
	is4xx := response.StatusCode >= 400 && response.StatusCode < 500
	if is2xx || is4xx {
		responseBody, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, errors.Wrapf(
				err, "invalid response, method: %s, url: %s, body: %s", method, url, response.Body,
			)
		}

		if is4xx {
			return nil, errors.Errorf("external error, method: %s, url: %s, body: %s", method, url, responseBody)
		}

		return responseBody, nil
	}

	return nil, errors.Errorf("failed, method: %s, url: %s, status: %d", method, url, response.StatusCode)
}

func New(timeout int) *Client {
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return &Client{
		E: &http.Client{
			Transport: customTransport,
			Timeout:   time.Duration(timeout) * time.Second,
		},
	}
}
