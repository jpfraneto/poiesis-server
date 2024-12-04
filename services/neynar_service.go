package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"context"

	"github.com/ankylat/anky/server/types"
	"github.com/joho/godotenv"
)

type NeynarService struct {
	apiKey string
}

type NeynarResponse struct {
	Casts []Cast `json:"casts"`
	Next  struct {
		Cursor string `json:"cursor"`
	} `json:"next"`
}

type Cast struct {
	Object            string        `json:"object"`
	Hash              string        `json:"hash"`
	ThreadHash        string        `json:"thread_hash"`
	ParentHash        *string       `json:"parent_hash"`
	ParentURL         *string       `json:"parent_url"`
	RootParentURL     *string       `json:"root_parent_url"`
	ParentAuthor      ParentAuthor  `json:"parent_author"`
	Author            Author        `json:"author"`
	Text              string        `json:"text"`
	Timestamp         string        `json:"timestamp"`
	Embeds            []Embed       `json:"embeds"`
	Reactions         Reactions     `json:"reactions"`
	Replies           Replies       `json:"replies"`
	Channel           *Channel      `json:"channel"`
	MentionedProfiles []interface{} `json:"mentioned_profiles"`
	ViewerContext     ViewerContext `json:"viewer_context"`
}

type ParentAuthor struct {
	Fid *int `json:"fid"`
}

type Author struct {
	Object            string            `json:"object"`
	Fid               int               `json:"fid"`
	CustodyAddress    string            `json:"custody_address"`
	Username          string            `json:"username"`
	DisplayName       string            `json:"display_name"`
	PfpURL            string            `json:"pfp_url"`
	Profile           Profile           `json:"profile"`
	FollowerCount     int               `json:"follower_count"`
	FollowingCount    int               `json:"following_count"`
	Verifications     []string          `json:"verifications"`
	VerifiedAddresses VerifiedAddresses `json:"verified_addresses"`
	ActiveStatus      string            `json:"active_status"`
	PowerBadge        bool              `json:"power_badge"`
	ViewerContext     ViewerContext     `json:"viewer_context"`
}

type Profile struct {
	Bio Bio `json:"bio"`
}

type Bio struct {
	Text string `json:"text"`
}

type VerifiedAddresses struct {
	EthAddresses []string `json:"eth_addresses"`
	SolAddresses []string `json:"sol_addresses"`
}

type Embed struct {
	URL      string   `json:"url"`
	Metadata Metadata `json:"metadata"`
}

type Metadata struct {
	ContentType   string `json:"content_type"`
	ContentLength int    `json:"content_length"`
	Status        string `json:"_status"`
	Image         *Image `json:"image,omitempty"`
	HTML          *HTML  `json:"html,omitempty"`
}

type Image struct {
	WidthPx  int `json:"width_px"`
	HeightPx int `json:"height_px"`
}

type HTML struct {
	Charset  string    `json:"charset"`
	OgImage  []OgImage `json:"ogImage"`
	OgTitle  string    `json:"ogTitle"`
	OgLocale string    `json:"ogLocale"`
}

type OgImage struct {
	URL  string `json:"url"`
	Type string `json:"type"`
}

type Reactions struct {
	LikesCount   int           `json:"likes_count"`
	RecastsCount int           `json:"recasts_count"`
	Likes        []Like        `json:"likes"`
	Recasts      []interface{} `json:"recasts"`
}

type Like struct {
	Fid   int    `json:"fid"`
	Fname string `json:"fname"`
}

type Replies struct {
	Count int `json:"count"`
}

type Channel struct {
	Object   string `json:"object"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	ImageURL string `json:"image_url"`
}

type ViewerContext struct {
	Following  bool `json:"following"`
	FollowedBy bool `json:"followed_by"`
	Blocking   bool `json:"blocking"`
	BlockedBy  bool `json:"blocked_by"`
	Liked      bool `json:"liked"`
	Recasted   bool `json:"recasted"`
}

func NewNeynarService() *NeynarService {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
	}

	apiKey := os.Getenv("NEYNAR_API_KEY")
	if apiKey == "" {
		log.Printf("Warning: NEYNAR_API_KEY not found in environment variables")
	} else {
		log.Printf("Initializing NeynarService with API key: %s", apiKey)
	}
	return &NeynarService{
		apiKey: apiKey,
	}
}

func (s *NeynarService) FetchUserCasts(fid int) ([]Cast, error) {
	url := fmt.Sprintf("https://api.neynar.com/v2/farcaster/feed/user/casts?fid=%d&viewer_fid=16098&limit=5&include_replies=false", fid)
	log.Printf("Fetching casts for FID %d from URL: %s", fid, url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return nil, err
	}
	req.Header.Add("accept", "application/json")
	req.Header.Add("api_key", s.apiKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error sending request: %v", err)
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, err
	}

	log.Printf("Received response: %s", string(body))

	var neynarResponse NeynarResponse
	err = json.Unmarshal(body, &neynarResponse)
	if err != nil {
		log.Printf("Error unmarshaling response: %v", err)
		return nil, err
	}

	return neynarResponse.Casts, nil
}

func (s *NeynarService) WriteCast(apiKey, signerUUID, cast_text, channelID, idem, sessionId string) (*types.Cast, error) {
	log.Println("Starting WriteCast function")

	url := "https://api.neynar.com/v2/farcaster/cast"
	log.Printf("URL: %s", url)

	payload := map[string]interface{}{
		"signer_uuid": signerUUID,
		"text":        cast_text,
		"channel_id":  channelID,
		"idem":        idem,
		"embeds": []map[string]string{
			{
				"url": fmt.Sprintf("https://farcaster.anky.bot/anky/%s", sessionId),
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling payload: %v", err)
		return nil, fmt.Errorf("error marshaling payload: %v", err)
	}
	log.Printf("Payload: %s", string(payloadBytes))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	log.Println("Request created successfully")

	req.Header.Add("accept", "application/json")
	req.Header.Add("api_key", apiKey)
	req.Header.Add("content-type", "application/json")
	log.Printf("Request headers: %v", req.Header)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error sending request: %v", err)
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer res.Body.Close()
	log.Println("Request sent successfully")

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, fmt.Errorf("error reading response body: %v", err)
	}
	log.Printf("Response body read successfully: %s", string(body))

	log.Printf("Response status code: %d", res.StatusCode)
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", res.StatusCode, string(body))
	}

	var response struct {
		Cast *types.Cast `json:"cast"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		log.Printf("Error unmarshaling response: %v", err)
		return nil, fmt.Errorf("error unmarshaling response: %v", err)
	}

	log.Printf("Successfully wrote cast. Response: %s", string(body))
	return response.Cast, nil
}

func (s *NeynarService) CreateNewFid(ctx context.Context) (int, error) {
	url := "https://farcaster.anky.bot/create-new-fid"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("ANKY_API_KEY", s.apiKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error sending request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return 0, fmt.Errorf("unexpected status code: %d, body: %s", res.StatusCode, string(body))
	}

	var response struct {
		Fid int `json:"fid"`
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response body: %v", err)
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return 0, fmt.Errorf("error unmarshaling response: %v", err)
	}

	return response.Fid, nil
}

func (s *NeynarService) LinkAnkyWithFid(ctx context.Context, ankyID string, fid int) error {
	url := fmt.Sprintf("https://farcaster.anky.bot/link-anky/%s/%d", ankyID, fid)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("ANKY_API_KEY", s.apiKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", res.StatusCode, string(body))
	}

	return nil
}
