package fetcher

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
	"time"
)

type Fetcher struct {
	Engine *http.Client
}

func (f *Fetcher) Do(method, url string, body interface{}, headers map[string]string) ([]byte, error) {
	var requestReader io.Reader
	var jsonBody []byte
	if body != nil {
		var err error
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return nil, f.errWrap(err, "cannot marshal body", method, url)
		}
		requestReader = bytes.NewBuffer(jsonBody)
	}

	request, err := http.NewRequest(method, url, requestReader)
	if err != nil {
		return nil, f.errWrap(err, "cannot create request", method, url)
	}

	request.Header.Add(echo.HeaderContentType, echo.MIMEApplicationJSON)
	for key, value := range headers {
		request.Header.Add(key, value)
	}

	response, err := f.Engine.Do(request)
	if err != nil {
		return nil, f.errWrap(err, "cannot do request", method, url)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusUnprocessableEntity {
		return nil, f.errMake(fmt.Sprintf("bad response status: %d", response.StatusCode), method, url)
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, f.errWrap(err, "cannot read response body", method, url)
	}

	if response.StatusCode == http.StatusUnprocessableEntity {
		message := fmt.Sprintf("bad request %s, response: %s", jsonBody, responseBody)
		return nil, f.errMake(message, method, url)
	}

	return responseBody, nil
}

func (f *Fetcher) errWrap(err error, message string, method, url string) error {
	return fmt.Errorf("fetcher: %s, method: %s, url: %s err: %s", message, method, url, err.Error())
}

func (f *Fetcher) errMake(message string, method, url string) error {
	return fmt.Errorf("fetcher: %s, method: %s, url: %s", message, method, url)
}

func New(timeout int) *Fetcher {
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return &Fetcher{
		Engine: &http.Client{
			Transport: customTransport,
			Timeout:   time.Duration(timeout) * time.Second,
		},
	}
}
