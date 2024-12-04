package utils

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/ankylat/anky/server/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func GetUserID(r *http.Request) (uuid.UUID, error) {
	vars := mux.Vars(r)
	return uuid.Parse(vars["userId"])
}

func GetAnkyID(r *http.Request) (uuid.UUID, error) {
	vars := mux.Vars(r)
	return uuid.Parse(vars["id"])
}

func CreateJWT(user *types.User) (string, error) {
	claims := &jwt.MapClaims{
		"expiresAt": time.Now().Add(400 * 24 * time.Hour).Unix(),
		"userID":    user.ID,
	}

	secretKey := os.Getenv("JWT_SECRET")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(secretKey))
}

func ValidateJWT(token string) (*jwt.MapClaims, error) {
	secretKey := os.Getenv("JWT_SECRET")
	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok && parsedToken.Valid {
		return &claims, nil
	}

	return nil, jwt.ErrSignatureInvalid
}

func PrettyPrintMap(m map[string]interface{}) {
	// Get all keys and sort them
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Print header
	log.Println("----------------------------------------")

	// Print each key-value pair in sorted order
	for _, k := range keys {
		v := m[k]
		switch val := v.(type) {
		case map[string]interface{}:
			log.Printf("%s:", k)
			PrettyPrintMap(val) // Recursively print nested maps
		case []interface{}:
			log.Printf("%s: [", k)
			for i, item := range val {
				log.Printf("  %d: %v", i, item)
			}
			log.Println("]")
		default:
			log.Printf("%s: %v", k, v)
		}
	}

	log.Println("----------------------------------------")
}

// PrintBeautifulLog prints information to the console in a visually appealing way.
// Parameters:
// - emoji: A string containing an emoji to visually categorize the log (e.g. "🚀", "⚠️", "✅")
// - stepNumber: An integer representing the step/sequence number in a process
// - message: The main message to be displayed
// - details: Optional additional details about the operation (can be nil)
// - isError: Boolean flag to indicate if this is an error message
//
// This function was designed to:
// 1. Make logs more visually organized and easier to scan
// 2. Add visual hierarchy with emojis and step numbers
// 3. Provide consistent formatting across the application
// 4. Allow for both simple and detailed logging
func PrintBeautifulLog(emoji string, stepNumber int, message string, details interface{}, isError bool) {
	// Create a colorful separator line
	separator := "=================================================="

	// Print the top separator
	fmt.Println("\n" + separator)

	// Format the step number with leading zeros for better alignment
	stepStr := fmt.Sprintf("%03d", stepNumber)

	// Choose text color based on whether it's an error
	var statusPrefix string
	if isError {
		statusPrefix = "❌ ERROR"
	} else {
		statusPrefix = "ℹ️ INFO"
	}

	// Print the main log line with emoji, step number and message
	fmt.Printf("%s [Step %s] %s | %s\n", statusPrefix, stepStr, emoji, message)

	// If additional details were provided, print them in a structured way
	if details != nil {
		fmt.Println("📋 Details:")
		switch v := details.(type) {
		case string:
			fmt.Printf("   %s\n", v)
		case error:
			fmt.Printf("   Error: %v\n", v)
		case map[string]interface{}:
			for key, value := range v {
				fmt.Printf("   %s: %v\n", key, value)
			}
		default:
			fmt.Printf("   %+v\n", v)
		}
	}

	// Print the bottom separator
	fmt.Println(separator + "\n")
}
