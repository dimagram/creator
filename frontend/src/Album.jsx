import {
	DragDropProvider,
	DragDropSensors,
	DragOverlay,
	SortableProvider,
	createSortable,
	closestCenter,
} from "@thisbeyond/solid-dnd";
import { createSignal, For } from "solid-js";
import './Album.css';

// Function to generate a UUID v4
const generateUUID = () => {
	return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
		const r = Math.random() * 16 | 0;
		const v = c === 'x' ? r : (r & 0x3 | 0x8);
		return v.toString(16);
	});
};

const Sortable = (props) => {
	const sortable = createSortable(props.item.id);
	const [isDragging, setIsDragging] = createSignal(false);

	// Handle drag state
	const onDragStart = () => setIsDragging(true);
	const onDragEnd = () => setTimeout(() => setIsDragging(false), 100);

	// Apply drag event handlers
	if (sortable.dragActivators) {
		const oldOnDragStart = sortable.dragActivators.onDragStart;
		sortable.dragActivators.onDragStart = (e) => {
			onDragStart();
			if (oldOnDragStart) oldOnDragStart(e);
		};

		const oldOnDragEnd = sortable.dragActivators.onDragEnd;
		sortable.dragActivators.onDragEnd = (e) => {
			onDragEnd();
			if (oldOnDragEnd) oldOnDragEnd(e);
		};
	}

	const handleClick = (e) => {
		// Prevent click from triggering if dragging
		if (sortable.isActiveDraggable || isDragging()) {
			return;
		}

		e.stopPropagation();

		// Only handle click if we have a selection callback
		if (props.onSelect) {
			props.onSelect(props.item);
		}
	};

	return (
		<div
			use:sortable
			class="sortable"
			classList={{
				"opacity-25": sortable.isActiveDraggable,
				"selected": props.isSelected
			}}
			onClick={handleClick}
		>
			<img src={props.item.url} alt={props.item.description || `Photo ${props.item.id}`} />
			<span class="photo-number">{props.item.id}</span>
			{props.isSelected && <div class="selected-overlay"></div>}
		</div>
	);
};

export const Album = (props) => {
	const [albumData, setAlbumData] = createSignal([]);

	// Load album data from API
	const loadAlbumData = async () => {
		const response = await fetch('/api/album');
		if (response.ok) {
			const data = await response.json();
			setAlbumData(data);
		}
	};

	// Load album on component initialization
	loadAlbumData();
	const [activeItem, setActiveItem] = createSignal(null);
	const [selectedItemId, setSelectedItemId] = createSignal(null);

	// Watch for the selected prop to be null, which means selection has been cleared
	const clearSelection = () => {
		setSelectedItemId(null);
	};

	// We'll set the ref once at the end of the component

	const ids = () => albumData().map(item => item.id);

	const onDragStart = ({ draggable }) => setActiveItem(draggable.id);

	const onDragEnd = ({ draggable, droppable }) => {
		if (draggable && droppable) {
			const currentItems = albumData();
			const fromIndex = currentItems.findIndex(item => item.id === draggable.id);
			const toIndex = currentItems.findIndex(item => item.id === droppable.id);

			if (fromIndex !== toIndex) {
				const updatedItems = currentItems.slice();
				updatedItems.splice(toIndex, 0, ...updatedItems.splice(fromIndex, 1));
				setAlbumData(updatedItems);

				// Notify parent of change for autosave
				if (props.onAlbumChange) {
					props.onAlbumChange();
				}
			}
		}
		setActiveItem(null);
	};

	const handleItemSelect = (item) => {
		// Set selected item ID for visual feedback
		setSelectedItemId(item.id);

		// Pass the selected item to parent component
		if (props.onSelectImage) {
			props.onSelectImage(item);
		}
	};

	// Update item metadata (called from parent component)
	const updateItemMetadata = (id, metadata) => {
		const currentItems = albumData();
		const itemIndex = currentItems.findIndex(item => item.id === id);

		if (itemIndex !== -1) {
			const updatedItems = [...currentItems];
			updatedItems[itemIndex] = { ...updatedItems[itemIndex], ...metadata };
			setAlbumData(updatedItems);
		}
	};

	// Function to delete an item from the album
	const deleteItem = (id) => {
		const currentItems = albumData();
		const updatedItems = currentItems.filter(item => item.id !== id);
		setAlbumData(updatedItems);
		setSelectedItemId(null);
	};

	// Function to add a new empty image
	const addNewImage = () => {
		// Use placeholder URL for empty images
		addNewImageWithUrl('https://placehold.co/400x400?text=New+Image');
	};
	
	// Function to add a new image with a specific URL
	const addNewImageWithUrl = (url) => {
		const currentItems = albumData();
		const newImageId = generateUUID();

		// Create a new image entry with the specified URL
		const newImage = {
			id: newImageId,
			url: url,
			description: '',
			credits: ''
		};

		// Add to album data
		const updatedItems = [...currentItems, newImage];
		setAlbumData(updatedItems);

		// Select the new image for editing
		handleItemSelect(newImage);

		// Notify parent of change for autosave
		if (props.onAlbumChange) {
			props.onAlbumChange();
		}
	};

	// Set up a method to select an image by ID (for external use)
	const selectImageById = (id) => {
		const item = albumData().find(item => item.id === id);
		if (item) {
			setSelectedItemId(id);
			if (props.onSelectImage) {
				props.onSelectImage(item);
			}
		}
	};

	// Create a single ref object with all methods
	const refObj = {
		clearSelection,
		getAlbumData: () => albumData(),
		setAlbumData,
		updateItemMetadata,
		deleteItem,
		addNewImage,
		addNewImageWithUrl,
		selectImageById
	};

	// Expose methods to parent via ref if provided
	if (props.ref) {
		props.ref(refObj);
	}

	return (
		<DragDropProvider
			onDragStart={onDragStart}
			onDragEnd={onDragEnd}
			collisionDetector={closestCenter}
		>
			<DragDropSensors />
			<h1>Queue</h1>
			<div class="photo-grid">
				<SortableProvider ids={ids()}>
					<For each={albumData()}>
						{(item) => (
							<Sortable
								item={item}
								isSelected={selectedItemId() === item.id}
								onSelect={handleItemSelect}
							/>
						)}
					</For>
				</SortableProvider>
				{/* Add Image tile */}
				<div class="add-image-tile" onClick={addNewImage}>
					<div class="add-image-icon">+</div>
				</div>
			</div>
			<DragOverlay>
				{activeItem() && (
					<div class="sortable">
						<img src={albumData().find(item => item.id === activeItem())?.url} />
					</div>
				)}
			</DragOverlay>
		</DragDropProvider>
	);
};