// The swift package provides a way to access the OpenStack Object Storage APIs.
// See http://docs.openstack.org/api/openstack-object-storage/1.0/content/.
package swift

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"launchpad.net/goose/client"
	"launchpad.net/goose/errors"
	goosehttp "launchpad.net/goose/http"
	"net/http"
	"net/url"
	"time"
)

// Client provides a means to access the OpenStack Object Storage Service.
type Client struct {
	client client.Client
}

func New(client client.Client) *Client {
	return &Client{client}
}

// CreateContainer creates a container with the given name.
func (c *Client) CreateContainer(containerName string) error {
	// Juju expects there to be a (semi) public url for some objects. This
	// could probably be more restrictive or placed in a separate container
	// with some refactoring, but for now just make everything public.
	headers := make(http.Header)
	headers.Add("X-Container-Read", ".r:*")
	url := fmt.Sprintf("/%s", containerName)
	requestData := goosehttp.RequestData{ReqHeaders: headers, ExpectedStatus: []int{http.StatusAccepted, http.StatusCreated}}
	err := c.client.SendRequest(client.PUT, "object-store", url, &requestData)
	if err != nil {
		err = errors.Newf(errors.UnspecifiedError, err, nil, "failed to create container: %s", containerName)
	}
	return err
}

// DeleteContainer deletes the specified container.
func (c *Client) DeleteContainer(containerName string) error {
	url := fmt.Sprintf("/%s", containerName)
	requestData := goosehttp.RequestData{ExpectedStatus: []int{http.StatusNoContent}}
	err := c.client.SendRequest(client.DELETE, "object-store", url, &requestData)
	if err != nil {
		err = errors.Newf(errors.UnspecifiedError, err, nil, "failed to delete container: %s", containerName)
	}
	return err
}

func (c *Client) touchObject(requestData *goosehttp.RequestData, op, containerName, objectName string) error {
	path := fmt.Sprintf("/%s/%s", containerName, objectName)
	err := c.client.SendRequest(op, "object-store", path, requestData)
	if err != nil {
		err = errors.Newf(errors.UnspecifiedError, err, nil, "failed to %s object %s from container %s", op, objectName, containerName)
	}
	return err
}

// HeadObject retrieves object metadata and other standard HTTP headers.
func (c *Client) HeadObject(containerName, objectName string) (headers http.Header, err error) {
	requestData := goosehttp.RequestData{ReqHeaders: headers}
	err = c.touchObject(&requestData, client.HEAD, containerName, objectName)
	return headers, err
}

// GetObject retrieves the specified object's data.
func (c *Client) GetObject(containerName, objectName string) (obj []byte, err error) {
	rc, err := c.GetReader(containerName, objectName)
	if err != nil {
		return
	}
	defer rc.Close()
	return ioutil.ReadAll(rc)
}

// The following defines a ReadCloser implementation which reads no data.
// It is used instead of returning a nil pointer, which is the same as http.Request.Body.
var emptyReadCloser noData

type noData struct {
	io.ReadCloser
}

// GetObject retrieves the specified object's data.
func (c *Client) GetReader(containerName, objectName string) (rc io.ReadCloser, err error) {
	requestData := goosehttp.RequestData{RespReader: &emptyReadCloser}
	err = c.touchObject(&requestData, client.GET, containerName, objectName)
	return requestData.RespReader, err
}

// DeleteObject removes an object from the storage system permanently.
func (c *Client) DeleteObject(containerName, objectName string) error {
	requestData := goosehttp.RequestData{ExpectedStatus: []int{http.StatusNoContent}}
	err := c.touchObject(&requestData, client.DELETE, containerName, objectName)
	return err
}

// PutObject writes, or overwrites, an object's content and metadata.
func (c *Client) PutObject(containerName, objectName string, data []byte) error {
	r := bytes.NewReader(data)
	return c.PutReader(containerName, objectName, r)
}

// PutReader writes, or overwrites, an object's content and metadata.
func (c *Client) PutReader(containerName, objectName string, r io.Reader) error {
	requestData := goosehttp.RequestData{ReqReader: r, ExpectedStatus: []int{http.StatusCreated}}
	err := c.touchObject(&requestData, client.PUT, containerName, objectName)
	return err
}

type ContainerContents struct {
	Name         string `json:"name"`
	Hash         string `json:"hash"`
	LengthBytes  int    `json:"bytes"`
	ContentType  string `json:"content_type"`
	LastModified string `json:"last_modified"`
}

// GetObject retrieves the specified object's data.
func (c *Client) List(containerName, prefix, delim, marker string, limit int) (contents []ContainerContents, err error) {
	params := make(url.Values)
	params.Add("prefix", prefix)
	params.Add("delimiter", delim)
	params.Add("marker", marker)
	if limit > 0 {
		params.Add("limit", fmt.Sprintf("%d", limit))
	}

	requestData := goosehttp.RequestData{Params: &params, RespValue: &contents}
	url := fmt.Sprintf("/%s", containerName)
	err = c.client.SendRequest(client.GET, "object-store", url, &requestData)
	if err != nil {
		err = errors.Newf(errors.UnspecifiedError, err, nil, "failed to list contents of container: %s", containerName)
	}
	return
}

// URL returns a non-signed URL that allows retrieving the object at path.
// It only works if the object is publicly readable (see SignedURL).
func (c *Client) URL(containerName, file string) (string, error) {
	return c.client.MakeServiceURL("object-store", []string{containerName, file})
}

// SignedURL returns a signed URL that allows anyone holding the URL
// to retrieve the object at path. The signature is valid until expires.
func (c *Client) SignedURL(containerName, file string, expires time.Time) (string, error) {
	// expiresUnix := expires.Unix()
	// TODO(wallyworld) - retrieve the signed URL, for now just return the public one
	rawURL, err := c.URL(containerName, file)
	if err != nil {
		return "", err
	}
	return rawURL, nil
}
