package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/pkg/sftp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
)

type AlbumItem struct {
	ID          interface{} `json:"id"`
	URL         string      `json:"url"`
	Description string      `json:"description"`
	Credits     string      `json:"credits"`
}

var port string

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the web server",
	Long:  `Start the web server on the specified port.`,
	Run: func(cmd *cobra.Command, args []string) {
		startServer()
	},
}

// GetServerCmd returns the server command
func GetServerCmd() *cobra.Command {
	return serverCmd
}

func init() {
	serverCmd.Flags().StringVarP(&port, "port", "p", "8080", "Port to run the server on")
	viper.BindPFlag("server.port", serverCmd.Flags().Lookup("port"))
	viper.SetDefault("server.port", "8080")
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)

		duration := time.Since(start)
		log.Printf("%s %s in %v", r.Method, r.URL.Path, duration)
	})
}

func publishProcess() error {
	// 1. Parse album.json
	albumPath := filepath.Join("data", "album.json")
	albumFile, err := os.ReadFile(albumPath)
	if err != nil {
		return fmt.Errorf("error reading album.json: %v", err)
	}

	var albumItems []AlbumItem
	if err := json.Unmarshal(albumFile, &albumItems); err != nil {
		return fmt.Errorf("error parsing album.json: %v", err)
	}

	if len(albumItems) == 0 {
		return fmt.Errorf("no items in album.json")
	}

	// 2. Get the first item
	firstItem := albumItems[0]
	log.Printf("Publishing item: %v with URL: %s\n", firstItem.ID, firstItem.URL)

	// 3. Upload item to SFTP server
	if err := uploadToSFTP(firstItem); err != nil {
		return fmt.Errorf("error uploading to SFTP server: %v", err)
	}

	// 4. Invalidate CDN cache
	if err := invalidateCache(); err != nil {
		log.Printf("Warning: Failed to invalidate CDN cache: %v\n", err)
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
			return fmt.Errorf("error parsing archive.json: %v", err)
		}
	} else if !os.IsNotExist(err) {
		// Error other than file not existing
		return fmt.Errorf("error reading archive.json: %v", err)
	}

	// Append the item
	archiveItems = append(archiveItems, firstItem)

	// Write back to archive.json
	archiveData, err := json.Marshal(archiveItems)
	if err != nil {
		return fmt.Errorf("error serializing archive data: %v", err)
	}

	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		return fmt.Errorf("error writing to archive.json: %v", err)
	}

	// 6. Delete the item from data/album.json
	albumItems = albumItems[1:]
	albumData, err := json.Marshal(albumItems)
	if err != nil {
		return fmt.Errorf("error serializing album data: %v", err)
	}

	if err := os.WriteFile(albumPath, albumData, 0o644); err != nil {
		return fmt.Errorf("error writing to album.json: %v", err)
	}

	log.Println("Successfully published the image, uploaded to SFTP, invalidated cache, archived the item, and updated album.")
	return nil
}

func startServer() {
	// Configure logging
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags)

	// Create data directory if it doesn't exist
	if err := os.MkdirAll("data", 0o755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Create a custom ServeMux for routing
	mux := http.NewServeMux()

	// Handle API routes
	mux.HandleFunc("/api/album", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle OPTIONS request (preflight)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// GET request - return the album.json file
		if r.Method == "GET" {
			albumPath := filepath.Join("data", "album.json")

			// Check if file exists
			if _, err := os.Stat(albumPath); os.IsNotExist(err) {
				// Return empty JSON if file doesn't exist
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte("{}"))
				return
			}

			// Open and return the file
			albumFile, err := os.Open(albumPath)
			if err != nil {
				http.Error(w, "Failed to open album file", http.StatusInternalServerError)
				log.Printf("Failed to open album file: %v", err)
				return
			}
			defer albumFile.Close()

			w.Header().Set("Content-Type", "application/json")
			io.Copy(w, albumFile)
			return
		}

		// POST request - save the album.json file
		if r.Method == "POST" {
			albumPath := filepath.Join("data", "album.json")

			// Create or truncate the album file
			albumFile, err := os.Create(albumPath)
			if err != nil {
				http.Error(w, "Failed to create album file", http.StatusInternalServerError)
				log.Printf("Failed to create album file: %v", err)
				return
			}
			defer albumFile.Close()

			// Copy request body to file without parsing
			_, err = io.Copy(albumFile, r.Body)
			if err != nil {
				http.Error(w, "Failed to write album data", http.StatusInternalServerError)
				log.Printf("Failed to write album data: %v", err)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}

		// Method not allowed
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	// Add publish endpoint
	mux.HandleFunc("/api/publish", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle OPTIONS request (preflight)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Only allow POST requests
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Run the publish process
		err := publishProcess()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("Publish failed: %v", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","message":"Successfully published"}`))
	})

	// Serve static files from frontend directory
	fileServer := http.FileServer(http.Dir("./frontend"))
	mux.Handle("/", fileServer)

	// Add upload endpoint
	mux.HandleFunc("/api/upload", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle OPTIONS request (preflight)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Only allow POST requests
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Max upload size of 10MB
		r.ParseMultipartForm(10 << 20)
		
		// Get the file from the request
		file, handler, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Error retrieving the file", http.StatusBadRequest)
			log.Printf("Error retrieving file: %v", err)
			return
		}
		defer file.Close()

		// Create uploads directory if it doesn't exist
		uploadDir := "data/uploads"
		if err := os.MkdirAll(uploadDir, 0o755); err != nil {
			http.Error(w, "Error creating upload directory", http.StatusInternalServerError)
			log.Printf("Error creating upload directory: %v", err)
			return
		}

		// Get file extension from original filename
		fileExt := filepath.Ext(handler.Filename)
		
		// Create a temporary filename and path - will be renamed after hash is calculated
		tempFilename := fmt.Sprintf("temp-%d%s", time.Now().UnixNano(), fileExt)
		filePath := fmt.Sprintf("%s/%s", uploadDir, tempFilename)
		
		// Create the file on the server
		dst, err := os.Create(filePath)
		if err != nil {
			http.Error(w, "Error creating file on server", http.StatusInternalServerError)
			log.Printf("Error creating file: %v", err)
			return
		}
		
		// Always delete the local file when we're done
		defer os.Remove(filePath)
		
		// Set up hasher and multiwriter to write file and compute hash simultaneously
		hasher := sha256.New()
		writer := io.MultiWriter(dst, hasher)
		
		// Copy data from source to both the file and hasher
		if _, err := io.Copy(writer, file); err != nil {
			http.Error(w, "Error saving file on server", http.StatusInternalServerError)
			log.Printf("Error saving file: %v", err)
			return
		}
		
		// Get the hash result and create final filename
		hashBytes := hasher.Sum(nil)
		hashString := fmt.Sprintf("%x", hashBytes)
		filename := hashString + fileExt
		finalPath := fmt.Sprintf("%s/%s", uploadDir, filename)
		
		// Close the file
		dst.Close()
		
		// Rename file to hash-based name if temp and final names are different
		if finalPath != filePath {
			// First remove any existing file with the same name
			os.Remove(finalPath)
			// Then rename our file
			if err := os.Rename(filePath, finalPath); err != nil {
				log.Printf("Error renaming file: %v", err)
				// Continue with the temp file if rename fails
			} else {
				// Update path if rename succeeded
				filePath = finalPath
				// Update defer to remove the new path
				defer os.Remove(finalPath)
			}
		}
		
		// Upload the file to SFTP
		if err := uploadFileToSFTP(filePath, filename); err != nil {
			http.Error(w, "Error uploading file to SFTP", http.StatusInternalServerError)
			log.Printf("Error uploading to SFTP: %v", err)
			return
		}

		// Get CDN URL from environment
		godotenv.Load()
		cdnURL := os.Getenv("BUNNY_CDN_URL")
		if cdnURL == "" {
			cdnURL = "https://example.com" // Fallback if not set
		}

		// Return the URL for the uploaded file
		imageURL := fmt.Sprintf("%s/content/%s", cdnURL, filename)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"url": imageURL,
		})
	})

	// Apply logging middleware to all requests
	handler := loggingMiddleware(mux)

	// Get port from viper config
	serverPort := viper.GetString("server.port")

	// Start the server
	log.Printf("Starting server at http://:%s", serverPort)
	if err := http.ListenAndServe(":"+serverPort, handler); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
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

	log.Println("Successfully uploaded item to SFTP server as 'today.json'")
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

	log.Println("Successfully invalidated CDN cache for 'today.json' file")
	return nil
}

func uploadFileToSFTP(localFilePath, remoteFileName string) error {
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

	// Open local file
	localFile, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer localFile.Close()

	// Ensure content directory exists on remote server
	contentDir := "content"
	sftpClient.MkdirAll(contentDir)

	// Create a file on the SFTP server in the content directory
	remoteFilePath := fmt.Sprintf("%s/%s", contentDir, remoteFileName)
	remoteFile, err := sftpClient.Create(remoteFilePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %v", err)
	}
	defer remoteFile.Close()

	// Copy local file to remote
	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("failed to write to remote file: %v", err)
	}

	log.Printf("Successfully uploaded file to SFTP server at '%s'", remoteFilePath)
	return nil
}