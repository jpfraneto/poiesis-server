package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ankylat/anky/server/storage"
	"github.com/ankylat/anky/server/types"
	"github.com/ankylat/anky/server/utils"
	"github.com/google/uuid"
	"golang.org/x/exp/rand"
)

// Interfaces in Go serve several important purposes:
//  1. Decoupling: They allow code to depend on behavior rather than concrete implementations,
//     making the system more modular and easier to modify
//  2. Testing: Interfaces make it easy to create mock implementations for testing
//  3. Flexibility: Different implementations can be swapped without changing dependent code
//  4. Composition: Interfaces can be composed to build more complex behaviors
//
// In this case, AnkyServiceInterface defines the contract for Anky-related operations.
// By programming to this interface rather than a concrete implementation:
// - We can easily swap the real implementation with test mocks
// - Other services can depend on the interface without knowing implementation details
// - We have a clear contract of the capabilities required for Anky processing
// - We can potentially have different implementations for different environments
type AnkyServiceInterface interface {
	ProcessAnkyCreation(anky *types.Anky, writingSession *types.WritingSession) error
	GenerateAnkyReflection(session *types.WritingSession) (map[string]string, error)
	GenerateImageWithMidjourney(prompt string) (string, error)
	GenerateFramesgivingNextWritingPrompt(session *utils.WritingSession) (string, error)
	ReflectBackFromWritingSessionConversation(pastSessions []string, sessionLongString string) (string, error)
	ProcessAnkyCreationFromWritingString(ctx context.Context, writing string, sessionID string, userID string) error

	PollImageStatus(id string) (string, error)
	CheckImageStatus(id string) (string, error)
	FetchImageDetails(id string) (*ImageDetails, error)
	PublishToFarcaster(session *types.WritingSession) (*types.Cast, error)
	OnboardingConversation(sessions []*types.WritingSession, ankyReflections []*types.AnkyOnboardingResponse) (string, error)
}

type AnkyService struct {
	store        *storage.PostgresStore
	imageHandler *ImageService
	farcaster    *FarcasterService
}

func NewAnkyService(store *storage.PostgresStore) (*AnkyService, error) {
	imageHandler, err := NewImageService()
	if err != nil {
		return nil, fmt.Errorf("failed to create image handler: %v", err)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create arweave service: %v", err)
	}

	return &AnkyService{
		store:        store,
		imageHandler: imageHandler,
		farcaster:    NewFarcasterService(),
	}, nil
}

func (s *AnkyService) ProcessAnkyCreationFromWritingString(ctx context.Context, writing string, sessionID string, userID string) error {
	fmt.Println("((((((((((((((((((((((((((((((((()))))))))))))))))))))))))))))))))")
	fmt.Println("((((((((((((((((((((((((((((((((()))))))))))))))))))))))))))))))))")
	fmt.Println("((((((((((((((((((((((((((((((((()))))))))))))))))))))))))))))))))")
	fmt.Println("((((((((((((((((hereeeee))))))))))")
	fmt.Println("((((((((((((((((((((((((((((((((()))))))))))))))))))))))))))))))))")
	fmt.Println("((((((((((((((((((((((((((((((((()))))))))))))))))))))))))))))))))")

	anky := &types.Anky{
		Status: "starting_processing",
	}
	s.store.UpdateAnky(ctx, anky)

	// 1. Generate Anky's reflection on the writing

	pinataService, err := NewPinataService()
	if err != nil {
		return err
	}

	writingSessionIPFSHash, err := pinataService.UploadTXTFile(writing)
	if err != nil {
		return err
	}

	anky_processing_response, err := s.GenerateAnkyReflectionFromRawString(writing, writingSessionIPFSHash)
	if err != nil {
		return err
	}
	fmt.Printf("Reflection: %s\n", anky_processing_response)
	fmt.Printf("Reflection: %s\n", anky_processing_response)
	fmt.Printf("Reflection: %s\n", anky_processing_response)
	fmt.Printf("Reflection: %s\n", anky_processing_response)

	anky.Status = "reflection_completed"
	s.store.UpdateAnky(ctx, anky)
	anky.AnkyReflection = anky_processing_response.reflection_to_user
	anky.ImagePrompt = anky_processing_response.image_prompt
	anky.Ticker = anky_processing_response.ticker
	anky.TokenName = anky_processing_response.token_name
	fmt.Printf("Anky++++++++++++++++++++++++++++++++++++++++: %+v\n", anky)

	anky.Status = "going_to_generate_image"
	s.store.UpdateAnky(ctx, anky)

	imageID, err := generateImageWithMidjourney("https://s.mj.run/YLJMlMJbo70 " + anky.ImagePrompt)

	if err != nil {
		log.Printf("Error generating image: %v", err)
		return err
	}
	log.Printf("Image generation response: %s", imageID)

	anky.Status = "generating_image"
	s.store.UpdateAnky(ctx, anky)

	status, err := pollImageStatus(imageID)
	if err != nil {
		log.Printf("Error polling image status: %v", err)
		return err
	}
	log.Printf("Image generation status: %s", status)

	anky.Status = "image_generated"
	s.store.UpdateAnky(ctx, anky)

	// Fetch the image details from the API
	imageDetails, err := fetchImageDetails(imageID)
	if err != nil {
		log.Printf("Error fetching image details: %v", err)
		return err
	}

	// TODO :::: choose the image with a better strategy
	if len(imageDetails.UpscaledURLs) == 0 {
		log.Printf("No upscaled images available")
		return fmt.Errorf("no upscaled images available")
	}

	randomIndex := rand.Intn(len(imageDetails.UpscaledURLs))
	chosenImageURL := imageDetails.UpscaledURLs[randomIndex]

	anky.Status = "uploading_image"
	s.store.UpdateAnky(ctx, anky)

	// Upload the generated image to Cloudinary
	imageHandler, err := NewImageService()
	if err != nil {
		log.Printf("Error creating ImageHandler: %v", err)
		return err
	}

	uploadResult, err := uploadImageToCloudinary(imageHandler, chosenImageURL, sessionID)
	if err != nil {
		log.Printf("Error uploading image to Cloudinary: %v", err)
		return err
	}

	imageIPFSHash, err := pinataService.UploadImageFromURL(uploadResult.SecureURL)
	if err != nil {
		return err
	}

	anky.ImageURL = uploadResult.SecureURL
	anky.ImageIPFSHash = imageIPFSHash

	log.Printf("Image uploaded to Cloudinary successfully. Public ID: %s, URL: %s", uploadResult.PublicID, uploadResult.SecureURL)
	log.Printf("Image uploaded to Pinata successfully. IPFS Hash: %s", imageIPFSHash)

	anky.Status = "image_uploaded"
	s.store.UpdateAnky(ctx, anky)

	// 5. Mark as complete
	anky.Status = "casting_to_farcaster"
	s.store.UpdateAnky(ctx, anky)
	// Get user to check for Farcaster signer UUID
	user, err := s.store.GetUserByID(ctx, uuid.MustParse(userID))
	if err != nil {
		log.Printf("Error getting user: %v", err)
		return err
	}

	if user.FarcasterUser != nil && user.FarcasterUser.SignerUUID != "" {
		castResponse, err := publishAnkyToFarcaster(writing, sessionID, userID, anky.Ticker, anky.TokenName, user.FarcasterUser.SignerUUID, anky.ImageIPFSHash)
		if err != nil {
			log.Printf("Error publishing to Farcaster: %v", err)
			return err
		}

		anky.CastHash = castResponse.Hash
		anky.Status = "completed"
	} else {
		anky.Status = "pending_to_cast"
	}

	s.store.UpdateAnky(ctx, anky)

	return nil
}

// CreateUserProfile creates a new Farcaster profile for a user by:
// 1. Creating a new FID (Farcaster ID) through Neynar's API
// 2. Linking that FID with the user's most recent Anky writing
// 3. Returning the approval URL that the user needs to visit to complete setup
func (s *AnkyService) CreateUserProfile(ctx context.Context, userID uuid.UUID) (string, error) {
	log.Printf("Starting CreateUserProfile service for user ID: %s", userID)

	// First we need to create a new FID (Farcaster ID) for this user
	// This is done by calling Neynar's API which will:
	// 1. Generate a new signer
	// 2. Create a new FID
	// 3. Return the FID number
	neynarService := NewNeynarService()

	newFid, err := neynarService.CreateNewFid(ctx)
	if err != nil {
		log.Printf("Error creating new FID through Neynar: %v", err)
		return "", fmt.Errorf("failed to create new FID: %v", err)
	}

	// We need to get the user's most recent Anky writing
	// This will be linked with their new Farcaster profile
	lastAnky, err := s.store.GetLastAnkyByUserID(ctx, userID)
	if err != nil {
		log.Printf("Error retrieving user's last Anky: %v", err)
		return "", fmt.Errorf("failed to get last Anky: %v", err)
	}

	// Make sure the user has at least one Anky writing
	if lastAnky == nil {
		return "", fmt.Errorf("user must create at least one Anky writing before creating a profile")
	}

	// Update the Anky in our database to store the FID
	// This creates the link between the user's writing and their Farcaster identity
	err = s.store.UpdateAnky(ctx, &types.Anky{
		ID:     lastAnky.ID,
		FID:    newFid,
		Status: "fid_linked",
	})
	if err != nil {
		log.Printf("Error updating Anky with new FID: %v", err)
		return "", fmt.Errorf("failed to link FID to Anky: %v", err)
	}

	// Now we need to tell Neynar to link this Anky with the FID
	// This creates the connection in Neynar's system
	err = s.LinkAnkyWithFid(ctx, lastAnky.ID, newFid)
	if err != nil {
		log.Printf("Error linking Anky with FID in Neynar: %v", err)
		return "", fmt.Errorf("failed to link Anky with FID in Neynar: %v", err)
	}

	// For now return a placeholder URL since the approval URL isn't returned by CreateNewFid
	return "https://farcaster.anky.bot/approve", nil
}

func (s *AnkyService) LinkAnkyWithFid(ctx context.Context, ankyID uuid.UUID, fid int) error {
	// TODO: LINK ANKY WITH NEWLY CREATED FID
	return nil
}

func (s *AnkyService) GenerateFramesgivingNextWritingPrompt(session *utils.WritingSession) (string, error) {
	log.Println("🚀 Starting to generate next writing prompt")

	// Create LLM service to analyze writing and generate prompt
	log.Println("🤖 Creating new LLM service")
	llmService := NewLLMService()

	// Build system prompt focused on gratitude exploration
	log.Println("📝 Building system prompt for gratitude exploration")
	systemPrompt := `You are an AI guide helping users explore gratitude through reflective writing.
Your task is to:
1. Analyze the user's stream of consciousness writing
2. Identify elements, experiences, relationships or feelings that could connect to gratitude
3. Generate a single clear question (inquiry - prompt) that:
   - Links themes from their writing to gratitude
   - Encourages personal reflection
   - Helps them recognize blessings or appreciation in their current circumstances and life context. Regardless of what it is. There is always something to be grateful for.
4. Keep the question concise and heartfelt (one sentence only). 

Important: Do not make any explanations to your reply. Just reply with the inquiry. Nothing else. No context. No explanation. Just the question.`

	// Create chat request with system instructions and user's writing
	log.Println("🔧 Creating chat request with system instructions and user content")
	chatRequest := types.ChatRequest{
		Messages: []types.Message{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: session.RawContent,
			},
		},
	}

	// Get response from LLM using SendChatRequest
	log.Println("📨 Sending chat request to LLM")
	responseChan, err := llmService.SendChatRequest(chatRequest, false)
	if err != nil {
		log.Printf("❌ Error generating gratitude prompt: %v", err)
		return "", fmt.Errorf("failed to generate gratitude prompt: %v", err)
	}

	// Collect full response from channel
	log.Println("📥 Collecting response from LLM")
	var fullResponse string
	for partialResponse := range responseChan {
		fullResponse += partialResponse
	}

	log.Println("✅ Successfully generated next writing prompt")
	return strings.TrimSpace(fullResponse), nil
}

func (s *AnkyService) ReflectBackFromWritingSessionConversation(pastSessions []string, sessionLongString string) (string, error) {

	// Split the session string into lines
	fmt.Printf("sessionLongString is: %v\n", sessionLongString)
	lines := strings.Split(sessionLongString, "\n")
	fmt.Printf("lines are: %v\n", lines)

	// Initialize session struct to store parsed data
	session := []string{}

	if len(lines) < 4 {
		return "", fmt.Errorf("invalid session data: insufficient lines")
	}

	// Extract metadata from first 4 lines
	writingSessionID := lines[0]
	ankyUserID := lines[1]
	writingPrompt := lines[2]
	sessionTimestamp, err := strconv.ParseInt(lines[3], 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid timestamp format: %v", err)
	}

	// Process keystrokes starting from line 4
	for i := 4; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			session = append(session, lines[i])
		}
	}

	fmt.Printf("📝 Parsed writing session metadata:\n")
	fmt.Printf("Session ID: %s\n", writingSessionID)
	fmt.Printf("User ID: %s\n", ankyUserID)
	fmt.Printf("Prompt: %s\n", writingPrompt)
	fmt.Printf("Timestamp: %d\n", sessionTimestamp)
	fmt.Printf("Keystrokes: %d\n", len(session))

	fmt.Println("🤖 Creating new LLM service to process the writing...")
	llmService := NewLLMService()
	fmt.Println("✅ LLM service created successfully")

	fmt.Println("📋 Setting up the initial chat request with system instructions")
	chatRequest := types.ChatRequest{
		Messages: []types.Message{
			{
				Role: "system",
				Content: `You are an AI guide for deep self-exploration. Your role is to analyze the user's stream of consciousness writing and provide short, focused prompts to help them go deeper.

				For each writing session, you'll receive:
				1. The initial prompt
				2. The user's writing session data with timing information
				3. Previous exchanges in the conversation

				Your responses should:
				- Be less than 20 words
				- Ask a specific, probing question based on their writing
				- Help them explore their thoughts more deeply
				- Understand which is the language of the user's writing and reply back in that same language.
				
				Do not make any refences to the process that you are following. Just reply with the inquiry. One line. As if you were ramana maharshi, piercing through the layers of the mind of the user.`,
			},
		},
	}

	fmt.Println("🔄 Starting to process each message in the conversation...")
	for i, content := range pastSessions {
		fmt.Printf("📌 Processing message #%d\n", i+1)
		if i%2 == 0 {
			fmt.Println("👤 Adding assistant message to chat")
			chatRequest.Messages = append(chatRequest.Messages, types.Message{
				Role:    "assistant",
				Content: content,
			})
		} else {
			fmt.Println("✍️ Processing user's writing session...")
			writingSession, err := utils.ParseWritingSession(content)
			if err != nil {
				fmt.Printf("❌ Error parsing writing session: %v\n", err)
				return "", err
			}

			minutes := len(writingSession.KeyStrokes) / 60
			fmt.Printf("⏱️ User wrote for %d minutes\n", minutes)

			contextMsg := fmt.Sprintf("The user wrote for %d minutes. Here is their writing: %s",
				minutes,
				writingSession.RawContent)

			fmt.Println("📤 Adding user's writing to chat context")
			chatRequest.Messages = append(chatRequest.Messages, types.Message{
				Role:    "user",
				Content: contextMsg,
			})
		}
		fmt.Printf("✅ Successfully processed message #%d\n", i+1)
	}

	fmt.Println("🚀 Sending chat request to LLM service...")
	responseChan, err := llmService.SendChatRequest(chatRequest, false)
	if err != nil {
		fmt.Printf("❌ Error sending chat request: %v\n", err)
		return "", err
	}

	fmt.Println("📥 Collecting response from LLM...")
	var fullResponse string
	for response := range responseChan {
		fullResponse += response
		fmt.Println("💬 Received response chunk:", response)
	}

	fmt.Printf("🎉 Completed reflection! Response length: %d characters\n", len(fullResponse))
	return fullResponse, nil
}

// AnkyProcessingResponse holds the structured response from the LLM chain
type AnkyProcessingResponse struct {
	reflection_to_user string
	image_prompt       string
	token_name         string
	ticker             string
}

func (s *AnkyService) TriggerAnkyMintingProcess(parsedSession *utils.WritingSession, fid string, writingSessionIpfsHash string) error {
	log.Println("🚀 Starting Anky minting process...")
	log.Printf("📝 Processing writing session for FID: %s", fid)

	// Generate reflection and metadata using LLM
	log.Println("🤖 Generating reflection from writing content...")
	response, err := s.GenerateAnkyReflectionFromRawString(parsedSession.RawContent, writingSessionIpfsHash)
	if err != nil {
		log.Printf("❌ Error generating reflection: %v", err)
		return fmt.Errorf("error generating reflection: %v", err)
	}

	// Log the response fields
	log.Println("✨ Generated Anky processing response:")
	log.Printf("📖 Reflection to user: %s", response.reflection_to_user)
	log.Printf("🎨 Image prompt: %s", response.image_prompt)
	log.Printf("🏷️ Token name: %s", response.token_name)
	log.Printf("💫 Ticker: %s", response.ticker)

	log.Println("✅ Anky minting process completed successfully")
	return nil
}

func (s *AnkyService) GenerateAnkyReflectionFromRawString(writing string, writingSessionIPFSHash string) (*AnkyProcessingResponse, error) {
	log.Println("🚀 Starting integrated LLM processing chain for writing")

	llmService := NewLLMService()

	// Step 1: Generate reflection story
	log.Println("📖 Step 1: Generating reflection story...")
	storyRequest := types.ChatRequest{
		Messages: []types.Message{
			{
				Role: "system",
				Content: `You are a master storyteller who transforms personal writing into powerful, meaningful narratives.

Your task is to generate a short story (max one page) that:
- Finds the constructive core message in ANY input, no matter how challenging
- Reframes difficult experiences into opportunities for growth and healing
- Creates vivid scenes that inspire hope while acknowledging reality
- Builds tension and momentum through carefully crafted pacing
- Weaves in moments of levity and wit to balance deeper themes
- Makes complex feelings accessible through relatable experiences
- Avoids flowery language while maintaining emotional depth

If the input contains concerning content:
- Focus on the underlying emotions and universal human experiences
- Transform pain points into moments of potential transformation
- Emphasize connection, support, and paths forward
- Maintain appropriate boundaries while showing deep empathy
- Guide the narrative toward hope without dismissing difficulties

The story's protagonist is Anky, a curious little girl who explores the world with wonder and helps others find light in darkness. Use her character to create emotional distance and safety when needed.

Format: Deliver only the story - make every word count and keep the energy focused on growth and possibility.`,
			},
			{
				Role:    "user",
				Content: writing,
			},
		},
	}

	story, err := s.processChatRequest(llmService, storyRequest)
	if err != nil {
		log.Printf("❌ Error generating story: %v", err)
		return nil, fmt.Errorf("error generating story: %v", err)
	}
	log.Printf("✨ Generated reflection story: %s", story)

	// Step 2: Generate image description
	log.Println("🎨 Step 2: Generating image description...")
	imageRequest := types.ChatRequest{
		Messages: []types.Message{
			{
				Role: "system",
				Content: `You are a visual interpretation expert who transforms narratives into detailed image descriptions.

For ANY input, create an uplifting visual scene that:
- Captures the underlying emotional truth in a constructive way
- Uses metaphor and symbolism to maintain appropriate boundaries
- Focuses on growth, healing, and possibility
- Features a blue cartoon character as a gentle guide
- Creates emotional safety through artistic distance

If challenging content is present:
- Transform difficult elements into abstract symbols of hope
- Use color, light and composition to suggest paths forward
- Create scenes that inspire while respecting serious themes
- Maintain appropriateness while honoring emotional depth

Format: Provide only the image prompt focused on positive transformation.`,
			},
			{
				Role:    "user",
				Content: "Here is the story to visualize:\n\n" + story,
			},
			{
				Role: "user",
				Content: `Create a detailed image generation prompt that:
- Captures the emotional essence of the story
- Maintains consistent metaphors and symbols
- Provides specific details about composition, lighting, and mood
- Creates a scene that resonates with the narrative journey

Format: Provide only the image prompt, no additional context or explanation.`,
			},
		},
	}

	imagePrompt, err := s.processChatRequest(llmService, imageRequest)
	if err != nil {
		log.Printf("❌ Error generating image prompt: %v", err)
		return nil, fmt.Errorf("error generating image prompt: %v", err)
	}
	log.Printf("🖼️ Generated image prompt: %s", imagePrompt)
	// Step 3: Generate token name
	log.Println("🏷️ Step 3: Generating token name...")
	tokenRequest := types.ChatRequest{
		Messages: []types.Message{
			{
				Role: "system",
				Content: `You are a token naming specialist who creates meaningful three-word identifiers.
You will receive a story and image description to inform your creation. Your goal is to create an uplifting, 
positive token name that celebrates growth, wonder, and the human experience.

Important guidelines:
- Avoid any references to harm, negativity, or darkness
- Focus on themes of discovery, joy, connection, and transformation
- Use language that is appropriate for all audiences
- Draw inspiration from nature, art, science, and universal human experiences`,
			},
			{
				Role:    "user",
				Content: "Story:\n\n" + story + "\n\nImage Description:\n\n" + imagePrompt,
			},
			{
				Role: "user",
				Content: `Generate a three-word token name that:
- Distills the core essence of this narrative and imagery into something uplifting and meaningful
- Maintains thematic consistency while focusing on positive aspects
- Creates a poetic and memorable identifier that inspires hope and connection
- Uses clear, universally appropriate language
- Avoids any potentially concerning or negative connotations

Format: Return exactly three words, separated by spaces. No additional context or explanation.

Example good token names:
"Wisdom Light Dancing"
"Nature Spirit Rising"
"Ocean Dream Awakening"`,
			},
		},
	}

	tokenName, err := s.processChatRequest(llmService, tokenRequest)
	if err != nil {
		log.Printf("❌ Error generating token name: %v", err)
		return nil, fmt.Errorf("error generating token name: %v", err)
	}
	log.Printf("💫 Generated token name: %s", tokenName)

	// Step 4: Generate ticker symbol
	log.Println("💱 Step 4: Generating ticker symbol...")
	tickerRequest := types.ChatRequest{
		Messages: []types.Message{
			{
				Role: "system",
				Content: `You are a financial symbol specialist who creates meaningful ticker symbols.
You will receive a story, image description, and token name to inform your creation.

Important guidelines:
- Create symbols that are positive and appropriate for all audiences
- Focus on growth, innovation, and universal human experiences
- Avoid any references to concerning topics or negative themes
- Draw inspiration from technology, nature, art, and positive human qualities
- Ensure the symbol would be appropriate in professional financial contexts`,
			},
			{
				Role:    "user",
				Content: "Story:\n\n" + story + "\n\nImage Description:\n\n" + imagePrompt + "\n\nToken Name:\n\n" + tokenName,
			},
			{
				Role: "user",
				Content: `Create a unique ticker symbol that:
- Maximum 24 characters
- Reflects the positive essence of the narrative, image, and token name
- Creates an intriguing and memorable identifier
- Must be in uppercase letters
- Should be appropriate for public markets and professional settings
- Avoids any potentially concerning connotations

Format: Return only the uppercase ticker symbol. No additional context or explanation.

Example good tickers:
"DREAM" "HOPE" "INNOVATE" "GROW" "CREATE", "adjkahsdy7ias6dgajkhsc", or anything that carries memetic energy. go wild. make something fun.`,
			},
		},
	}

	ticker, err := s.processChatRequest(llmService, tickerRequest)
	if err != nil {
		log.Printf("❌ Error generating ticker: %v", err)
		return nil, fmt.Errorf("error generating ticker: %v", err)
	}
	log.Printf("🎯 Generated ticker symbol: %s", ticker)

	// Validate outputs
	log.Println("✅ Validating outputs...")
	if err := validateOutputs(story, imagePrompt, tokenName, ticker); err != nil {
		log.Printf("❌ Validation error: %v", err)
		return nil, fmt.Errorf("validation error: %v", err)
	}

	log.Println("🎉 Successfully generated all components!")

	pinataService, err := NewPinataService()
	if err != nil {
		log.Printf("❌ Error creating Pinata service: %v", err)
		return nil, fmt.Errorf("error creating Pinata service: %v", err)
	}

	ankyImageIpfsHash, err := s.GenerateAnkyFromPrompt(imagePrompt)
	if err != nil {
		log.Printf("❌ Error generating Anky image: %v", err)
		return nil, fmt.Errorf("error generating Anky image: %v", err)
	}
	log.Printf("🖼️ Generated Anky image hash: %s", ankyImageIpfsHash)

	// Update NFT metadata
	metadata := map[string]interface{}{
		"name":        tokenName,
		"description": story,
		"image":       fmt.Sprintf("ipfs://%s", ankyImageIpfsHash),
		"attributes": []map[string]string{
			{"trait_type": "Image Prompt", "value": imagePrompt},
			{"trait_type": "Ticker", "value": ticker},
			{"trait_type": "Token Name", "value": tokenName},
			{"trait_type": "Story", "value": story},
			{"trait_type": "Writing Session", "value": writingSessionIPFSHash},
		},
	}

	metadataHash, err := pinataService.UploadJSONMetadata(metadata)
	if err != nil {
		log.Printf("❌ Error uploading metadata: %v", err)
		return nil, fmt.Errorf("error uploading metadata: %v", err)
	}
	log.Printf("📄 Metadata pinned to IPFS: %s", metadataHash)

	// Reveal the Anky NFT on-chain
	// err = blockchainService.RevealAnky(context.Background(), metadataHash, ipfsHash)
	// if err != nil {
	// 	log.Printf("❌ Error revealing Anky on blockchain: %v", err)
	// 	return nil, fmt.Errorf("error revealing Anky: %v", err)
	// }
	// log.Printf("⛓️ Successfully revealed Anky on blockchain")

	// HERE WE NEED TO CALL THE SMART CONTRACT TO REVEAL THE ANKY AND DEPLOY THE NFT

	return &AnkyProcessingResponse{
		reflection_to_user: story,
		image_prompt:       imagePrompt,
		token_name:         tokenName,
		ticker:             ticker,
	}, nil
}

// Helper function to validate outputs
func validateOutputs(story, imagePrompt, tokenName, ticker string) error {
	// Validate story length (approximate one page ~ 3000 characters)
	if len(story) > 3000 {
		return fmt.Errorf("story exceeds maximum length")
	}

	// Validate token name has exactly three words
	words := strings.Fields(tokenName)
	if len(words) != 3 {
		return fmt.Errorf("token name must contain exactly three words")
	}

	// Validate ticker length and format
	if len(ticker) > 24 {
		return fmt.Errorf("ticker exceeds 24 characters")
	}
	if ticker != strings.ToUpper(ticker) {
		return fmt.Errorf("ticker must be uppercase")
	}

	return nil
}

// Helper function to process chat requests and extract response
func (s *AnkyService) processChatRequest(llmService *LLMService, request types.ChatRequest) (string, error) {
	responseChan, err := llmService.SendChatRequest(request, false)
	if err != nil {
		return "", err
	}

	var fullResponse string
	for partialResponse := range responseChan {
		fullResponse += partialResponse
	}

	return strings.TrimSpace(fullResponse), nil
}

func (s *AnkyService) SimplePrompt(ctx context.Context, prompt string) (string, error) {
	llmService := NewLLMService()
	responseChan, err := llmService.SendSimpleRequest(prompt)
	if err != nil {
		return "", fmt.Errorf("error sending simple request: %v", err)
	}

	var fullResponse string
	for partialResponse := range responseChan {
		fullResponse += partialResponse
	}

	return fullResponse, nil
}

func (s *AnkyService) MessagesPromptRequest(messages []string) (string, error) {
	llmService := NewLLMService()

	// Convert string messages to Message structs
	chatMessages := make([]types.Message, len(messages))
	for i, msg := range messages {
		chatMessages[i] = types.Message{
			Role:    "user",
			Content: msg,
		}
	}

	chatRequest := types.ChatRequest{
		Messages: chatMessages,
	}

	responseChan, err := llmService.SendChatRequest(chatRequest, false)
	if err != nil {
		return "", fmt.Errorf("error sending chat request: %v", err)
	}

	var fullResponse string
	for partialResponse := range responseChan {
		fullResponse += partialResponse
	}

	return fullResponse, nil
}

func generateImageWithMidjourney(prompt string) (string, error) {
	data := map[string]interface{}{
		"prompt": prompt,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("error marshaling data: %v", err)
	}

	req, err := http.NewRequest("POST", "http://localhost:8055/items/images/", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+os.Getenv("IMAGINE_API_TOKEN"))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	var responseData struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	err = json.NewDecoder(resp.Body).Decode(&responseData)
	if err != nil {
		return "", fmt.Errorf("error decoding response: %v", err)
	}

	return responseData.Data.ID, nil
}

func pollImageStatus(id string) (string, error) {
	fmt.Println("Starting pollImageStatus for id:", id)
	for {
		fmt.Println("Checking image status for id:", id)
		status, err := checkImageStatus(id)
		if err != nil {
			fmt.Println("Error checking image status:", err)
			return "", err
		}

		fmt.Println("Current status for id", id, ":", status)

		if status == "completed" {
			fmt.Println("Image generation completed for id:", id)
			return status, nil
		}

		if status == "failed" {
			fmt.Println("Image generation failed for id:", id)
			return status, fmt.Errorf("image generation failed")
		}

		fmt.Println("Waiting 5 seconds before next status check for id:", id)
		time.Sleep(5 * time.Second)
	}
}

func checkImageStatus(id string) (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:8055/items/images/%s", id), nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+os.Getenv("IMAGINE_API_TOKEN"))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	var responseData struct {
		Data struct {
			Status string `json:"status"`
			URL    string `json:"url"`
		} `json:"data"`
	}

	err = json.NewDecoder(resp.Body).Decode(&responseData)
	if err != nil {
		return "", fmt.Errorf("error decoding response: %v", err)
	}

	return responseData.Data.Status, nil
}

func fetchImageDetails(id string) (*ImageDetails, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:8055/items/images/%s", id), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+os.Getenv("IMAGINE_API_TOKEN"))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	var responseData struct {
		Data ImageDetails `json:"data"`
	}

	err = json.NewDecoder(resp.Body).Decode(&responseData)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	return &responseData.Data, nil
}

type ImageDetails struct {
	Status       string   `json:"status"`
	URL          string   `json:"url"`
	UpscaledURLs []string `json:"upscaled_urls"`
}

func (s *AnkyService) GenerateAnkyFromPrompt(prompt string) (string, error) {
	log.Println("Starting GenerateAnkyFromPrompt service")

	// Generate image using Midjourney
	log.Println("Generating image with Midjourney")
	imageID, err := generateImageWithMidjourney("https://s.mj.run/YLJMlMJbo70 " + prompt)
	if err != nil {
		log.Printf("Failed to generate image: %v", err)
		return "", fmt.Errorf("failed to generate image: %v", err)
	}
	log.Printf("Generated image ID: %s", imageID)

	// Poll for image completion
	log.Println("Polling for image completion")
	status, err := pollImageStatus(imageID)
	if err != nil {
		log.Printf("Error polling image status: %v", err)
		return "", fmt.Errorf("error polling image status: %v", err)
	}
	log.Printf("Image status: %s", status)

	if status != "completed" {
		log.Println("Image generation failed")
		return "", fmt.Errorf("image generation failed")
	}

	// Fetch final image details
	log.Println("Fetching image details")
	imageDetails, err := fetchImageDetails(imageID)
	if err != nil {
		log.Printf("Error fetching image details: %v", err)
		return "", fmt.Errorf("error fetching image details: %v", err)
	}
	log.Printf("Retrieved image URL: %s", imageDetails.URL)

	// Upload to Cloudinary
	log.Println("Uploading to Cloudinary")
	imageHandler, err := NewImageService()
	if err != nil {
		log.Printf("Error creating ImageHandler: %v", err)
		return "", fmt.Errorf("error creating ImageHandler: %v", err)
	}
	uploadResult, err := uploadImageToCloudinary(imageHandler, imageDetails.URL, uuid.New().String())
	if err != nil {
		log.Printf("Error uploading to Cloudinary: %v", err)
		return "", fmt.Errorf("error uploading to Cloudinary: %v", err)
	}
	log.Printf("Successfully uploaded to Cloudinary. URL: %s", uploadResult.SecureURL)

	pinataService, err := NewPinataService()
	if err != nil {
		log.Printf("❌ Error creating Pinata service: %v", err)
		return "", fmt.Errorf("error creating Pinata service: %v", err)
	}
	ipfsHash, err := pinataService.UploadImageFromURL(uploadResult.SecureURL)
	if err != nil {
		log.Printf("❌ Error uploading image to Pinata: %v", err)
		return "", fmt.Errorf("error uploading image to Pinata: %v", err)
	}

	return ipfsHash, nil
}

func (s *AnkyService) EditCast(ctx context.Context, text string, userFid int) (string, error) {
	log.Printf("Starting edit cast service with text: %s and userFid: %d", text, userFid)
	return "", nil
}

func (s *AnkyService) OnboardingConversation(ctx context.Context, userId uuid.UUID, sessions []*types.WritingSession, ankyReflections []string) (string, error) {
	log.Printf("Starting onboarding conversation for attempt #%d", len(sessions))

	llmService := NewLLMService()

	systemPrompt := `You are Anky, a wise guide inspired by Ramana Maharshi's practice of self-inquiry. Your role is to help users with their journey of daily stream-of-consciousness writing.

Context:
- Users are asked to write continuously for 8 minutes
- The interface shows only a prompt and text area
- The session ends if they pause for more than 8 seconds
- This user has made ${sessions.length} previous attempts

Your Task:
Provide a single-sentence response that:
1. References specific words, themes or ideas from their writing to show deep understanding
2. Acknowledges their progress based on writing duration:
   - Under 1 minute: Validate their first steps
   - 1-4 minutes: Recognize their growing momentum  
   - 4-7 minutes: Celebrate their deeper exploration
   - 7+ minutes: Honor their full expression
3. Offers encouragement that builds naturally from their own words and themes

Key Guidelines:
- Make them feel truly seen and understood
- Inspire them to continue their writing practice
- Keep focus on their unique perspective and voice
- Maintain a warm, supportive tone
- Craft a response that resonates with their specific experience

Remember: Your response will be the only feedback they see after their writing session. Make it meaningful and motivating. Make it short and concise, less than 88 characters.`

	// Build conversation history with progression context
	messages := []types.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	// Add session context with progression markers
	for i := 0; i < len(sessions); i++ {
		timeSpent := sessions[i].TimeSpent
		wordsWritten := sessions[i].WordsWritten

		attemptContext := fmt.Sprintf(`Writing Duration: %d seconds
Words Written: %d

Their words:
%s`,
			timeSpent,
			wordsWritten,
			sessions[i].Writing,
		)

		messages = append(messages, types.Message{
			Role:    "user",
			Content: attemptContext,
		})

		if i < len(ankyReflections) {
			messages = append(messages, types.Message{
				Role:    "assistant",
				Content: ankyReflections[i],
			})
		}
	}

	chatRequest := types.ChatRequest{
		Messages: messages,
	}

	log.Printf("Sending reflective conversation request %v", chatRequest)
	responseChan, err := llmService.SendChatRequest(chatRequest, false)
	if err != nil {
		log.Printf("Error sending chat request: %v", err)
		return "", err
	}

	var fullResponse string
	for partialResponse := range responseChan {
		fullResponse += partialResponse
	}

	return fullResponse, nil
}

func getOnboardingStage(duration int) string {
	switch {
	case duration < 60:
		return "Initial_Exploration"
	case duration < 240:
		return "Building_Momentum"
	case duration < 420:
		return "Approaching_Goal"
	case duration >= 420:
		return "Goal_Achieved"
	default:
		return "Unknown"
	}
}
