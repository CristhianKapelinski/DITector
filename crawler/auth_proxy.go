package crawler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/NSSL-SJTU/DITector/myutils"
)

// Account represents a Docker Hub account
type Account struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token,omitempty"`
}

// IdentityManager handles rotation of IPs and Accounts
type IdentityManager struct {
	Proxies  []string
	Accounts []*Account
	mu       sync.Mutex
	proxyIdx int
	accIdx   int
}

// LoadIdentities loads proxies and accounts from JSON files
func LoadIdentities(proxyFile, accountFile string) (*IdentityManager, error) {
	im := &IdentityManager{}

	// Load Proxies (one proxy URL per line, e.g. "http://user:pass@host:port")
	if proxyFile != "" {
		data, err := os.ReadFile(proxyFile)
		if err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
				if line = strings.TrimSpace(line); line != "" {
					im.Proxies = append(im.Proxies, line)
				}
			}
			fmt.Printf("Loaded %d proxies from %s\n", len(im.Proxies), proxyFile)
		}
	}

	// Load Accounts
	if accountFile != "" {
		data, err := os.ReadFile(accountFile)
		if err == nil {
			json.Unmarshal(data, &im.Accounts)
			fmt.Printf("Loaded %d accounts\n", len(im.Accounts))
		}
	}

	return im, nil
}

var loginMu sync.Mutex

// LoginDockerHub performs authentication and returns a JWT token
func (im *IdentityManager) LoginDockerHub(acc *Account) error {
	loginMu.Lock()
	defer loginMu.Unlock()

	// Check if another worker already got the token while we were waiting
	if acc.Token != "" {
		return nil
	}

	myutils.Logger.Info(fmt.Sprintf("Attempting login for user: %s", acc.Username))
	
	loginURL := "https://hub.docker.com/v2/users/login/"
	payload, _ := json.Marshal(map[string]string{
		"username": acc.Username,
		"password": acc.Password,
	})

	req, err := http.NewRequest("POST", loginURL, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// Use a clean client for login to avoid proxy issues during auth
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	var res struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}

	acc.Token = res.Token
	myutils.Logger.Info(fmt.Sprintf("Successfully obtained new JWT for %s", acc.Username))
	return nil
}

// GetNextClient returns an http.Client and a valid Token (logins if necessary)
func (im *IdentityManager) GetNextClient() (*http.Client, string) {
	im.mu.Lock()
	defer im.mu.Unlock()

	transport := &http.Transport{}
	
	if len(im.Proxies) > 0 {
		proxyURL, _ := url.Parse(im.Proxies[im.proxyIdx])
		transport.Proxy = http.ProxyURL(proxyURL)
		im.proxyIdx = (im.proxyIdx + 1) % len(im.Proxies)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   20 * time.Second,
	}

	var authToken string
	if len(im.Accounts) > 0 {
		acc := im.Accounts[im.accIdx]
		// Auto-login if token is empty
		if acc.Token == "" {
			err := im.LoginDockerHub(acc)
			if err != nil {
				myutils.Logger.Error(fmt.Sprintf("Auto-login failed for %s: %v", acc.Username, err))
			}
		}
		authToken = acc.Token
		im.accIdx = (im.accIdx + 1) % len(im.Accounts)
	}

	return client, authToken
}
