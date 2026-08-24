// Package atproto is a minimal AT Protocol client covering just what hinoki
// needs: session creation and record CRUD against a PDS.
package atproto

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	PDS        string
	HTTPClient *http.Client

	AccessJwt string
	DID       string
}

func NewClient(pds string) *Client {
	return &Client{
		PDS:        strings.TrimRight(pds, "/"),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type sessionResponse struct {
	AccessJwt  string `json:"accessJwt"`
	RefreshJwt string `json:"refreshJwt"`
	DID        string `json:"did"`
}

// CreateSession authenticates with an identifier (handle or DID) and an app
// password, populating c.AccessJwt and c.DID on success.
func (c *Client) CreateSession(identifier, appPassword string) error {
	body := map[string]string{
		"identifier": identifier,
		"password":   appPassword,
	}
	var resp sessionResponse
	if err := c.post("com.atproto.server.createSession", body, &resp); err != nil {
		return fmt.Errorf("creating session: %w", err)
	}
	c.AccessJwt = resp.AccessJwt
	c.DID = resp.DID
	return nil
}

// CreateRecord creates a new record in the given collection. If rkey is
// empty, the PDS assigns one (returned in the result URI's final segment).
func (c *Client) CreateRecord(collection, rkey string, record any) (uri, cid string, err error) {
	body := map[string]any{
		"repo":       c.DID,
		"collection": collection,
		"record":     record,
	}
	if rkey != "" {
		body["rkey"] = rkey
	}
	var resp struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	}
	if err := c.authedPost("com.atproto.repo.createRecord", body, &resp); err != nil {
		return "", "", err
	}
	return resp.URI, resp.CID, nil
}

// PutRecord creates or overwrites a record at a known rkey.
func (c *Client) PutRecord(collection, rkey string, record any) (uri, cid string, err error) {
	body := map[string]any{
		"repo":       c.DID,
		"collection": collection,
		"rkey":       rkey,
		"record":     record,
	}
	var resp struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	}
	if err := c.authedPost("com.atproto.repo.putRecord", body, &resp); err != nil {
		return "", "", err
	}
	return resp.URI, resp.CID, nil
}

// DeleteRecord deletes a record at the given rkey.
func (c *Client) DeleteRecord(collection, rkey string) error {
	body := map[string]any{
		"repo":       c.DID,
		"collection": collection,
		"rkey":       rkey,
	}
	return c.authedPost("com.atproto.repo.deleteRecord", body, nil)
}

// GetRecord fetches a record; ok is false if it does not exist.
func (c *Client) GetRecord(collection, rkey string, out any) (ok bool, err error) {
	url := fmt.Sprintf("%s/xrpc/com.atproto.repo.getRecord?repo=%s&collection=%s&rkey=%s",
		c.PDS, c.DID, collection, rkey)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.AccessJwt)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("getRecord failed: %s: %s", resp.Status, data)
	}

	var wrapper struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return false, err
	}
	if err := json.Unmarshal(wrapper.Value, out); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Client) post(path string, body, out any) error {
	return c.doRequest(path, body, out, false)
}

func (c *Client) authedPost(path string, body, out any) error {
	return c.doRequest(path, body, out, true)
}

func (c *Client) doRequest(path string, body, out any, authed bool) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.PDS+"/xrpc/"+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if authed {
		if c.AccessJwt == "" {
			return fmt.Errorf("not authenticated")
		}
		req.Header.Set("Authorization", "Bearer "+c.AccessJwt)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s failed: %s: %s", path, resp.Status, data)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return err
		}
	}
	return nil
}

// NewTID generates an AT Protocol timestamp identifier suitable for use as
// an rkey, matching the format used by the reference implementations.
func NewTID() string {
	const alphabet = "234567abcdefghijklmnopqrstuvwxyz"
	micros := time.Now().UnixMicro()
	clockID, _ := rand.Int(rand.Reader, big.NewInt(1<<10))
	val := (uint64(micros) << 10) | clockID.Uint64()

	buf := make([]byte, 13)
	for i := 12; i >= 0; i-- {
		buf[i] = alphabet[val&0x1f]
		val >>= 5
	}
	return string(buf)
}
