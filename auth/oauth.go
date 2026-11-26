package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const DriveScope = "https://www.googleapis.com/auth/drive"

// callbackServer is a one-shot local http server that receives the OAuth
// authorization response on a loopback redirect URI
// (http://127.0.0.1:<port>/). It replaces the out-of-band (OOB) flow,
// which was shut down by Google in early 2023.
type callbackServer struct {
	listener net.Listener
	server   *http.Server
	state    string
	code     chan string
	failure  chan error
}

func newCallbackServer() (*callbackServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("Failed to start local callback server: %s", err)
	}

	state, err := randomState()
	if err != nil {
		listener.Close()
		return nil, err
	}

	cs := &callbackServer{
		listener: listener,
		state:    state,
		code:     make(chan string, 1),
		failure:  make(chan error, 1),
	}
	cs.server = &http.Server{Handler: http.HandlerFunc(cs.handle)}

	// The server is closed by wait(), the returned error can be ignored
	go cs.server.Serve(listener)

	return cs, nil
}

// redirectURL returns the loopback URI the OAuth response is redirected to
func (self *callbackServer) redirectURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d/", self.listener.Addr().(*net.TCPAddr).Port)
}

// wait blocks until the authorization response is received, an error
// occurs or the given timeout elapses
func (self *callbackServer) wait(timeout time.Duration) (string, error) {
	defer self.server.Close()

	select {
	case code := <-self.code:
		return code, nil
	case err := <-self.failure:
		return "", err
	case <-time.After(timeout):
		return "", fmt.Errorf("Timed out waiting for authorization response")
	}
}

func (self *callbackServer) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	query := r.URL.Query()

	if errDesc := query.Get("error"); errDesc != "" {
		fmt.Fprintf(w, "Authorization failed: %s", errDesc)
		self.failure <- fmt.Errorf("Authorization failed: %s", errDesc)
		return
	}

	if state := query.Get("state"); state != self.state {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		self.failure <- fmt.Errorf("Received invalid state parameter")
		return
	}

	code := query.Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		self.failure <- fmt.Errorf("Authorization response was missing the code parameter")
		return
	}

	fmt.Fprint(w, "gdrive authorized successfully. You can close this tab and return to your terminal.")
	self.code <- code
}

func randomState() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("Failed to generate state parameter: %s", err)
	}
	return hex.EncodeToString(buf), nil
}

// NewFileSourceClient returns an authenticated http client which uses
// the given token file to cache credentials between runs
func NewFileSourceClient(clientId, clientSecret, tokenFile string) (*http.Client, error) {
	// Read cached token
	token, exists, err := ReadToken(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to read token: %s", err)
	}

	// Run the authorization flow if the token file does not exist
	// or the refresh token is missing
	if !exists || token.RefreshToken == "" {
		token, err = authorize(clientId, clientSecret)
		if err != nil {
			return nil, err
		}

		// Save the token right away so the user does not have to
		// re-authorize if the command fails or is interrupted
		if err := SaveToken(tokenFile, token); err != nil {
			return nil, err
		}
	}

	conf := getConfig(clientId, clientSecret, "")
	return oauth2.NewClient(
		context.Background(),
		FileSource(tokenFile, token, conf),
	), nil
}

// authorize runs the OAuth authorization flow by starting a local
// callback server and directing the user to the authorization url
func authorize(clientId, clientSecret string) (*oauth2.Token, error) {
	callback, err := newCallbackServer()
	if err != nil {
		return nil, err
	}

	conf := getConfig(clientId, clientSecret, callback.redirectURL())

	// Prompt for consent to make sure the response contains a refresh token
	authUrl := conf.AuthCodeURL(callback.state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	fmt.Println("Authentication needed")
	fmt.Println("Go to the following url in your browser:")
	fmt.Printf("\n%s\n\n", authUrl)
	fmt.Println("Waiting for authorization response...")

	code, err := callback.wait(10 * time.Minute)
	if err != nil {
		return nil, err
	}

	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("Failed to exchange authorization code: %s", err)
	}

	return token, nil
}

// NewRefreshTokenClient returns an http client which uses the given
// refresh token to retrieve access tokens
func NewRefreshTokenClient(clientId, clientSecret, refreshToken string) *http.Client {
	conf := getConfig(clientId, clientSecret, "")
	token := &oauth2.Token{
		TokenType:    "Bearer",
		RefreshToken: refreshToken,
		Expiry:       time.Now(),
	}

	return oauth2.NewClient(
		context.Background(),
		conf.TokenSource(context.Background(), token),
	)
}

// NewAccessTokenClient returns an http client which uses the given access token
func NewAccessTokenClient(clientId, clientSecret, accessToken string) *http.Client {
	conf := getConfig(clientId, clientSecret, "")
	token := &oauth2.Token{
		TokenType:   "Bearer",
		AccessToken: accessToken,
	}

	return oauth2.NewClient(
		context.Background(),
		conf.TokenSource(context.Background(), token),
	)
}

// NewServiceAccountClient returns an http client which authenticates
// with the given service account file
func NewServiceAccountClient(serviceAccountFile string) (*http.Client, error) {
	content, exists, err := ReadFile(serviceAccountFile)
	if !exists {
		return nil, fmt.Errorf("Service account filename %q not found", serviceAccountFile)
	}

	if err != nil {
		return nil, err
	}

	conf, err := google.JWTConfigFromJSON(content, DriveScope)
	if err != nil {
		return nil, err
	}
	return conf.Client(context.Background()), nil
}

func getConfig(clientId, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		Scopes:       []string{DriveScope},
		RedirectURL:  redirectURL,
		Endpoint:     google.Endpoint,
	}
}
