package webapi

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	authorizeURL = "https://accounts.spotify.com/authorize"
	tokenURL     = "https://accounts.spotify.com/api/token"

	// scopes requested for search, playback control, and library/playlist reads.
	scopes = "user-read-playback-state user-modify-playback-state user-read-currently-playing playlist-read-private playlist-read-collaborative user-library-read user-read-private"
)

// Token holds the OAuth tokens and their expiry. It is persisted to disk so the
// user only authenticates once.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	Scope        string    `json:"scope"`
}

func (t *Token) valid() bool {
	return t != nil && t.AccessToken != "" && time.Now().Before(t.Expiry)
}

// Authenticator runs the Authorization Code + PKCE flow and persists the token.
type Authenticator struct {
	clientID    string
	redirectURI string
	tokenPath   string
	http        *http.Client
}

// NewAuthenticator creates an Authenticator. tokenPath is where the token JSON
// is stored (e.g. ~/.spotify-cli/webapi-token.json).
func NewAuthenticator(clientID, redirectURI, tokenPath string) *Authenticator {
	return &Authenticator{
		clientID:    clientID,
		redirectURI: redirectURI,
		tokenPath:   tokenPath,
		http:        &http.Client{Timeout: 15 * time.Second},
	}
}

// LoadToken reads a previously saved token, or returns nil if none exists.
func (a *Authenticator) LoadToken() *Token {
	data, err := os.ReadFile(a.tokenPath)
	if err != nil {
		return nil
	}
	var t Token
	if err := json.Unmarshal(data, &t); err != nil {
		return nil
	}
	return &t
}

func (a *Authenticator) saveToken(t *Token) error {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.tokenPath, data, 0600)
}

// Login runs the full interactive authorization flow and returns a fresh token.
//
// As with the daemon login, the browser (on Windows) cannot reach the WSL
// loopback callback, so a paste fallback is provided: the user pastes the
// failed 127.0.0.1 callback URL and we extract the authorization code.
func (a *Authenticator) Login() (*Token, error) {
	verifier, err := randomString(64)
	if err != nil {
		return nil, err
	}
	challenge := s256Challenge(verifier)
	state, err := randomString(16)
	if err != nil {
		return nil, err
	}

	authURL := a.authorizeURL(challenge, state)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	srv := a.startCallbackServer(state, codeCh, errCh)
	if srv != nil {
		defer srv.Close()
	}

	printAuthInstructions(authURL)

	// The paste reader blocks on stdin. If the loopback server delivers the code
	// first, this goroutine is still blocked — we must let it finish before
	// returning so it does not compete with the TUI for stdin afterwards.
	doneReading := make(chan struct{})
	go func() {
		readPastedCode(state, codeCh)
		close(doneReading)
	}()

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("timed out waiting for Spotify authorization")
	}

	// Ensure the stdin paste-reader has exited. If the code came from the paste
	// itself, it already has; if it came from the loopback server, prompt the
	// user to press Enter to release the blocked read.
	select {
	case <-doneReading:
	default:
		fmt.Println("\n[✓] Authorization received from browser. Press Enter to continue...")
		<-doneReading
	}

	tok, err := a.exchange(code, verifier)
	if err != nil {
		return nil, err
	}
	if err := a.saveToken(tok); err != nil {
		return nil, fmt.Errorf("saving token: %w", err)
	}
	return tok, nil
}

// Refresh exchanges the refresh token for a new access token and persists it.
func (a *Authenticator) Refresh(t *Token) (*Token, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {t.RefreshToken},
		"client_id":     {a.clientID},
	}
	newTok, err := a.postToken(form)
	if err != nil {
		return nil, err
	}
	// Spotify may omit the refresh token on refresh; keep the old one.
	if newTok.RefreshToken == "" {
		newTok.RefreshToken = t.RefreshToken
	}
	if err := a.saveToken(newTok); err != nil {
		return nil, fmt.Errorf("saving token: %w", err)
	}
	return newTok, nil
}

func (a *Authenticator) authorizeURL(challenge, state string) string {
	q := url.Values{
		"client_id":             {a.clientID},
		"response_type":         {"code"},
		"redirect_uri":          {a.redirectURI},
		"scope":                 {scopes},
		"code_challenge_method": {"S256"},
		"code_challenge":        {challenge},
		"state":                 {state},
	}
	return authorizeURL + "?" + q.Encode()
}

func (a *Authenticator) exchange(code, verifier string) (*Token, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {a.redirectURI},
		"client_id":     {a.clientID},
		"code_verifier": {verifier},
	}
	return a.postToken(form)
}

// postToken posts an x-www-form-urlencoded request to the token endpoint and
// decodes the response into a Token (computing Expiry from expires_in).
func (a *Authenticator) postToken(form url.Values) (*Token, error) {
	resp, err := a.http.PostForm(tokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK || body.AccessToken == "" {
		msg := body.Error
		if body.ErrorDesc != "" {
			msg = body.ErrorDesc
		}
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, msg)
	}

	return &Token{
		AccessToken:  body.AccessToken,
		RefreshToken: body.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(body.ExpiresIn) * time.Second),
		Scope:        body.Scope,
	}, nil
}

// startCallbackServer launches a loopback HTTP server on the redirect URI's
// host:port to capture the OAuth redirect. Returns nil if it cannot bind (the
// paste fallback still works in that case).
func (a *Authenticator) startCallbackServer(state string, codeCh chan<- string, errCh chan<- error) *http.Server {
	u, err := url.Parse(a.redirectURI)
	if err != nil {
		return nil
	}
	mux := http.NewServeMux()
	mux.HandleFunc(u.Path, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			fmt.Fprintf(w, "Authorization failed: %s. You can close this tab.", e)
			select {
			case errCh <- fmt.Errorf("authorization denied: %s", e):
			default:
			}
			return
		}
		if q.Get("state") != state {
			fmt.Fprint(w, "State mismatch. You can close this tab.")
			return
		}
		fmt.Fprint(w, "Login complete. You can close this tab and return to the terminal.")
		select {
		case codeCh <- q.Get("code"):
		default:
		}
	})

	srv := &http.Server{Addr: u.Host, Handler: mux}
	ln, err := net.Listen("tcp", u.Host)
	if err != nil {
		return nil
	}
	go srv.Serve(ln)
	return srv
}

// readPastedCode reads a line from stdin and, if it is a callback URL (or raw
// code), forwards the authorization code. Used as the WSL paste fallback.
func readPastedCode(state string, codeCh chan<- string) {
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return
	}
	code := parseCode(strings.TrimSpace(line), state)
	if code != "" {
		select {
		case codeCh <- code:
		default:
		}
	}
}

// parseCode extracts the authorization code from a pasted callback URL, or
// returns the input unchanged if it is already a bare code.
func parseCode(input, state string) string {
	if input == "" {
		return ""
	}
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		u, err := url.Parse(input)
		if err != nil {
			return ""
		}
		q := u.Query()
		if s := q.Get("state"); s != "" && s != state {
			return ""
		}
		return q.Get("code")
	}
	return input
}

func printAuthInstructions(authURL string) {
	fmt.Println()
	fmt.Println("[i] Spotify Web API login required (for search and library browsing).")
	fmt.Println("  ┌─────────────────────────────────────────────────────────────")
	fmt.Println("  │ 1. Open this link in your browser and authorize the app:")
	fmt.Println("  │")
	fmt.Printf("  │      %s\n", authURL)
	fmt.Println("  │")
	fmt.Println("  │ 2. The browser will then try to open a http://127.0.0.1:8080/")
	fmt.Println("  │    callback page and likely fail to load — that is expected in")
	fmt.Println("  │    WSL. Copy that address from the browser and paste it below.")
	fmt.Println("  └─────────────────────────────────────────────────────────────")
	fmt.Print("  Paste the callback URL here (or wait if the browser succeeded): ")
}

// randomString returns a URL-safe base64 string derived from n random bytes.
func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// s256Challenge computes the PKCE S256 challenge for a verifier.
func s256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
