package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
)

type PinataService struct {
	jwt         string
	apiEndpoint string
	
}

func NewPinataService() (*PinataService, error) {
	// Get JWT from environment
	jwt := os.Getenv("PINATA_JWT")
	if jwt == "" {
		return nil, fmt.Errorf("PINATA_JWT not found in environment")
	}

	return &PinataService{
		jwt:         jwt,
		apiEndpoint: "https://anky.pinata.cloud",
	}, nil
}



func (s *PinataService) UploadImageFromURL(imageURL string) (string, error) {
	log.Printf("Starting Pinata upload process for image URL: %s", imageURL)

	// Download image from URL
	resp, err := http.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %v", err)
	}
	defer resp.Body.Close()

	// Read image data
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read image data: %v", err)
	}

	// Create multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "image")
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %v", err)
	}

	// Write image data to form
	if _, err := part.Write(imageData); err != nil {
		return "", fmt.Errorf("failed to write image data: %v", err)
	}
	writer.Close()

	// Create upload request
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/pinning/pinFileToIPFS", s.apiEndpoint), body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.jwt))
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Parse response
	var result struct {
		IpfsHash string `json:"IpfsHash"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	log.Printf("Successfully uploaded to IPFS with hash: %s", result.IpfsHash)
	return result.IpfsHash, nil
}

func (s *PinataService) UploadJSONMetadata(metadata interface{}) (string, error) {
	log.Printf("Starting Pinata upload process for metadata")

	// Convert metadata to JSON
	jsonData, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %v", err)
	}

	// Create request
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/pinning/pinJSONToIPFS", s.apiEndpoint), bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.jwt))
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Parse response
	var result struct {
		IpfsHash string `json:"IpfsHash"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	log.Printf("Successfully uploaded metadata to IPFS with hash: %s", result.IpfsHash)
	return result.IpfsHash, nil
}

func (s *PinataService) UploadTXTFile(file_long_string string) (string, error) {
	log.Printf("Starting Pinata upload process for text file")

	// Create form data
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Create file field
	fw, err := w.CreateFormFile("file", "content.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %v", err)
	}

	// Write string content to file field
	if _, err := fw.Write([]byte(file_long_string)); err != nil {
		return "", fmt.Errorf("failed to write file content: %v", err)
	}

	// Close multipart writer
	w.Close()

	// Create request
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/pinning/pinFileToIPFS", s.apiEndpoint), &b)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.jwt))
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pinata request failed with status: %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		IpfsHash string `json:"IpfsHash"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	log.Printf("Successfully uploaded text file to IPFS with hash: %s", result.IpfsHash)
	return result.IpfsHash, nil
}
