package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// ErrorResponse matchar serverns felformat
type ErrorResponse struct {
	Error string `json:"error"`
}

// SuccessResponse matchar serverns framgångsformat
type SuccessResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Client hanterar kommunikationen med LocalLedger API:et.
type Client struct {
	token string
	url   string
}

// NewClient letar upp session_token och skapar en ny klient.
func NewClient(workspace string, port int) (*Client, error) {
	tokenPath := filepath.Join(workspace, ".session_token")
	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("LocalLedger server is not running (or .session_token is missing).\nPlease start the server first by running 'localledger serve'.")
		}
		return nil, fmt.Errorf("failed to read session token: %w", err)
	}

	// Om port är 0, försök läsa från .server_port
	if port == 0 {
		portPath := filepath.Join(workspace, ".server_port")
		portBytes, err := os.ReadFile(portPath)
		if err == nil {
			fmt.Sscanf(string(portBytes), "%d", &port)
		} else {
			port = 8080 // Fallback
		}
	}

	return &Client{
		token: string(tokenBytes),
		url:   fmt.Sprintf("http://127.0.0.1:%d", port),
	}, nil
}

// Verify anropar GET /api/verify och returnerar resultatet.
func (c *Client) Verify() error {
	req, err := http.NewRequest(http.MethodGet, c.url+"/api/verify", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("network error. Is the server running? Details: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		var succResp SuccessResponse
		if err := json.Unmarshal(bodyBytes, &succResp); err == nil && succResp.Message != "" {
			fmt.Printf("✅ %s\n", succResp.Message)
		} else {
			fmt.Println("✅ WORM chain is intact and valid.")
		}
		return nil
	}

	// Läs felmeddelande från servern
	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Error != "" {
		return fmt.Errorf("API Error (%d): %s", resp.StatusCode, errResp.Error)
	}

	return fmt.Errorf("unexpected API error: %d - %s", resp.StatusCode, string(bodyBytes))
}
