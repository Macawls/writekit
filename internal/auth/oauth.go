package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	googleOAuth "golang.org/x/oauth2/google"
)

type OAuthProvider struct {
	Config *oauth2.Config
	Name   string
}

func NewGoogleProvider(clientID, clientSecret, redirectURL string) *OAuthProvider {
	return &OAuthProvider{
		Name: "google",
		Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     googleOAuth.Endpoint,
			Scopes:       []string{"openid", "email", "profile"},
		},
	}
}

func NewGithubProvider(clientID, clientSecret, redirectURL string) *OAuthProvider {
	return &OAuthProvider{
		Name: "github",
		Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     github.Endpoint,
			Scopes:       []string{"user:email"},
		},
	}
}

func NewDiscordProvider(clientID, clientSecret, redirectURL string) *OAuthProvider {
	return &OAuthProvider{
		Name: "discord",
		Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://discord.com/api/oauth2/authorize",
				TokenURL: "https://discord.com/api/oauth2/token",
			},
			Scopes: []string{"identify", "email"},
		},
	}
}

type OAuthUserInfo struct {
	Email         string
	Name          string
	AvatarURL     string
	ID            string
	EmailVerified bool
}

func (p *OAuthProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.Config.Exchange(ctx, code)
}

func (p *OAuthProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error) {
	client := p.Config.Client(ctx, token)

	switch p.Name {
	case "google":
		return getGoogleUserInfo(client)
	case "github":
		return getGithubUserInfo(client)
	case "discord":
		return getDiscordUserInfo(client)
	default:
		return nil, fmt.Errorf("unknown provider: %s", p.Name)
	}
}

func getGoogleUserInfo(client *http.Client) (*OAuthUserInfo, error) {
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	var data struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	return &OAuthUserInfo{
		Email:         data.Email,
		Name:          data.Name,
		AvatarURL:     data.Picture,
		ID:            data.ID,
		EmailVerified: true,
	}, nil
}

func getGithubUserInfo(client *http.Client) (*OAuthUserInfo, error) {
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	var data struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	name := data.Name
	if name == "" {
		name = data.Login
	}

	email := data.Email
	if email == "" {
		email, _ = getGithubPrimaryEmail(client)
	}

	return &OAuthUserInfo{
		Email:         email,
		Name:          name,
		AvatarURL:     data.AvatarURL,
		ID:            fmt.Sprintf("%d", data.ID),
		EmailVerified: email != "",
	}, nil
}

func getGithubPrimaryEmail(client *http.Client) (string, error) {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", err
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	return "", fmt.Errorf("no verified primary email")
}

func getDiscordUserInfo(client *http.Client) (*OAuthUserInfo, error) {
	resp, err := client.Get("https://discord.com/api/users/@me")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	var data struct {
		ID            string `json:"id"`
		Username      string `json:"username"`
		GlobalName    string `json:"global_name"`
		Email         string `json:"email"`
		Verified      bool   `json:"verified"`
		Avatar        string `json:"avatar"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	name := data.GlobalName
	if name == "" {
		name = data.Username
	}

	var avatarURL string
	if data.Avatar != "" {
		avatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", data.ID, data.Avatar)
	}

	return &OAuthUserInfo{
		Email:         data.Email,
		Name:          name,
		AvatarURL:     avatarURL,
		ID:            data.ID,
		EmailVerified: data.Verified,
	}, nil
}
