package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var unpublishCmd = &cobra.Command{
	Use:   "unpublish",
	Short: "Unpublish the most recently published image",
	Long:  `Move the most recently published image from archive back to album, and update the SFTP "today" file with the new latest image.`,
	Run: func(cmd *cobra.Command, args []string) {
		unpublishProcess()
	},
}

// GetUnpublishCmd returns the unpublish command
func GetUnpublishCmd() *cobra.Command {
	return unpublishCmd
}

func unpublishProcess() {
	// 1. Parse archive.json
	archivePath := filepath.Join("data", "archive.json")
	archiveFile, err := os.ReadFile(archivePath)
	if err != nil {
		fmt.Printf("Error reading archive.json: %v\n", err)
		os.Exit(1)
	}

	var archiveItems []AlbumItem
	if err := json.Unmarshal(archiveFile, &archiveItems); err != nil {
		fmt.Printf("Error parsing archive.json: %v\n", err)
		os.Exit(1)
	}

	if len(archiveItems) == 0 {
		fmt.Println("No items in archive.json")
		os.Exit(1)
	}

	// 2. Get the last item from archive
	lastIndex := len(archiveItems) - 1
	lastItem := archiveItems[lastIndex]
	fmt.Printf("Unpublishing item: %v with URL: %s\n", lastItem.ID, lastItem.URL)

	// 3. Read album.json
	albumPath := filepath.Join("data", "album.json")
	var albumItems []AlbumItem

	albumFile, err := os.ReadFile(albumPath)
	if err == nil {
		// File exists, parse it
		if err := json.Unmarshal(albumFile, &albumItems); err != nil {
			fmt.Printf("Error parsing album.json: %v\n", err)
			os.Exit(1)
		}
	} else if !os.IsNotExist(err) {
		// Error other than file not existing
		fmt.Printf("Error reading album.json: %v\n", err)
		os.Exit(1)
	}

	// 4. Remove the last item from archive and add to album
	archiveItems = archiveItems[:lastIndex]
	albumItems = append([]AlbumItem{lastItem}, albumItems...)

	// If there are still items in the archive, update the SFTP "today" file to the new last item
	if len(archiveItems) > 0 {
		newLastItem := archiveItems[len(archiveItems)-1]
		if err := uploadToSFTP(newLastItem); err != nil {
			fmt.Printf("Error uploading to SFTP server: %v\n", err)
			os.Exit(1)
		}
		
		// Invalidate CDN cache
		if err := invalidateCache(); err != nil {
			fmt.Printf("Warning: Failed to invalidate CDN cache: %v\n", err)
			// Continue execution even if cache invalidation fails
		}
		
		fmt.Printf("Updated SFTP 'today' file to point to the new last item: %v\n", newLastItem.ID)
	}

	// 6. Write back to archive.json
	archiveData, err := json.Marshal(archiveItems)
	if err != nil {
		fmt.Printf("Error serializing archive data: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		fmt.Printf("Error writing to archive.json: %v\n", err)
		os.Exit(1)
	}

	// 7. Write back to album.json
	albumData, err := json.Marshal(albumItems)
	if err != nil {
		fmt.Printf("Error serializing album data: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(albumPath, albumData, 0o644); err != nil {
		fmt.Printf("Error writing to album.json: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully unpublished the image, updated SFTP 'today' file, invalidated cache, moved item from archive to album.")
}