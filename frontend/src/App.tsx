import type { Component } from 'solid-js';
import styles from './App.module.css';
import { createSignal } from 'solid-js';
import { Album } from './Album';
import ImageMetadataEditor from './ImageMetadataEditor';

interface ImageData {
  id: number;
  url: string;
  description?: string;
  credits?: string;
}

const App: Component = () => {
	const [selectedImage, setSelectedImage] = createSignal<ImageData | null>(null);
	const [albumRef, setAlbumRef] = createSignal<any>(null);
	
	// Function to handle when an image is selected from the album
	const handleImageSelect = (image: ImageData) => {
		setSelectedImage(image);
	};
	
	// Function to handle when editing is canceled
	const handleCancel = () => {
		// Clear the selection when the editor is canceled
		setSelectedImage(null);
		
		// Also clear the selection in the Album component
		if (albumRef()) {
			albumRef().clearSelection();
		}
	};
	
	// Function to save the updated metadata back to the album
	const handleSaveMetadata = (updatedImage: ImageData) => {
		// Update the image metadata in the album
		if (albumRef() && albumRef().updateItemMetadata) {
			albumRef().updateItemMetadata(updatedImage.id, {
				url: updatedImage.url,
				description: updatedImage.description,
				credits: updatedImage.credits
			});
			
			// Autosave to API
			saveAlbumJson();
		}
		
		// Update the selected image with the new metadata
		setSelectedImage(updatedImage);
	};
	
	// Function to delete an image from the album
	const handleDeleteImage = (imageId: number) => {
		if (albumRef() && albumRef().deleteItem) {
			// Delete the image from the album
			albumRef().deleteItem(imageId);
			
			// Clear the selection
			setSelectedImage(null);
			
			// Also clear the selection in the Album component
			if (albumRef().clearSelection) {
				albumRef().clearSelection();
			}
			
			// Autosave to API
			saveAlbumJson();
		}
	};
	
	// Function to save album data to the API
	const saveAlbumJson = async () => {
		if (albumRef() && albumRef().getAlbumData) {
			const albumData = albumRef().getAlbumData();
			
			try {
				const response = await fetch('http://localhost:8080/api/album', {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
					},
					body: JSON.stringify(albumData),
				});
				
				if (!response.ok) {
					console.error('Failed to save album:', await response.text());
				}
			} catch (error) {
				console.error('Error saving album:', error);
			}
		}
	};
	
	return (
		<div class={styles.app}>
			<header class={styles.header}>
				<h1>Image Metadata Editor</h1>
			</header>
			<main class={styles.mainContent}>
				<div class={styles.albumSection}>
					<Album 
						ref={setAlbumRef} 
						onSelectImage={handleImageSelect}
						onAlbumChange={saveAlbumJson}
					/>
				</div>
				<div class={styles.editorSection}>
					<ImageMetadataEditor 
						selectedImage={selectedImage()} 
						onCancel={handleCancel}
						onSave={handleSaveMetadata}
						onDelete={handleDeleteImage}
					/>
				</div>
			</main>
		</div>
	);
};

export default App;