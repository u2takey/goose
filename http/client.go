// An HTTP Client which sends json and binary requests, handling data marshalling and response processing.

package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	gooseerrors "launchpad.net/goose/errors"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	http.Client
	AuthToken string
}

type ErrorResponse struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
	Title   string `json:"title"`
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("Failed: %d %s: %s", e.Code, e.Title, e.Message)
}

type ErrorWrapper struct {
	Error ErrorResponse `json:"error"`
}

type RequestData struct {
	ReqHeaders     http.Header
	Params         url.Values
	ExpectedStatus []int
	ReqValue       interface{}
	RespValue      interface{}
	ReqData        []byte
	RespData       *[]byte
}

// JsonRequest JSON encodes and sends the supplied object (if any) to the specified URL.
// Optional method arguments are pass using the RequestData object.
// Relevant RequestData fields:
// ReqHeaders: additional HTTP header values to add to the request.
// ExpectedStatus: the allowed HTTP response status values, else an error is returned.
// ReqValue: the data object to send.
// RespValue: the data object to decode the result into.
func (c *Client) JsonRequest(method, url string, reqData *RequestData) (err error) {
	err = nil
	var (
		req  *http.Request
		body []byte
	)
	if reqData.Params != nil {
		url += "?" + reqData.Params.Encode()
	}
	if reqData.ReqValue != nil {
		body, err = json.Marshal(reqData.ReqValue)
		if err != nil {
			err = gooseerrors.AddContext(err, "failed marshalling the request body")
			return
		}
		reqBody := strings.NewReader(string(body))
		req, err = http.NewRequest(method, url, reqBody)
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		err = gooseerrors.AddContext(err, "failed creating the request")
		return
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	respBody, err := c.sendRequest(req, reqData.ReqHeaders, reqData.ExpectedStatus, string(body))
	if err != nil {
		return
	}

	if len(respBody) > 0 {
		if reqData.RespValue != nil {
			var mm interface {}
			json.Unmarshal(respBody, &mm)
			fmt.Println(mm)
			err = json.Unmarshal(respBody, &reqData.RespValue)
			if err != nil {
				err = gooseerrors.AddContext(err, "failed unmarshaling the response body: %s", respBody)
			}
		}
	}
	return
}

// Sends the supplied byte array (if any) to the specified URL.
// Optional method arguments are pass using the RequestData object.
// Relevant RequestData fields:
// ReqHeaders: additional HTTP header values to add to the request.
// ExpectedStatus: the allowed HTTP response status values, else an error is returned.
// ReqData: the byte array to send.
// RespData: the byte array to decode the result into.
func (c *Client) BinaryRequest(method, url string, reqData *RequestData) (err error) {
	err = nil

	var req *http.Request

	if reqData.Params != nil {
		url += "?" + reqData.Params.Encode()
	}
	if reqData.ReqData != nil {
		rawReqReader := bytes.NewReader(reqData.ReqData)
		req, err = http.NewRequest(method, url, rawReqReader)
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		err = gooseerrors.AddContext(err, "failed creating the request")
		return
	}
	req.Header.Add("Content-Type", "application/octet-stream")
	req.Header.Add("Accept", "application/octet-stream")

	respBody, err := c.sendRequest(req, reqData.ReqHeaders, reqData.ExpectedStatus, string(reqData.ReqData))
	if err != nil {
		return
	}

	if len(respBody) > 0 {
		if reqData.RespData != nil {
			*reqData.RespData = respBody
		}
	}
	return
}

// Sends the specified request and checks that the HTTP response status is as expected.
// req: the request to send.
// extraHeaders: additional HTTP headers to include with the request.
// expectedStatus: a slice of allowed response status codes.
// payloadInfo: a string to include with an error message if something goes wrong.
func (c *Client) sendRequest(req *http.Request, extraHeaders http.Header, expectedStatus []int, payloadInfo string) (respBody []byte, err error) {
	if extraHeaders != nil {
		for header, values := range extraHeaders {
			for _, value := range values {
				req.Header.Add(header, value)
			}
		}
	}
	if c.AuthToken != "" {
		req.Header.Add("X-Auth-Token", c.AuthToken)
	}
	rawResp, err := c.Do(req)
	if err != nil {
		err = gooseerrors.AddContext(err, "failed executing the request")
		return
	}
	foundStatus := false
	if len(expectedStatus) == 0 {
		expectedStatus = []int{http.StatusOK}
	}
	for _, status := range expectedStatus {
		if rawResp.StatusCode == status {
			foundStatus = true
			break
		}
	}
	if !foundStatus && len(expectedStatus) > 0 {
		defer rawResp.Body.Close()
		var errInfo interface{}
		errInfo, _ = ioutil.ReadAll(rawResp.Body)
		// Check if we have a JSON representation of the failure, if so decode it.
		if rawResp.Header.Get("Content-Type") == "application/json" {
			var wrappedErr ErrorWrapper
			if err := json.Unmarshal(errInfo.([]byte), &wrappedErr); err == nil {
				errInfo = wrappedErr.Error
			}
		}
		err = errors.New(
			fmt.Sprintf(
				"request (%s) returned unexpected status: %s; error info: %v; request body: %s",
				req.URL,
				rawResp.Status,
				errInfo,
				payloadInfo))
		return
	}

	respBody, err = ioutil.ReadAll(rawResp.Body)
	rawResp.Body.Close()
	if err != nil {
		err = gooseerrors.AddContext(err, "failed reading the response body")
		return
	}
	return
}
