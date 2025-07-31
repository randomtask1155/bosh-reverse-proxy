package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var (
	oauthAPI      = "https://%s:8443/oauth/token"
	deploymentAPI = "https://%s:25555/deployments"
	InstancesAPI  = "https://%s:25555/deployments/%s/instances" // insert deployment name
)

type BoshClient struct {
	ClientID       string
	ClientPassword string
	Hostname       string
	Token          string
	Client         *http.Client
}

func NewBoshClient(id, pass, host string) (BoshClient, error) {
	b := BoshClient{id, pass, host, "", &http.Client{Transport: BRPDefaultTransport}}

	formData := url.Values{}
	formData.Add("client_id", id)
	formData.Add("client_secret", pass)
	formData.Add("grant_type", "client_credentials")
	encodedData := formData.Encode()

	req, err := http.NewRequest("POST", fmt.Sprintf(oauthAPI, host), strings.NewReader(encodedData))
	if err != nil {
		return b, fmt.Errorf("error creating login request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := b.Client.Do(req)
	if err != nil {
		return b, fmt.Errorf("error issuing login request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return b, fmt.Errorf("error reading login response body: %v", err)
	}
	rt := BoshAuthResponse{}
	err = json.Unmarshal(body, &rt)
	if err != nil {
		return b, fmt.Errorf("error decoding login response body: %s decode-error: %v", body, err)
	}
	b.Token = rt.AccessToken
	return b, nil
}

type BoshAuthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIN   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	JTI         string `json:"jti"`
}

type BoshInstances struct {
	Deployments []Deployment
}

type Deployment struct {
	Name      string `json:"name"`
	Instances []Instances
}

type Instances struct {
	ID      string   `json:"id"`
	AgentID string   `json:"agent_id"`
	Job     string   `json:"job"`
	Index   int      `json:"index"`
	IPs     []string `json:"ips"`
}

func (b *BoshClient) GetRequest(url string) ([]byte, error) {
	var body []byte
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Error("error creating request", "error", err)
		return body, fmt.Errorf("%s: %v", url, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.Token)

	resp, err := b.Client.Do(req)
	if err != nil {
		return body, fmt.Errorf("%s: %v", url, err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (b *BoshClient) GetInstances() (BoshInstances, error) {
	bi := BoshInstances{}

	body, err := b.GetRequest(fmt.Sprintf(deploymentAPI, b.Hostname))
	if err != nil {
		return bi, err
	}
	bi.Deployments = make([]Deployment, 0)
	err = json.Unmarshal(body, &bi.Deployments)
	if err != nil {
		return bi, fmt.Errorf("error decoding deployment response body: %v", err)
	}

	for i, d := range bi.Deployments {
		logger.Debug("processing deployment", "name", d.Name)
		bi.Deployments[i].Instances, err = b.UpdateInstaces(d.Name)
		if err != nil {
			logger.Error("failed to get instances for deployment %s: %v", "deployment", d.Name, "error", err)
		}
	}
	return bi, nil
}

func (b *BoshClient) UpdateInstaces(name string) ([]Instances, error) {
	is := make([]Instances, 0)

	body, err := b.GetRequest(fmt.Sprintf(InstancesAPI, b.Hostname, name))
	if err != nil {
		return is, err
	}
	err = json.Unmarshal(body, &is)
	return is, err
}
