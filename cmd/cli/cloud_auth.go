package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// prompt reads a line of input from stdin, displaying msg as the prompt.
func prompt(msg string) string {
	fmt.Print(msg)
	s := bufio.NewScanner(os.Stdin)
	s.Scan()
	return strings.TrimSpace(s.Text())
}

type meResponse struct {
	UserID   string     `json:"user_id"`
	Username string     `json:"username"`
	IsRoot   bool       `json:"is_root"`
	TenantID string     `json:"tenant_id"`
	Role     string     `json:"role"`
	Tenants  []meTenant `json:"tenants"`
}

type meTenant struct {
	ID         string `json:"id"`
	TenantName string `json:"tenant_name"`
	Role       string `json:"role"`
}

func cloudLoginCmd() *cobra.Command {
	var username, password string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with a stroppy-cloud server",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve server
			server := cloudServer
			if server == "" {
				server = os.Getenv("STROPPY_SERVER")
			}
			if server == "" {
				server = prompt("Server URL [http://localhost:8080]: ")
				if server == "" {
					server = "http://localhost:8080"
				}
			}
			server = strings.TrimRight(server, "/")

			// Resolve username / password
			if username == "" {
				username = prompt("Username: ")
			}
			if password == "" {
				fmt.Print("Password: ")
				pw, err := term.ReadPassword(int(syscall.Stdin))
				fmt.Println()
				if err != nil {
					return fmt.Errorf("read password: %w", err)
				}
				password = string(pw)
			}

			// 1. POST /api/v1/auth/login
			loginBody, _ := json.Marshal(map[string]string{
				"username": username,
				"password": password,
			})
			resp, err := http.Post(server+"/api/v1/auth/login", "application/json", bytes.NewReader(loginBody))
			if err != nil {
				return fmt.Errorf("login request failed: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("login failed (HTTP %d)", resp.StatusCode)
			}

			var loginResp struct {
				AccessToken string `json:"access_token"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
				return fmt.Errorf("parse login response: %w", err)
			}

			refreshToken := ""
			for _, c := range resp.Cookies() {
				if c.Name == "refresh_token" {
					refreshToken = c.Value
					break
				}
			}

			accessToken := loginResp.AccessToken

			// 2. GET /api/v1/auth/me
			meReq, _ := http.NewRequest("GET", server+"/api/v1/auth/me", nil)
			meReq.Header.Set("Authorization", "Bearer "+accessToken)
			meResp, err := http.DefaultClient.Do(meReq)
			if err != nil {
				return fmt.Errorf("me request failed: %w", err)
			}
			defer meResp.Body.Close()

			if meResp.StatusCode != http.StatusOK {
				return fmt.Errorf("me request failed (HTTP %d)", meResp.StatusCode)
			}

			var me meResponse
			if err := json.NewDecoder(meResp.Body).Decode(&me); err != nil {
				return fmt.Errorf("parse me response: %w", err)
			}

			tenantName := ""
			role := me.Role
			selectedTenantID := me.TenantID

			// 3. If multiple tenants and none pre-selected, prompt
			if len(me.Tenants) > 1 && me.TenantID == "" {
				fmt.Println("Available tenants:")
				for i, t := range me.Tenants {
					fmt.Printf("  [%d] %s (%s)\n", i+1, t.TenantName, t.Role)
				}
				choice := prompt("Select tenant [1]: ")
				idx := 0
				if choice != "" {
					n, err := strconv.Atoi(choice)
					if err != nil || n < 1 || n > len(me.Tenants) {
						return fmt.Errorf("invalid tenant selection: %s", choice)
					}
					idx = n - 1
				}
				selectedTenantID = me.Tenants[idx].ID
				tenantName = me.Tenants[idx].TenantName
				role = me.Tenants[idx].Role
			}

			// Resolve tenant name from list if we have a tenant ID but no name yet
			if tenantName == "" && selectedTenantID != "" {
				for _, t := range me.Tenants {
					if t.ID == selectedTenantID {
						tenantName = t.TenantName
						role = t.Role
						break
					}
				}
			}
			if tenantName == "" && len(me.Tenants) == 1 {
				selectedTenantID = me.Tenants[0].ID
				tenantName = me.Tenants[0].TenantName
				role = me.Tenants[0].Role
			}

			// 4. Select tenant if needed
			if selectedTenantID != "" && selectedTenantID != me.TenantID {
				selBody, _ := json.Marshal(map[string]string{"tenant_id": selectedTenantID})
				selReq, _ := http.NewRequest("POST", server+"/api/v1/auth/select-tenant", bytes.NewReader(selBody))
				selReq.Header.Set("Authorization", "Bearer "+accessToken)
				selReq.Header.Set("Content-Type", "application/json")
				selResp, err := http.DefaultClient.Do(selReq)
				if err != nil {
					return fmt.Errorf("select-tenant request failed: %w", err)
				}
				defer selResp.Body.Close()

				if selResp.StatusCode != http.StatusOK {
					return fmt.Errorf("select-tenant failed (HTTP %d)", selResp.StatusCode)
				}

				var selResult struct {
					AccessToken string `json:"access_token"`
				}
				if err := json.NewDecoder(selResp.Body).Decode(&selResult); err != nil {
					return fmt.Errorf("parse select-tenant response: %w", err)
				}
				accessToken = selResult.AccessToken
			}

			// 5. Save credentials
			creds, err := loadCredentials()
			if err != nil {
				return fmt.Errorf("load credentials: %w", err)
			}
			pname := resolveProfile(creds)
			creds.Profiles[pname] = &Profile{
				Server:       server,
				Tenant:       tenantName,
				AccessToken:  accessToken,
				RefreshToken: refreshToken,
			}
			creds.Current = pname
			if err := creds.Save(); err != nil {
				return fmt.Errorf("save credentials: %w", err)
			}

			// 6. Print result
			fmt.Printf("Logged in as %s @ %s (%s)\n", me.Username, tenantName, role)
			fmt.Printf("Profile: %s | Credentials saved to %s\n", pname, credentialsPath())
			return nil
		},
	}

	cmd.Flags().StringVar(&username, "username", "", "login username")
	cmd.Flags().StringVar(&password, "password", "", "login password (omit for interactive prompt)")
	return cmd
}

func cloudLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out from the stroppy-cloud server",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := loadCredentials()
			if err != nil {
				return err
			}

			pname := resolveProfile(creds)
			p := creds.Profiles[pname]
			if p != nil && p.RefreshToken != "" {
				server := p.Server
				if server == "" {
					server = "http://localhost:8080"
				}
				req, _ := http.NewRequest("POST", server+"/api/v1/auth/logout", nil)
				req.AddCookie(&http.Cookie{Name: "refresh_token", Value: p.RefreshToken})
				_, _ = http.DefaultClient.Do(req) //nolint:bodyclose
			}

			delete(creds.Profiles, pname)
			// If we deleted the current profile, reset to "default".
			if creds.Current == pname {
				creds.Current = "default"
			}
			if err := creds.Save(); err != nil {
				return fmt.Errorf("save credentials: %w", err)
			}

			fmt.Printf("Logged out (profile: %s).\n", pname)
			return nil
		},
	}
}

func cloudStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := loadCredentials()
			if err != nil {
				return err
			}

			pname := resolveProfile(creds)
			p := creds.Profiles[pname]
			if p == nil {
				fmt.Printf("Not logged in (profile: %s). Run: stroppy-cloud cloud login\n", pname)
				return nil
			}

			fmt.Printf("Profile:  %s\n", pname)
			fmt.Printf("Server:   %s\n", p.Server)
			fmt.Printf("Tenant:   %s\n", p.Tenant)

			tokenStatus := "none"
			if p.AccessToken != "" {
				if isJWTExpired(p.AccessToken) {
					tokenStatus = "expired (will auto-refresh)"
				} else {
					tokenStatus = "valid"
				}
			}
			fmt.Printf("Token:    %s\n", tokenStatus)
			return nil
		},
	}
}

func cloudTenantsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tenants",
		Short: "List available tenants",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newCloudClient()
			if err != nil {
				return err
			}

			data, status, err := c.doJSON("GET", "/api/v1/auth/me", nil)
			if err != nil {
				return err
			}
			if status != http.StatusOK {
				return fmt.Errorf("request failed (HTTP %d): %s", status, string(data))
			}

			var me meResponse
			if err := json.Unmarshal(data, &me); err != nil {
				return fmt.Errorf("parse response: %w", err)
			}

			currentTenant := ""
			if p := c.creds.CurrentProfile(); p != nil {
				currentTenant = p.Tenant
			}

			for _, t := range me.Tenants {
				marker := " "
				if t.TenantName == currentTenant || t.ID == me.TenantID {
					marker = "*"
				}
				fmt.Printf("  %s %s (%s)\n", marker, t.TenantName, t.Role)
			}
			return nil
		},
	}
}

func cloudUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <tenant>",
		Short: "Switch to a different tenant",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tenant := args[0]

			c, err := newCloudClient()
			if err != nil {
				return err
			}

			// First get tenant list to resolve name → ID
			meData, status, err := c.doJSON("GET", "/api/v1/auth/me", nil)
			if err != nil {
				return err
			}
			if status != http.StatusOK {
				return fmt.Errorf("request failed (HTTP %d): %s", status, string(meData))
			}

			var me meResponse
			if err := json.Unmarshal(meData, &me); err != nil {
				return fmt.Errorf("parse response: %w", err)
			}

			// Find tenant by name or ID
			var tenantID, tenantName string
			for _, t := range me.Tenants {
				if t.TenantName == tenant || t.ID == tenant {
					tenantID = t.ID
					tenantName = t.TenantName
					break
				}
			}
			if tenantID == "" {
				return fmt.Errorf("tenant not found: %s", tenant)
			}

			// POST /api/v1/auth/select-tenant
			body, _ := json.Marshal(map[string]string{"tenant_id": tenantID})
			data, status, err := c.doJSON("POST", "/api/v1/auth/select-tenant", bytes.NewReader(body))
			if err != nil {
				return err
			}
			if status != http.StatusOK {
				return fmt.Errorf("select-tenant failed (HTTP %d): %s", status, string(data))
			}

			var result struct {
				AccessToken string `json:"access_token"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("parse response: %w", err)
			}

			// Update credentials
			p := c.creds.Profiles[c.profileName]
			if p == nil {
				return fmt.Errorf("no current profile; please run: stroppy-cloud cloud login")
			}
			p.AccessToken = result.AccessToken
			p.Tenant = tenantName
			if err := c.creds.Save(); err != nil {
				return fmt.Errorf("save credentials: %w", err)
			}

			fmt.Printf("Switched to tenant: %s\n", tenantName)
			return nil
		},
	}
}

func cloudProfilesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "profiles",
		Short: "List saved profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := loadCredentials()
			if err != nil {
				return err
			}
			if len(creds.Profiles) == 0 {
				fmt.Println("No profiles. Run: stroppy-cloud cloud login")
				return nil
			}
			for name, p := range creds.Profiles {
				marker := " "
				if name == creds.Current {
					marker = "*"
				}
				tenant := p.Tenant
				if tenant == "" {
					tenant = "(no tenant)"
				}
				fmt.Printf("  %s %-15s  %s  %s\n", marker, name, p.Server, tenant)
			}
			return nil
		},
	}
}

func cloudUseProfileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use-profile <name>",
		Short: "Switch the default profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			creds, err := loadCredentials()
			if err != nil {
				return err
			}
			if _, ok := creds.Profiles[name]; !ok {
				return fmt.Errorf("profile %q not found", name)
			}
			creds.Current = name
			if err := creds.Save(); err != nil {
				return fmt.Errorf("save credentials: %w", err)
			}
			fmt.Printf("Default profile: %s\n", name)
			return nil
		},
	}
}
