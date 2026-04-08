package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	cloudServer  string
	cloudToken   string
	cloudProfile string
)

func cloudCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cloud",
		Short: "Interact with a remote stroppy-cloud server",
	}

	cmd.PersistentFlags().StringVar(&cloudServer, "server", "", "server URL (env: STROPPY_SERVER)")
	cmd.PersistentFlags().StringVar(&cloudToken, "token", "", "API or access token (env: STROPPY_TOKEN)")
	cmd.PersistentFlags().StringVar(&cloudProfile, "profile", "", "credentials profile (env: STROPPY_PROFILE)")

	cmd.AddCommand(
		cloudLoginCmd(),
		cloudLogoutCmd(),
		cloudStatusCmd(),
		cloudTenantsCmd(),
		cloudUseCmd(),
		cloudProfilesCmd(),
		cloudUseProfileCmd(),
		cloudRunCmd(),
		cloudCompareCmd(),
		cloudBenchCmd(),
		cloudUploadCmd(),
		cloudWaitCmd(),
		cloudPackagesCmd(),
		cloudProbeCmd(),
		cloudPresetsCmd(),
	)

	return cmd
}

type cloudHTTPClient struct {
	server      string
	token       string
	creds       *Credentials
	profileName string
}

// resolveProfile returns the active profile name from --profile flag, env, or credentials default.
func resolveProfile(creds *Credentials) string {
	if cloudProfile != "" {
		return cloudProfile
	}
	if p := os.Getenv("STROPPY_PROFILE"); p != "" {
		return p
	}
	return creds.Current
}

func newCloudClient() (*cloudHTTPClient, error) {
	creds, err := loadCredentials()
	if err != nil {
		return nil, fmt.Errorf("load credentials: %w", err)
	}

	pname := resolveProfile(creds)
	c := &cloudHTTPClient{creds: creds, profileName: pname}

	prof := creds.Profiles[pname]

	c.server = cloudServer
	if c.server == "" {
		c.server = os.Getenv("STROPPY_SERVER")
	}
	if c.server == "" && prof != nil {
		c.server = prof.Server
	}
	if c.server == "" {
		c.server = "http://localhost:8080"
	}
	c.server = strings.TrimRight(c.server, "/")

	c.token = cloudToken
	if c.token == "" {
		c.token = os.Getenv("STROPPY_TOKEN")
	}
	if c.token == "" && prof != nil {
		c.token = prof.AccessToken
	}

	return c, nil
}

func (c *cloudHTTPClient) ensureValidToken() error {
	if cloudToken != "" || os.Getenv("STROPPY_TOKEN") != "" {
		return nil
	}

	p := c.creds.Profiles[c.profileName]
	if p == nil || p.RefreshToken == "" {
		return nil
	}

	if !isJWTExpired(c.token) {
		return nil
	}

	req, _ := http.NewRequest("POST", c.server+"/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: p.RefreshToken})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("session expired, please run: stroppy-cloud cloud login")
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("parse refresh response: %w", err)
	}

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "refresh_token" {
			p.RefreshToken = cookie.Value
			break
		}
	}

	c.token = result.AccessToken
	p.AccessToken = result.AccessToken
	_ = c.creds.Save()

	return nil
}

func (c *cloudHTTPClient) do(req *http.Request) (*http.Response, error) {
	if err := c.ensureValidToken(); err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return http.DefaultClient.Do(req)
}

func (c *cloudHTTPClient) doJSON(method, path string, body io.Reader) ([]byte, int, error) {
	req, err := http.NewRequest(method, c.server+path, body)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return data, resp.StatusCode, nil
}

func (c *cloudHTTPClient) url(path string) string {
	return c.server + path
}

func isJWTExpired(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return true
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return true
	}
	return time.Now().Unix() > claims.Exp-30
}
