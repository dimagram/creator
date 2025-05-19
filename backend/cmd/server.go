package cmd

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	port string
)

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

	// Serve static files from frontend directory
	fileServer := http.FileServer(http.Dir("./frontend"))
	mux.Handle("/", fileServer)

	// Apply logging middleware to all requests
	handler := loggingMiddleware(mux)

	// Get port from viper config
	serverPort := viper.GetString("server.port")

	// Start the server
	log.Printf("Starting server at http://localhost:%s", serverPort)
	if err := http.ListenAndServe(":"+serverPort, handler); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
