package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/pkg/sftp"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

type AlbumItem struct {
	ID          interface{} `json:"id"`
	URL         string      `json:"url"`
	Description string      `json:"description"`
	Credits     string      `json:"credits"`
}

var publishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish the first image in the album",
	Long:  `Upload the first image URL to SFTP server as "today", invalidate CDN cache, then archive the item.`,
	Run: func(cmd *cobra.Command, args []string) {
		publishProcess()
	},
}

// GetPublishCmd returns the publish command
func GetPublishCmd() *cobra.Command {
	return publishCmd
}

func publishProcess() {
	// 1. Parse album.json
	albumPath := filepath.Join("data", "album.json")
	albumFile, err := os.ReadFile(albumPath)
	if err != nil {
		fmt.Printf("Error reading album.json: %v\n", err)
		os.Exit(1)
	}

	var albumItems []AlbumItem
	if err := json.Unmarshal(albumFile, &albumItems); err != nil {
		fmt.Printf("Error parsing album.json: %v\n", err)
		os.Exit(1)
	}

	if len(albumItems) == 0 {
		fmt.Println("No items in album.json")
		os.Exit(1)
	}

	// 2. Get the first item
	firstItem := albumItems[0]
	fmt.Printf("Publishing item: %v with URL: %s\n", firstItem.ID, firstItem.URL)

	// 3. Upload item to SFTP server
	if err := uploadToSFTP(firstItem); err != nil {
		fmt.Printf("Error uploading to SFTP server: %v\n", err)
		os.Exit(1)
	}

	// 4. Invalidate CDN cache
	if err := invalidateCache(); err != nil {
		fmt.Printf("Warning: Failed to invalidate CDN cache: %v\n", err)
		// Continue execution even if cache invalidation fails
	}

	// 5. Append the item to data/archive.json
	archivePath := filepath.Join("data", "archive.json")
	var archiveItems []AlbumItem

	// Read existing archive.json if it exists
	archiveFile, err := os.ReadFile(archivePath)
	if err == nil {
		// File exists, parse it
		if err := json.Unmarshal(archiveFile, &archiveItems); err != nil {
			fmt.Printf("Error parsing archive.json: %v\n", err)
			os.Exit(1)
		}
	} else if !os.IsNotExist(err) {
		// Error other than file not existing
		fmt.Printf("Error reading archive.json: %v\n", err)
		os.Exit(1)
	}

	// Append the item
	archiveItems = append(archiveItems, firstItem)

	// Write back to archive.json
	archiveData, err := json.Marshal(archiveItems)
	if err != nil {
		fmt.Printf("Error serializing archive data: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		fmt.Printf("Error writing to archive.json: %v\n", err)
		os.Exit(1)
	}

	// 6. Delete the item from data/album.json
	albumItems = albumItems[1:]
	albumData, err := json.Marshal(albumItems)
	if err != nil {
		fmt.Printf("Error serializing album data: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(albumPath, albumData, 0o644); err != nil {
		fmt.Printf("Error writing to album.json: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully published the image, uploaded to SFTP, invalidated cache, archived the item, and updated album.")
}

func uploadToSFTP(item AlbumItem) error {
	godotenv.Load()

	// Get SFTP credentials from environment
	host := os.Getenv("SFTP_HOST")
	portStr := os.Getenv("SFTP_PORT")
	user := os.Getenv("SFTP_USER")
	password := os.Getenv("SFTP_PASSWORD")
	keyPath := os.Getenv("SFTP_PRIVATE_KEY_PATH")

	// Validate required environment variables
	if host == "" || user == "" || (password == "" && keyPath == "") {
		return fmt.Errorf("required SFTP environment variables not set")
	}

	// Default port is 22 if not specified
	port := 22
	if portStr != "" {
		var err error
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("invalid SFTP port: %v", err)
		}
	}

	// Configure SSH client
	var authMethods []ssh.AuthMethod
	if password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	} else if keyPath != "" {
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("unable to read private key: %v", err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("unable to parse private key: %v", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Note: This is not secure for production
		Timeout:         15 * time.Second,
	}

	// Connect to SFTP server
	addr := fmt.Sprintf("%s:%d", host, port)
	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %v", err)
	}
	defer sshClient.Close()

	// Create SFTP client
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %v", err)
	}
	defer sftpClient.Close()

	// Create JSON content to upload
	content, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("error serializing item data: %v", err)
	}

	// Create a file on the SFTP server
	remoteFile, err := sftpClient.Create("today.json")
	if err != nil {
		return fmt.Errorf("failed to create remote file: %v", err)
	}
	defer remoteFile.Close()

	// Write JSON content to the file
	_, err = remoteFile.Write(content)
	if err != nil {
		return fmt.Errorf("failed to write to remote file: %v", err)
	}

	fmt.Println("Successfully uploaded item to SFTP server as 'today.json'")
	return nil
}

func invalidateCache() error {
	godotenv.Load()

	// Get API credentials from environment
	apiKey := os.Getenv("BUNNY_API_KEY")
	cdnURL := os.Getenv("BUNNY_CDN_URL")

	// Validate required environment variables
	if apiKey == "" || cdnURL == "" {
		return fmt.Errorf("required API environment variables not set (BUNNY_API_KEY, BUNNY_CDN_URL)")
	}

	// Construct purge URL for the "today" file
	purgeURL := fmt.Sprintf("https://api.bunny.net/purge?url=%s/today.json", cdnURL)

	// Create HTTP request
	req, err := http.NewRequest("POST", purgeURL, bytes.NewBuffer([]byte{}))
	if err != nil {
		return fmt.Errorf("error creating HTTP request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AccessKey", apiKey)

	// Send request
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	fmt.Println("Successfully invalidated CDN cache for 'today.json' file")
	return nil
}

