import { Component, createSignal, Show, createEffect, onMount, onCleanup } from 'solid-js';
import styles from './ImageMetadataEditor.module.css';

interface ImageData {
  id: number;
  url: string;
  description?: string;
  credits?: string;
}

interface ImageMetadataEditorProps {
  selectedImage?: ImageData | null;
  onCancel?: () => void;
  onSave?: (imageData: ImageData) => void;
  onDelete?: (imageId: number) => void;
}

const ImageMetadataEditor: Component<ImageMetadataEditorProps> = (props) => {
  const [url, setUrl] = createSignal('');
  const [description, setDescription] = createSignal('');
  const [credits, setCredits] = createSignal('');
  const [statusMessage, setStatusMessage] = createSignal('');
  const [deleteConfirm, setDeleteConfirm] = createSignal(false);

  // Listen for changes to the selectedImage prop
  createEffect(() => {
    const selectedImage = props.selectedImage;
    if (selectedImage) {
      // Load existing metadata if available
      setUrl(selectedImage.url || '');
      setDescription(selectedImage.description || '');
      setCredits(selectedImage.credits || '');
      setStatusMessage('');
      setDeleteConfirm(false);
    }
  });
  
  // Handle escape key to cancel
  const handleKeyDown = (e) => {
    if (e.key === 'Escape' && props.selectedImage) {
      // Clear form and cancel editing
      setUrl('');
      setDescription('');
      setCredits('');
      setStatusMessage('');
      setDeleteConfirm(false);
      
      // Tell parent component to clear selection
      if (props.onCancel) {
        props.onCancel();
      }
    }
  };
  
  // Set up and clean up event listeners
  onMount(() => {
    document.addEventListener('keydown', handleKeyDown);
  });
  
  onCleanup(() => {
    document.removeEventListener('keydown', handleKeyDown);
  });

  // Save the metadata
  const saveMetadata = () => {
    if (!props.selectedImage) return;
    
    // Create updated image data
    const updatedImageData: ImageData = {
      ...props.selectedImage,
      url: url(),
      description: description(),
      credits: credits()
    };
    
    // Call the onSave callback with the updated data
    if (props.onSave) {
      props.onSave(updatedImageData);
      setStatusMessage('Metadata saved successfully!');
      setTimeout(() => setStatusMessage(''), 3000);
    }
  };

  // Delete the image
  const deleteImage = () => {
    if (!props.selectedImage) return;
    
    if (!deleteConfirm()) {
      // First click - show confirmation
      setDeleteConfirm(true);
      setStatusMessage('Click delete again to confirm deletion');
      return;
    }
    
    // Second click - actually delete
    if (props.onDelete) {
      props.onDelete(props.selectedImage.id);
      setStatusMessage('Image deleted');
      // No need to reset form since the image will be removed
    }
  };
  
  return (
    <div class={styles.container}>
      <h2>Image Metadata Editor</h2>
      
      {statusMessage() && (
        <div class={`${styles.statusMessage} ${statusMessage().includes('Error') ? styles.error : styles.success}`}>
          {statusMessage()}
        </div>
      )}
      
      <Show
        when={props.selectedImage}
        fallback={
          <div class={styles.dropzone}>
            <p>Click on an image from the gallery to edit its metadata</p>
          </div>
        }
      >
        <div class={styles.editor}>
          <div class={styles.imagePreview}>
            <img src={props.selectedImage?.url} alt="Preview" />
          </div>
          
          <div class={styles.metadataForm}>
            <div class={styles.formGroup}>
              <label for="url">Image URL:</label>
              <input 
                type="text" 
                id="url" 
                value={url()} 
                onInput={(e) => setUrl(e.currentTarget.value)}
                placeholder="Image URL..."
              />
            </div>
            
            <div class={styles.formGroup}>
              <label for="description">Description:</label>
              <textarea 
                id="description" 
                value={description()} 
                onInput={(e) => setDescription(e.currentTarget.value)}
                placeholder="Enter image description..."
              />
            </div>
            
            <div class={styles.formGroup}>
              <label for="credits">Credits:</label>
              <input 
                type="text" 
                id="credits" 
                value={credits()} 
                onInput={(e) => setCredits(e.currentTarget.value)}
                placeholder="Enter photographer or creator name..."
              />
            </div>
            
            <div class={styles.buttonGroup}>
              <button 
                onClick={() => {
                  // Clear form and cancel editing
                  setUrl('');
                  setDescription('');
                  setCredits('');
                  setStatusMessage('');
                  setDeleteConfirm(false);
                  
                  // Tell parent component to clear selection
                  if (props.onCancel) {
                    props.onCancel();
                  }
                }}
                class={styles.cancelButton}
              >
                Cancel
              </button>
              <button 
                onClick={saveMetadata}
                class={styles.saveButton}
              >
                Save Metadata
              </button>
            </div>
            
            <div class={styles.deleteContainer}>
              <button 
                onClick={deleteImage}
                class={`${styles.deleteButton} ${deleteConfirm() ? styles.confirmDelete : ''}`}
              >
                {deleteConfirm() ? 'Confirm Delete' : 'Delete Image'}
              </button>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
};

export default ImageMetadataEditor;