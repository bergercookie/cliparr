// Package jellyfin is a minimal client for the parts of the Jellyfin API that
// Cliparr needs: authenticating users, searching media and proxying streams.
//
// Cliparr never uses an admin API key. Every call is made with the access
// token that Jellyfin issued to the logged-in user, so Jellyfin itself decides
// what that user may see.
package jellyfin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ticksPerMs converts Jellyfin ticks (100ns units) to milliseconds.
const ticksPerMs = 10_000

// ErrUnauthorized is returned when Jellyfin rejects credentials or a token.
var ErrUnauthorized = errors.New("jellyfin: unauthorized")

// ErrNotFound is returned when an item does not exist or is not visible to the user.
var ErrNotFound = errors.New("jellyfin: not found")

// StatusError is an unexpected HTTP status from Jellyfin.
type StatusError struct {
	Method, Path string
	Code         int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("jellyfin %s %s: unexpected status %d", e.Method, e.Path, e.Code)
}

// Client talks to a single Jellyfin server.
type Client struct {
	base     *url.URL
	deviceID string
	version  string
	http     *http.Client
}

// New creates a client. deviceID identifies this Cliparr installation in
// Jellyfin's device list and should stay stable across restarts.
func New(baseURL, deviceID, version string) (*Client, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse jellyfin url: %w", err)
	}
	return &Client{
		base:     base,
		deviceID: deviceID,
		version:  version,
		http:     &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// User is a Jellyfin user.
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Session is the result of a successful login.
type Session struct {
	User  User
	Token string
}

// Item is a movie or episode.
type Item struct {
	ID         string
	Title      string // display title, e.g. "Show S01E02 - Name" or "Movie (1999)"
	Type       string // "Movie" or "Episode"
	RuntimeMs  int64
	SeriesName string
	ResumeMs   int64 // where this user would resume playback, 0 if not started
}

// Device is a running Jellyfin client that Cliparr can tell what to play.
type Device struct {
	ID     string
	Name   string // e.g. "Living Room TV"
	Client string // e.g. "Jellyfin Android TV"
}

// Authenticate logs a user in with their Jellyfin credentials.
func (c *Client) Authenticate(ctx context.Context, username, password string) (Session, error) {
	body, _ := json.Marshal(map[string]string{"Username": username, "Pw": password})
	var resp struct {
		AccessToken string
		User        struct{ Id, Name string }
	}
	if err := c.do(ctx, http.MethodPost, "/Users/AuthenticateByName", "", nil, body, &resp); err != nil {
		return Session{}, err
	}
	return Session{User: User{ID: resp.User.Id, Name: resp.User.Name}, Token: resp.AccessToken}, nil
}

// UserByToken returns the user an access token belongs to. API keys are not
// tied to a user and are rejected, so only a real user's token gets through.
func (c *Client) UserByToken(ctx context.Context, token string) (User, error) {
	var me struct{ Id, Name string }
	err := c.do(ctx, http.MethodGet, "/Users/Me", token, nil, nil, &me)
	var status *StatusError
	if errors.As(err, &status) && status.Code == http.StatusBadRequest {
		// Jellyfin answers 400 when the token has no user, as an API key has not.
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, err
	}
	return User{ID: me.Id, Name: me.Name}, nil
}

// UserByName looks a user up by username (case-insensitive). It needs an
// administrator API key, because regular user tokens cannot list users.
func (c *Client) UserByName(ctx context.Context, apiKey, name string) (User, error) {
	var users []struct{ Id, Name string }
	if err := c.do(ctx, http.MethodGet, "/Users", apiKey, nil, nil, &users); err != nil {
		return User{}, err
	}
	for _, u := range users {
		if strings.EqualFold(u.Name, name) {
			return User{ID: u.Id, Name: u.Name}, nil
		}
	}
	return User{}, ErrNotFound
}

// Search finds movies and episodes whose name matches query.
func (c *Client) Search(ctx context.Context, token, userID, query string, limit int) ([]Item, error) {
	q := url.Values{
		"userId":           {userID},
		"recursive":        {"true"},
		"includeItemTypes": {"Movie,Episode"},
		"searchTerm":       {query},
		"limit":            {strconv.Itoa(limit)},
		"fields":           {"ProductionYear"},
	}
	return c.items(ctx, token, q)
}

// Item fetches one movie or episode as seen by the user.
func (c *Client) Item(ctx context.Context, token, userID, id string) (Item, error) {
	items, err := c.items(ctx, token, url.Values{"userId": {userID}, "ids": {id}})
	if err != nil {
		return Item{}, err
	}
	if len(items) == 0 {
		return Item{}, ErrNotFound
	}
	return items[0], nil
}

func (c *Client) items(ctx context.Context, token string, q url.Values) ([]Item, error) {
	var resp struct {
		Items []struct {
			Id                string
			Name              string
			Type              string
			SeriesName        string
			ParentIndexNumber int
			IndexNumber       int
			ProductionYear    int
			RunTimeTicks      int64
			UserData          struct{ PlaybackPositionTicks int64 }
		}
	}
	if err := c.do(ctx, http.MethodGet, "/Items", token, q, nil, &resp); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(resp.Items))
	for _, r := range resp.Items {
		title := r.Name
		switch {
		case r.Type == "Episode":
			title = fmt.Sprintf("%s S%02dE%02d - %s", r.SeriesName, r.ParentIndexNumber, r.IndexNumber, r.Name)
		case r.ProductionYear != 0:
			title = fmt.Sprintf("%s (%d)", r.Name, r.ProductionYear)
		}
		items = append(items, Item{
			ID: r.Id, Title: title, Type: r.Type, SeriesName: r.SeriesName,
			RuntimeMs: r.RunTimeTicks / ticksPerMs,
			ResumeMs:  r.UserData.PlaybackPositionTicks / ticksPerMs,
		})
	}
	return items, nil
}

// ServerID returns the unique id of the Jellyfin server. Jellyfin's web
// client needs it in links. The endpoint is public, so no login is required.
func (c *Client) ServerID(ctx context.Context) (string, error) {
	var info struct{ Id string }
	if err := c.do(ctx, http.MethodGet, "/System/Info/Public", "", nil, nil, &info); err != nil {
		return "", err
	}
	return info.Id, nil
}

// Devices lists the running Jellyfin clients (TVs, phones, browser tabs) that
// the user is allowed to remote-control.
func (c *Client) Devices(ctx context.Context, token, userID string) ([]Device, error) {
	var sessions []struct {
		Id                    string
		DeviceName            string
		Client                string
		SupportsRemoteControl bool
	}
	query := url.Values{"controllableByUserId": {userID}}
	if err := c.do(ctx, http.MethodGet, "/Sessions", token, query, nil, &sessions); err != nil {
		return nil, err
	}
	devices := []Device{}
	for _, s := range sessions {
		if s.SupportsRemoteControl {
			devices = append(devices, Device{ID: s.Id, Name: s.DeviceName, Client: s.Client})
		}
	}
	return devices, nil
}

// PlayOnDevice makes a running Jellyfin client start playing an item at a
// given position. This is the one way to reach an exact time, because
// Jellyfin's web addresses have no start-time parameter.
func (c *Client) PlayOnDevice(ctx context.Context, token, deviceID, itemID string, startMs int64) error {
	query := url.Values{
		"playCommand":        {"PlayNow"},
		"itemIds":            {itemID},
		"startPositionTicks": {strconv.FormatInt(startMs*ticksPerMs, 10)},
	}
	return c.do(ctx, http.MethodPost, "/Sessions/"+url.PathEscape(deviceID)+"/Playing", token, query, nil, nil)
}

// SetResumePoint stores where the user "left off" in an item, so that
// Jellyfin's Resume button starts playback at that position.
func (c *Client) SetResumePoint(ctx context.Context, token, userID, itemID string, positionMs int64) error {
	body, err := json.Marshal(map[string]int64{"PlaybackPositionTicks": positionMs * ticksPerMs})
	if err != nil {
		return err
	}
	// Jellyfin 10.9 moved this endpoint; fall back to the older address.
	path := "/UserItems/" + url.PathEscape(itemID) + "/UserData"
	err = c.do(ctx, http.MethodPost, path, token, url.Values{"userId": {userID}}, body, nil)
	if errors.Is(err, ErrNotFound) {
		path = "/Users/" + url.PathEscape(userID) + "/Items/" + url.PathEscape(itemID) + "/UserData"
		err = c.do(ctx, http.MethodPost, path, token, nil, body, nil)
	}
	return err
}

// StreamSource returns the address of an item's original video file and the
// Authorization header that unlocks it, for tools such as ffmpeg that read the
// file directly from Jellyfin.
func (c *Client) StreamSource(token, itemID string) (rawURL, authHeader string) {
	u := c.endpoint("/Videos/"+url.PathEscape(itemID)+"/stream", url.Values{"static": {"true"}})
	return u.String(), c.authHeader(token)
}

// ProxyStream forwards the original video file of an item to w. Range
// requests are passed through so browsers can seek.
func (c *Client) ProxyStream(w http.ResponseWriter, r *http.Request, token, id string) {
	c.proxy(w, r, token, "/Videos/"+url.PathEscape(id)+"/stream", url.Values{"static": {"true"}})
}

// ProxyImage forwards the primary poster image of an item to w.
func (c *Client) ProxyImage(w http.ResponseWriter, r *http.Request, token, id string) {
	c.proxy(w, r, token, "/Items/"+url.PathEscape(id)+"/Images/Primary", url.Values{"maxHeight": {"400"}})
}

func (c *Client) proxy(w http.ResponseWriter, r *http.Request, token, path string, query url.Values) {
	target := c.endpoint(path, query)
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			// Start from a clean header set so the caller's Cliparr
			// credentials are never forwarded to Jellyfin.
			out := http.Header{"Authorization": {c.authHeader(token)}}
			for _, h := range []string{"Range", "If-Range", "If-None-Match", "Accept"} {
				if v := pr.In.Header.Get(h); v != "" {
					out.Set(h, v)
				}
			}
			pr.Out.URL = target
			pr.Out.Host = target.Host
			pr.Out.Header = out
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "jellyfin unreachable", http.StatusBadGateway)
		},
	}
	rp.ServeHTTP(w, r)
}

func (c *Client) do(ctx context.Context, method, path, token string, query url.Values, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint(path, query).String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader(token))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("jellyfin request: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return ErrUnauthorized
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode >= 300:
		return &StatusError{Method: method, Path: path, Code: resp.StatusCode}
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 10<<20)).Decode(out); err != nil {
		return fmt.Errorf("decode jellyfin response: %w", err)
	}
	return nil
}

func (c *Client) endpoint(path string, query url.Values) *url.URL {
	u := *c.base
	u.Path = c.base.Path + path
	u.RawQuery = query.Encode()
	return &u
}

func (c *Client) authHeader(token string) string {
	h := fmt.Sprintf(`MediaBrowser Client="Cliparr", Device="Cliparr Server", DeviceId=%q, Version=%q`, c.deviceID, c.version)
	if token != "" {
		h += fmt.Sprintf(`, Token=%q`, token)
	}
	return h
}
