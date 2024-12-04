package utils

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type WritingSession struct {
	UserID     string
	SessionID  string
	Prompt     string
	Timestamp  string
	KeyStrokes []KeyStroke
	RawContent string
	TimeSpent  int
}

type KeyStroke struct {
	Key   string
	Delay int
}

func ParseWritingSession(content string) (*WritingSession, error) {
	fmt.Println("🔍 Starting to parse writing session...")
	fmt.Printf("📄 Raw content:\n%s\n", content)
	lines := strings.Split(content, "\n")
	fmt.Printf("📝 Found %d lines in content\n", len(lines))

	if len(lines) < 4 {
		fmt.Println("❌ Invalid format: Not enough lines")
		return nil, fmt.Errorf("invalid writing session format")
	}

	session := &WritingSession{
		UserID:    strings.TrimSpace(lines[0]),
		SessionID: strings.TrimSpace(lines[1]),
		Prompt:    strings.TrimSpace(lines[2]),
		Timestamp: strings.TrimSpace(lines[3]),
		TimeSpent: 0,
	}

	fmt.Printf("📋 Session metadata:\n"+
		"UserID: %s\n"+
		"SessionID: %s\n"+
		"Prompt: %s\n"+
		"Timestamp: %s\n",
		session.UserID, session.SessionID, session.Prompt, session.Timestamp)

	var keyStrokes []KeyStroke
	var constructedText strings.Builder
	totalMilliseconds := 0 // Track total time in milliseconds
	fmt.Println("⏱️ Starting to track session duration")

	for i := 4; i < len(lines); i++ {
		line := lines[i] // Don't trim the space here
		if line == "" {
			continue
		}

		// Handle the case where the line starts with a space (meaning it's a space keystroke)
		var key string
		var delayStr string

		if strings.HasPrefix(line, " ") && strings.Count(line, " ") == 2 {
			// This is a space keystroke
			key = " "
			delayStr = strings.TrimSpace(line)
			fmt.Println("🔤 Found space keystroke")
		} else {
			lastSpaceIndex := strings.LastIndex(line, " ")
			if lastSpaceIndex == -1 {
				fmt.Printf("⚠️ Skipping invalid line: %s\n", line)
				continue
			}
			key = strings.TrimSpace(line[:lastSpaceIndex])
			delayStr = strings.TrimSpace(line[lastSpaceIndex+1:])
		}

		// Try to parse delay as float first
		delayFloat, err := strconv.ParseFloat(delayStr, 64)
		if err != nil {
			fmt.Printf("⚠️ Invalid delay value: %s\n", delayStr)
			continue
		}

		// Convert to milliseconds and add to total
		delay := int(delayFloat * 1000)
		totalMilliseconds += delay
		fmt.Printf("⏱️ Added delay of %d milliseconds\n", delay)

		keyStroke := KeyStroke{
			Key:   key,
			Delay: delay,
		}
		keyStrokes = append(keyStrokes, keyStroke)

		switch key {
		case "Backspace":
			if constructedText.Len() > 0 {
				str := constructedText.String()
				constructedText.Reset()
				constructedText.WriteString(str[:len(str)-1])
				fmt.Println("⌫ Processed backspace")
			}
		case "Enter":
			constructedText.WriteString("\n")
			fmt.Println("↵ Processed enter key")
		case " ":
			constructedText.WriteRune(' ')
			fmt.Println("␣ Processed space")
		default:
			constructedText.WriteString(key)
			fmt.Printf("⌨️ Added key: %s\n", key)
		}
	}

	session.KeyStrokes = keyStrokes
	session.RawContent = constructedText.String()
	session.TimeSpent = (totalMilliseconds / 1000) + 8 // Convert to seconds and add base duration
	session.TimeSpent = 490

	fmt.Printf("✅ Finished parsing session:\n"+
		"Total keystrokes: %d\n"+
		"Content length: %d characters\n"+
		"Total time: %d seconds\n",
		len(keyStrokes), len(session.RawContent), session.TimeSpent)

	return session, nil
}
func SaveWritingSessionLocally(content string) (*WritingSession, error) {
	fmt.Println("🔍 Starting to parse writing session...")
	fmt.Printf("📄 Raw content:\n%s\n", content)
	lines := strings.Split(content, "\n")
	fmt.Printf("📝 Found %d lines in content\n", len(lines))

	if len(lines) < 4 {
		fmt.Println("❌ Invalid format: Not enough lines")
		return nil, fmt.Errorf("invalid writing session format")
	}

	session := &WritingSession{
		UserID:    strings.TrimSpace(lines[0]),
		SessionID: strings.TrimSpace(lines[1]),
		Prompt:    strings.TrimSpace(lines[2]),
		Timestamp: strings.TrimSpace(lines[3]),
		TimeSpent: 0,
	}

	// Create user directory if it doesn't exist
	userDir := fmt.Sprintf("data/framesgiving/%s", session.UserID)
	if err := os.MkdirAll(userDir, 0755); err != nil {
		fmt.Printf("❌ Error creating directory: %v\n", err)
		return nil, fmt.Errorf("error creating directory: %v", err)
	}

	// Save full session content to individual file
	sessionPath := fmt.Sprintf("%s/%s.txt", userDir, session.SessionID)
	if err := os.WriteFile(sessionPath, []byte(content), 0644); err != nil {
		fmt.Printf("❌ Error saving session file: %v\n", err)
		return nil, fmt.Errorf("error saving session file: %v", err)
	}

	// Append session info to user's writing sessions file
	sessionsPath := fmt.Sprintf("%s/%s_writing_sessions.txt", userDir, session.UserID)
	sessionLine := fmt.Sprintf("%s\n", session.SessionID)

	f, err := os.OpenFile(sessionsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("❌ Error opening sessions file: %v\n", err)
		return nil, fmt.Errorf("error opening sessions file: %v", err)
	}
	defer f.Close()

	if _, err := f.WriteString(sessionLine); err != nil {
		fmt.Printf("❌ Error writing to sessions file: %v\n", err)
		return nil, fmt.Errorf("error writing to sessions file: %v", err)
	}

	fmt.Printf("✅ Successfully saved writing session for user %s\n", session.UserID)
	return session, nil
}

func TranslateToTheAnkyverse(sessionID string) string {
	// Define the Ankyverse characters
	characters := []string{
		"\u0C85", "\u0C86", "\u0C87", "\u0C88", "\u0C89", "\u0C8A", "\u0C8B", "\u0C8C", "\u0C8E", "\u0C8F",
		"\u0C90", "\u0C92", "\u0C93", "\u0C94", "\u0C95", "\u0C96", "\u0C97", "\u0C98", "\u0C99", "\u0C9A",
		"\u0C9B", "\u0C9C", "\u0C9D", "\u0C9E", "\u0C9F", "\u0CA0", "\u0CA1", "\u0CA2", "\u0CA3", "\u0CA4",
		"\u0CA5", "\u0CA6", "\u0CA7", "\u0CA8", "\u0CAA", "\u0CAB", "\u0CAC", "\u0CAD", "\u0CAE", "\u0CAF",
		"\u0CB0", "\u0CB1", "\u0CB2", "\u0CB3", "\u0CB5", "\u0CB6", "\u0CB7", "\u0CB8", "\u0CB9", "\u0CBC",
		"\u0CBD", "\u0CBE", "\u0CBF", "\u0CC0", "\u0CC1", "\u0CC2", "\u0CC3", "\u0CC4", "\u0CC6", "\u0CC7",
		"\u0CC8", "\u0CCA", "\u0CCB", "\u0CCC", "\u0CCD", "\u0CD5", "\u0CD6", "\u0CDE", "\u0CE0", "\u0CE1",
		"\u0CE2", "\u0CE3", "\u0CE6", "\u0CE7", "\u0CE8", "\u0CE9", "\u0CEA", "\u0CEB", "\u0CEC", "\u0CED",
		"\u0CEE", "\u0CEF", "\u0CF1", "\u0CF2", "\u0C05", "\u0C06", "\u0C07", "\u0C08", "\u0C09", "\u0C0A",
		"\u0C0B", "\u0C0C", "\u0C0E", "\u0C0F", "\u0C10", "\u0C12", "\u0C13", "\u0C14",
	}

	// Encode the sessionID to Ankyverse language
	var encoded strings.Builder
	for i := 0; i < len(sessionID); i++ {
		charCode := int(sessionID[i])
		index := (charCode - 32) % len(characters)
		encoded.WriteString(characters[index])
	}

	return encoded.String()
}
